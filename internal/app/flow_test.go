package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/db"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/export"
)

// typstBin finds Typst for integration tests (SPESEN_TYPST or PATH).
func typstBin(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("SPESEN_TYPST"); p != "" {
		return p
	}
	p, err := exec.LookPath("typst")
	if err != nil {
		t.Skip("typst not installed; set SPESEN_TYPST")
	}
	return p
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fontDir := filepath.Join(dir, "fonts")
	if err := export.ExtractFonts(fontDir); err != nil {
		t.Fatal(err)
	}
	a, err := New(ctx, conn, Config{DataDir: filepath.Join(dir, "data"), Typst: typstBin(t), FontDir: fontDir, MaxUploadBytes: 25 << 20, MaxPixels: 40_000_000, MaxPDFPages: 50}, slog.New(slog.NewTextHandler(testLogWriter(), nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// must panics on error, which fails the running test.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// syntheticReceipt draws a receipt-like PNG; n varies the content so each
// upload has distinct bytes.
func syntheticReceipt(n byte) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 420, 640))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	gray := &image.Uniform{color.RGBA{60, 60, 60, 255}}
	for i := 0; i < 14; i++ {
		w := 120 + (int(n)*37+i*53)%220
		draw.Draw(img, image.Rect(30, 40+i*38, 30+w, 52+i*38), gray, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func receipt(t *testing.T, a *App, u *User, name string, n byte) int64 {
	t.Helper()
	r := must(a.Upload(context.Background(), u, name, syntheticReceipt(n), ""))
	r = must(a.ConfirmReceipt(context.Background(), u, r.ID))
	if !strings.HasPrefix(r.Number, "B-") {
		t.Fatalf("receipt number %q", r.Number)
	}
	return r.ID
}

// TestF01FirstWorkingFlow is the mandatory first flow (docs/ACCEPTANCE.md).
func TestF01FirstWorkingFlow(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	admin := must(a.Setup(ctx, "tok", "tok", "user-a", "User A", "correct horse battery"))
	u := must(a.PasswordLogin(ctx, "user-a", "correct horse battery", "test"))
	if u.ID != admin.ID {
		t.Fatal("login user mismatch")
	}
	if u.Theme != DefaultTheme || DefaultTheme != "dark" {
		t.Fatalf("new accounts start dark, got %q", u.Theme)
	}
	must(a.SaveEmployer(ctx, u, EmployerInput{Name: "Example Company", Address: "Musterstraße 1\n10115 Berlin"}))

	trip := must(a.CreateTrip(ctx, u, TripInput{
		Purpose: "Kundentermin", Destination: "Hamburg",
		Start: "2026-04-13T08:00", StartTZ: "Europe/Berlin", End: "2026-04-15T18:00", EndTZ: "Europe/Berlin",
		Nights: []NightDTO{{Date: "2026-04-13", Kind: "private_actual"}, {Date: "2026-04-14", Kind: "private_actual"}},
	}))
	hotel := must(a.CreateExpense(ctx, u, ExpenseInput{
		TripID: &trip.ID, Date: "2026-04-15", CostType: "lodging", Payer: "user", Supplier: "Hotel Hafen", Currency: "EUR", GrossMinor: 17800,
		VATUnknown: true, Nights: []string{"2026-04-13", "2026-04-14"}, IncludesBreakfast: true, ReceiptIDs: []int64{receipt(t, a, u, "hotel.png", 1)},
	}))
	_ = hotel
	trip = must(a.GetTrip(ctx, u, trip.ID))
	breakfasts := 0
	for _, d := range trip.Days {
		if d.Breakfast && strings.HasPrefix(d.BreakfastOrigin, "hotel:") {
			breakfasts++
		}
	}
	if breakfasts != 2 {
		t.Fatalf("suggested breakfasts = %d, want 2", breakfasts)
	}

	must(a.CreateExpense(ctx, u, ExpenseInput{Date: "2026-04-16", CostType: "tools_equipment", Payer: "user", Supplier: "Werkzeug GmbH", Currency: "EUR", GrossMinor: 4000, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "tools.png", 2)}}))
	software := must(a.CreateExpense(ctx, u, ExpenseInput{Date: "2026-04-17", CostType: "software_services", Payer: "user", Supplier: "Software AG", Currency: "EUR", GrossMinor: 2000, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "software.png", 3)}}))
	must(a.CreateAdvance(ctx, u, AdvanceInput{Date: "2026-04-01", Amount: 10000, Note: "Reisekostenvorschuss"}))

	claim := must(a.CreateClaim(ctx, u, ClaimInput{Title: "Q2 2026", PeriodStart: "2026-04-01", PeriodEnd: "2026-06-30"}))
	if len(claim.TripIDs) != 1 || len(claim.OutlayIDs) != 2 || len(claim.AdvanceIDs) != 1 {
		t.Fatalf("suggested items: %+v", claim)
	}
	p := must(a.PreviewClaim(ctx, u, claim.ID))
	if len(p.Blockers) > 0 {
		t.Fatalf("blockers: %+v", p.Blockers)
	}
	if p.Totals.Payout != 18280 {
		t.Fatalf("payout = %v, want 182.80", p.Totals.Payout)
	}
	claim = must(a.SubmitClaim(ctx, u, claim.ID, SubmitInput{Revision: p.Revision, AcknowledgeWarnings: true}))
	if claim.Status != "submitted" || claim.Number == "" || claim.CurrentVersion != 1 {
		t.Fatalf("after submit: %+v", claim)
	}
	pdf1, _ := must2(a.VersionFile(ctx, u, claim.ID, 1, "pdf"))
	zip1, _ := must2(a.VersionFile(ctx, u, claim.ID, 1, "zip"))
	checkZip(t, zip1, 3)
	if err := os.WriteFile(filepath.Join(os.TempDir(), "spesen-f01-v1.pdf"), pdf1, 0o644); err != nil {
		t.Log(err)
	}

	// Submitted data is locked server-side.
	if _, err := a.UpdateExpense(ctx, u, software.ID, software.ExpenseInput); err != ErrLocked {
		t.Fatalf("edit after submit: %v", err)
	}

	// Correction: unlock, change the software outlay, resubmit.
	must(a.UnlockClaim(ctx, u, claim.ID, "Softwarebetrag korrigiert"))
	in := software.ExpenseInput
	in.GrossMinor, in.Version = 3000, 0
	must(a.UpdateExpense(ctx, u, software.ID, in))
	p = must(a.PreviewClaim(ctx, u, claim.ID))
	if p.Totals.Payout != 19280 {
		t.Fatalf("corrected payout = %v, want 192.80", p.Totals.Payout)
	}
	claim2 := must(a.SubmitClaim(ctx, u, claim.ID, SubmitInput{Revision: p.Revision, AcknowledgeWarnings: true}))
	if claim2.Number != claim.Number || claim2.CurrentVersion != 2 {
		t.Fatalf("resubmit: %+v", claim2)
	}
	again, _ := must2(a.VersionFile(ctx, u, claim.ID, 1, "pdf"))
	if !bytes.Equal(again, pdf1) {
		t.Fatal("version 1 PDF changed")
	}
	if claim2.Versions[1].SupersededAt == "" || claim2.Versions[0].Payout != 19280 || claim2.Versions[1].Payout != 18280 {
		t.Fatalf("versions: %+v", claim2.Versions)
	}

	// Paid and undo; payment does not rewrite export bytes.
	must(a.MarkPaid(ctx, u, claim.ID, PaymentInput{Date: "2026-07-05"}))
	if _, err := a.UnlockClaim(ctx, u, claim.ID, "x"); err == nil {
		t.Fatal("unlock while paid must fail")
	}
	must(a.UndoPaid(ctx, u, claim.ID, "Zahlung storniert"))
	pdf2, _ := must2(a.VersionFile(ctx, u, claim.ID, 2, "pdf"))
	if err := os.WriteFile(filepath.Join(os.TempDir(), "spesen-f01-v2.pdf"), pdf2, 0o644); err != nil {
		t.Log(err)
	}

	// A27: backup, restore into a clean location and verify by checksum.
	restored := filepath.Join(t.TempDir(), "restore")
	if err := Backup(ctx, a.DB, a.Cfg.DataDir, restored); err != nil {
		t.Fatal(err)
	}
	conn2, err := db.Open(ctx, filepath.Join(restored, "spesen.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	rep := must((&App{DB: conn2, Files: must(NewFiles(restored))}).Verify(ctx))
	if len(rep.Problems) != 0 || rep.Receipts != 3 || rep.Exports != 4 {
		t.Fatalf("restore verify: %+v", rep)
	}
	var restoredPayout int64
	if err := conn2.QueryRow(`SELECT payout FROM claim_versions WHERE version = 2`).Scan(&restoredPayout); err != nil || restoredPayout != 19280 {
		t.Fatalf("restored payout %d, %v", restoredPayout, err)
	}
	var key string
	_ = conn2.QueryRow(`SELECT pdf_key FROM claim_versions WHERE version = 1`).Scan(&key)
	if err := os.WriteFile(filepath.Join(restored, filepath.FromSlash(key)), []byte("tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	if rep := must((&App{DB: conn2, Files: must(NewFiles(restored))}).Verify(ctx)); len(rep.Problems) != 1 {
		t.Fatalf("tampering not detected: %+v", rep)
	}

	// A second claim cannot reuse the outlay.
	c2 := must(a.CreateClaim(ctx, u, ClaimInput{PeriodStart: "2026-04-01", PeriodEnd: "2026-06-30"}))
	if len(c2.OutlayIDs) != 0 || len(c2.TripIDs) != 0 || len(c2.AdvanceIDs) != 0 {
		t.Fatalf("second claim reused items: %+v", c2)
	}
	if _, err := a.SetClaimItem(ctx, u, c2.ID, ClaimItem{Kind: "outlay", ID: software.ID}, true); err == nil {
		t.Fatal("outlay reused")
	}
}

func must2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		panic(err)
	}
	return a, b
}

func checkZip(t *testing.T, b []byte, evidence int) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		files[f.Name] = data
	}
	manifest := string(files["manifest-sha256.txt"])
	n := 0
	for name, data := range files {
		if name == "manifest-sha256.txt" {
			continue
		}
		sum := sha256.Sum256(data)
		if !strings.Contains(manifest, hex.EncodeToString(sum[:])+"  "+name) {
			t.Errorf("manifest misses %s", name)
		}
		if strings.HasPrefix(name, "evidence/") {
			n++
		}
	}
	if n != evidence {
		t.Errorf("evidence files = %d, want %d", n, evidence)
	}
	for _, want := range []string{"snapshot-v1.json", "expenses-v1.csv", "travel-days-v1.csv", "mileage-v1.csv"} {
		if _, ok := files[want]; !ok {
			t.Errorf("zip misses %s", want)
		}
	}
}

func testLogWriter() io.Writer {
	if os.Getenv("SPESEN_TEST_LOG") != "" {
		return os.Stderr
	}
	return io.Discard
}
