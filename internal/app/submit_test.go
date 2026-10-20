package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// storedFiles lists the files under one storage area of the data directory.
func storedFiles(t *testing.T, a *App, area string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(filepath.Join(a.Cfg.DataDir, area), func(p string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// TestA23FailedExportKeepsDraft: when rendering fails, nothing of the
// submission survives (no number, no version, no export files, no staging)
// and a later submission works with the first number.
func TestA23FailedExportKeepsDraft(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)
	must(a.CreateExpense(ctx, u, ExpenseInput{Date: "2026-05-04", CostType: "tools_equipment", Payer: "user", Supplier: "Werkzeug", Currency: "EUR", GrossMinor: 1999, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "tools.png", 1)}}))
	c := must(a.CreateClaim(ctx, u, ClaimInput{PeriodStart: "2026-05-01", PeriodEnd: "2026-05-31"}))
	p := must(a.PreviewClaim(ctx, u, c.ID))

	bin := a.Typst.Bin
	a.Typst.Bin = "false" // exits 1, like a Typst crash
	_, err := a.SubmitClaim(ctx, u, c.ID, SubmitInput{Revision: p.Revision, AcknowledgeWarnings: true})
	a.Typst.Bin = bin
	if code(err) != "export.failed" {
		t.Fatalf("submit with failing Typst: %v", err)
	}
	after := must(a.GetClaim(ctx, u, c.ID))
	if after.Status != "draft" || after.Number != "" || after.CurrentVersion != 0 {
		t.Fatalf("after failed export: status %q, number %q, version %d", after.Status, after.Number, after.CurrentVersion)
	}
	if f := storedFiles(t, a, "exports"); len(f) != 0 {
		t.Fatalf("export files left behind: %v", f)
	}
	if f := storedFiles(t, a, "staging"); len(f) != 0 {
		t.Fatalf("staging files left behind: %v", f)
	}

	claim, _ := submit(t, a, u, c.ID)
	if claim.Number != "A-"+claim.Versions[0].SubmittedAt[:4]+"-001" {
		t.Fatalf("number after retry = %q, want the first number of the year", claim.Number)
	}
}

// TestA23StartupRemovesUnpublishedExports simulates a crash between storing
// the export files and committing: startup recovery removes what no claim
// version references and keeps published exports.
func TestA23StartupRemovesUnpublishedExports(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)
	must(a.CreateExpense(ctx, u, ExpenseInput{Date: "2026-05-04", CostType: "tools_equipment", Payer: "user", Supplier: "Werkzeug", Currency: "EUR", GrossMinor: 1999, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "tools.png", 1)}}))
	submit(t, a, u, must(a.CreateClaim(ctx, u, ClaimInput{PeriodStart: "2026-05-01", PeriodEnd: "2026-05-31"})).ID)
	published := storedFiles(t, a, "exports")

	orphan := filepath.Join(t.TempDir(), "orphan.pdf")
	if err := os.WriteFile(orphan, []byte("%PDF-1.7 unpublished"), 0o640); err != nil {
		t.Fatal(err)
	}
	must(a.Files.PutFile("exports", ".pdf", orphan, ""))
	must(a.Files.Staging("claim-99-v1"))
	if err := os.WriteFile(filepath.Join(a.Cfg.DataDir, "staging", "claim-99-v1", "data.json"), []byte("{}"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := a.recoverSubmissions(ctx); err != nil {
		t.Fatal(err)
	}
	if got := storedFiles(t, a, "exports"); len(got) != len(published) {
		t.Fatalf("exports after recovery = %v, want only %v", got, published)
	}
	if f := storedFiles(t, a, "staging"); len(f) != 0 {
		t.Fatalf("staging after recovery: %v", f)
	}
	if rep := must(a.Verify(ctx)); len(rep.Problems) != 0 {
		t.Fatalf("published exports damaged: %+v", rep.Problems)
	}
}

// TestA23ConcurrentSubmissions: submissions that race each other neither
// pay a day twice (C07 recheck inside the submission) nor share a number,
// and the same claim cannot be submitted twice.
func TestA23ConcurrentSubmissions(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)
	day := func(start, end string) *Trip {
		return must(a.CreateTrip(ctx, u, TripInput{Purpose: "Termin " + start, Start: "2026-05-04T" + start, StartTZ: "Europe/Berlin", End: "2026-05-04T" + end, EndTZ: "Europe/Berlin"}))
	}
	morning, afternoon := day("07:00", "11:00"), day("13:00", "18:00")
	claims := []*Claim{claimFor(t, a, u, "2026-05-04", morning.ID), claimFor(t, a, u, "2026-05-04", afternoon.ID)}

	// Both previews are taken before either submission, as two browser tabs would.
	revisions := make([]string, len(claims))
	for i, c := range claims {
		revisions[i] = must(a.PreviewClaim(ctx, u, c.ID)).Revision
	}
	results := make([]error, len(claims)+1)
	var wg sync.WaitGroup
	for i, c := range claims {
		wg.Go(func() {
			_, results[i] = a.SubmitClaim(ctx, u, c.ID, SubmitInput{Revision: revisions[i], AcknowledgeWarnings: true})
		})
	}
	// The afternoon claim a second time, from a third tab.
	wg.Go(func() {
		_, results[2] = a.SubmitClaim(ctx, u, claims[1].ID, SubmitInput{Revision: revisions[1], AcknowledgeWarnings: true})
	})
	wg.Wait()

	if code(results[2]) == "" && code(results[1]) == "" {
		t.Fatalf("the afternoon claim was submitted twice: %v, %v", results[1], results[2])
	}
	var paid int64
	numbers := map[string]bool{}
	for _, c := range claims {
		got := must(a.GetClaim(ctx, u, c.ID))
		if got.Status != "submitted" {
			// The losing submission saw a changed revision; a fresh preview submits it.
			got, _ = submit(t, a, u, c.ID)
		}
		if len(got.Versions) != 1 || numbers[got.Number] {
			t.Fatalf("claim %d: versions %d, number %q (seen: %v)", got.ID, len(got.Versions), got.Number, numbers)
		}
		numbers[got.Number] = true
		paid += int64(got.Versions[0].Payout)
	}
	if paid != 1400 {
		t.Fatalf("paid for the day = %d cents, want 1400 once", paid)
	}
}
