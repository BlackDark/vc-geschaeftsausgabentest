package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/api"
)

// Config is read from SPESEN_* environment variables. Secrets can also be
// read from a file named by the variable with a _FILE suffix.
type Config struct {
	Listen         string
	DataDir        string
	BaseURL        string
	SecureCookies  bool
	SetupToken     string
	PasswordLogin  bool
	OIDC           *api.OIDCConfig
	Typst          string
	MaxUploadBytes int64
	MaxPixels      int64
	MaxPDFPages    int
	LogLevel       slog.Level
}

func env(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return strings.TrimSpace(v)
	}
	return def
}

func secret(name string) (string, error) {
	if p := env(name+"_FILE", ""); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return env(name, ""), nil
}

func list(name string) []string {
	var out []string
	for _, s := range strings.Split(env(name, ""), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// LoadConfig reads and validates the configuration; invalid combinations
// fail at startup.
func LoadConfig() (*Config, error) {
	c := &Config{
		Listen:  env("SPESEN_LISTEN", ":8080"),
		DataDir: env("SPESEN_DATA_DIR", "/data"),
		BaseURL: strings.TrimRight(env("SPESEN_BASE_URL", "http://localhost:8080"), "/"),
		Typst:   env("SPESEN_TYPST", "typst"),
	}
	var errs []error
	u, err := url.Parse(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("SPESEN_BASE_URL must be an absolute http(s) URL"))
	} else {
		c.SecureCookies = u.Scheme == "https"
	}
	if c.SetupToken, err = secret("SPESEN_SETUP_TOKEN"); err != nil {
		errs = append(errs, err)
	}
	c.PasswordLogin, err = strconv.ParseBool(env("SPESEN_PASSWORD_LOGIN", "true"))
	if err != nil {
		errs = append(errs, errors.New("SPESEN_PASSWORD_LOGIN must be true or false"))
	}
	if issuer := env("SPESEN_OIDC_ISSUER", ""); issuer != "" {
		o := &api.OIDCConfig{
			Issuer: issuer, ClientID: env("SPESEN_OIDC_CLIENT_ID", ""), Label: env("SPESEN_OIDC_LABEL", "Pocket ID"),
			GroupsClaim: env("SPESEN_OIDC_GROUPS_CLAIM", "groups"), AllowedGroups: list("SPESEN_OIDC_ALLOWED_GROUPS"), AdminGroups: list("SPESEN_OIDC_ADMIN_GROUPS"),
			Scopes: list("SPESEN_OIDC_SCOPES"),
		}
		if o.ClientSecret, err = secret("SPESEN_OIDC_CLIENT_SECRET"); err != nil {
			errs = append(errs, err)
		}
		if o.ClientID == "" {
			errs = append(errs, errors.New("SPESEN_OIDC_CLIENT_ID is required with SPESEN_OIDC_ISSUER"))
		}
		c.OIDC = o
	}
	if !c.PasswordLogin && c.OIDC == nil {
		errs = append(errs, errors.New("enable at least one login mode (SPESEN_PASSWORD_LOGIN or SPESEN_OIDC_ISSUER)"))
	}
	mb, err := strconv.Atoi(env("SPESEN_MAX_UPLOAD_MB", "25"))
	if err != nil || mb < 1 || mb > 25 {
		errs = append(errs, errors.New("SPESEN_MAX_UPLOAD_MB must be between 1 and 25"))
	}
	c.MaxUploadBytes = int64(mb) << 20
	px, err := strconv.ParseInt(env("SPESEN_MAX_MEGAPIXELS", "40"), 10, 64)
	if err != nil || px < 1 || px > 40 {
		errs = append(errs, errors.New("SPESEN_MAX_MEGAPIXELS must be between 1 and 40"))
	}
	c.MaxPixels = px * 1_000_000
	if c.MaxPDFPages, err = strconv.Atoi(env("SPESEN_MAX_PDF_PAGES", "50")); err != nil || c.MaxPDFPages < 1 || c.MaxPDFPages > 50 {
		errs = append(errs, errors.New("SPESEN_MAX_PDF_PAGES must be between 1 and 50"))
	}
	if err := c.LogLevel.UnmarshalText([]byte(env("SPESEN_LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, errors.New("SPESEN_LOG_LEVEL must be debug, info, warn or error"))
	}
	return c, errors.Join(errs...)
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
