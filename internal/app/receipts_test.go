package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// typstPDF compiles a document with the given number of pages.
func typstPDF(t *testing.T, pages int) []byte {
	t.Helper()
	dir := t.TempDir()
	src := fmt.Sprintf("#for i in range(%d) { [Beleg Seite #(i + 1)]; if i < %d { pagebreak() } }", pages, pages-1)
	if err := os.WriteFile(filepath.Join(dir, "doc.typ"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "doc.pdf")
	if b, err := exec.Command(typstBin(t), "compile", "--root", dir, filepath.Join(dir, "doc.typ"), out).CombinedOutput(); err != nil {
		t.Fatalf("typst: %v\n%s", err, b)
	}
	return must(os.ReadFile(out))
}

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// TestA21UploadLimits covers multipage PDFs and the rejections a user can
// hit: size, pages, pixels, encryption and unreadable content.
func TestA21UploadLimits(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)

	r := must(a.Upload(ctx, u, "rechnung.pdf", typstPDF(t, 3), ""))
	if r.Pages != 3 {
		t.Fatalf("pages = %d, want 3", r.Pages)
	}

	png := syntheticReceipt(9)
	for _, c := range []struct {
		name  string
		data  []byte
		limit func(*Config)
		want  string
	}{
		{"too large", png, func(c *Config) { c.MaxUploadBytes = int64(len(png) - 1) }, "upload.too_large"},
		{"too many pages", typstPDF(t, 3), func(c *Config) { c.MaxPDFPages = 2 }, "upload.too_many_pages"},
		{"too many pixels", png, func(c *Config) { c.MaxPixels = 420*640 - 1 }, "upload.too_many_pixels"},
		{"encrypted", []byte("%PDF-1.7\n1 0 obj << /Encrypt 2 0 R >> endobj\n%%EOF\n"), nil, "upload.pdf_encrypted"},
		{"broken PDF", []byte("%PDF-1.7\nnot really a pdf\n%%EOF\n"), nil, "upload.pdf_invalid"},
		{"truncated PNG", png[:len(png)/2], nil, "upload.unreadable"},
		{"unsupported", []byte("GIF89a......"), nil, "upload.unsupported"},
	} {
		t.Run(c.name, func(t *testing.T) {
			saved := a.Cfg
			defer func() { a.Cfg = saved }()
			if c.limit != nil {
				c.limit(&a.Cfg)
			}
			_, err := a.Upload(ctx, u, "x", c.data, "")
			if got := code(err); got != c.want {
				t.Fatalf("error %v, want %s", err, c.want)
			}
		})
	}

	// A rejected upload leaves no file behind: only the accepted PDF and its
	// preview are stored.
	n := 0
	_ = filepath.WalkDir(filepath.Join(a.Cfg.DataDir, "files"), func(_ string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			n++
		}
		return nil
	})
	if n != 2 {
		t.Fatalf("stored files = %d, want 2 (one PDF, one preview)", n)
	}
}
