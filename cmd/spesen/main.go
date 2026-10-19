// Command spesen runs the expense claim server and its maintenance tasks.
//
//	spesen [serve]                      run the server
//	spesen openapi                      print the OpenAPI document
//	spesen import-rates YEAR FILE SRC   import a further year's foreign rates
//	spesen backup DIR                   write a consistent backup to DIR
//	spesen verify                       check checksums of receipts and exports
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/api"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/app"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/db"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/export"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/rates"
	"github.com/blackdark/vc-geschaeftsausgabentest/web"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "openapi":
		err = printOpenAPI()
	case "import-rates":
		err = importRates(os.Args[2:])
	case "backup":
		err = backup(os.Args[2:])
	case "verify":
		err = verify()
	case "healthcheck":
		err = healthcheck()
	case "version", "--version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func logger(cfg *Config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
}

func serve() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	log := logger(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "spesen.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	fontDir, err := os.MkdirTemp("", "spesen-fonts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(fontDir)
	if err := export.ExtractFonts(fontDir); err != nil {
		return err
	}
	a, err := app.New(ctx, conn, app.Config{DataDir: cfg.DataDir, Typst: cfg.Typst, FontDir: fontDir, MaxUploadBytes: cfg.MaxUploadBytes, MaxPixels: cfg.MaxPixels, MaxPDFPages: cfg.MaxPDFPages}, log)
	if err != nil {
		return err
	}
	if err := ensureSetupToken(ctx, a, cfg, log); err != nil {
		return err
	}
	handler, err := api.New(a, api.Config{BaseURL: cfg.BaseURL, SecureCookies: cfg.SecureCookies, SetupToken: cfg.SetupToken, PasswordEnabled: cfg.PasswordLogin, OIDC: cfg.OIDC, Version: version}, log, web.Handler())
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("listening", "addr", cfg.Listen, "version", version, "password", cfg.PasswordLogin, "oidc", cfg.OIDC != nil)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ensureSetupToken writes a one-time token file when no administrator
// exists and the operator did not supply SPESEN_SETUP_TOKEN.
func ensureSetupToken(ctx context.Context, a *app.App, cfg *Config, log *slog.Logger) error {
	needs, err := a.NeedsSetup(ctx)
	if err != nil || !needs || cfg.SetupToken != "" {
		return err
	}
	cfg.SetupToken = randomToken()
	p := filepath.Join(cfg.DataDir, "setup-token")
	if err := os.WriteFile(p, []byte(cfg.SetupToken+"\n"), 0o600); err != nil {
		return err
	}
	log.Warn("no administrator yet: the setup token was written to a file", "path", p)
	return nil
}

func printOpenAPI() error {
	b, err := json.MarshalIndent(api.NewForSpec(version).OpenAPI(), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func importRates(args []string) error {
	if len(args) != 3 {
		return errors.New("usage: spesen import-rates YEAR FILE SOURCE")
	}
	year, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid year %q", args[0])
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "spesen.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := rates.Seed(ctx, conn); err != nil {
		return err
	}
	f, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := rates.Import(ctx, conn, year, f, args[2])
	if err != nil {
		return err
	}
	fmt.Printf("imported %d rows for %d; restart the server to load them\n", n, year)
	return nil
}

// backup writes a consistent SQLite copy (VACUUM INTO) and copies the
// referenced files. Run it while the server is running or stopped.
func backup(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: spesen backup DIR")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	dest := args[0]
	if _, err := os.Stat(filepath.Join(dest, "spesen.db")); err == nil {
		return fmt.Errorf("%s already contains a backup", dest)
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "spesen.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := app.Backup(ctx, conn, cfg.DataDir, dest); err != nil {
		return err
	}
	fmt.Println("backup written to", dest)
	return nil
}

// verify checks every stored original and export against its recorded
// SHA-256; run it after a restore.
func verify() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "spesen.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	files, err := app.NewFiles(cfg.DataDir)
	if err != nil {
		return err
	}
	r, err := (&app.App{DB: conn, Files: files}).Verify(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("checked %d receipt originals and %d export files\n", r.Receipts, r.Exports)
	for _, p := range r.Problems {
		fmt.Println("PROBLEM:", p)
	}
	if len(r.Problems) > 0 {
		return fmt.Errorf("%d problems found", len(r.Problems))
	}
	fmt.Println("all checksums match")
	return nil
}

func healthcheck() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second}
	addr := cfg.Listen
	if addr != "" && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
