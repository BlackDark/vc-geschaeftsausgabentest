package export

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

//go:embed templates/claim.typ
var templates embed.FS

//go:embed fonts/*.ttf
var fonts embed.FS

// ExtractFonts writes the bundled fonts to dir for Typst's --font-path.
func ExtractFonts(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	entries, err := fonts.ReadDir("fonts")
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := fonts.ReadFile("fonts/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o640); err != nil {
			return err
		}
	}
	return nil
}

// Artifacts are the generated, verified export files.
type Artifacts struct {
	PDFPath, ZipPath string
	PDFSHA, ZipSHA   string
}

// Evidence loads a receipt's original bytes.
type Evidence func(storageKey string) ([]byte, error)

// Build renders the claim PDF (summary plus receipt pages) and the ZIP with
// originals, snapshot JSON, CSVs and a SHA-256 manifest (P16).
func Build(ctx context.Context, t *Typst, snap *Snapshot, countries map[string]string, evidence Evidence, dir string) (*Artifacts, error) {
	originals := map[string][]byte{}
	for _, r := range snap.Receipts {
		if r.Kind != "file" {
			continue
		}
		b, err := evidence(r.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("receipt %s: %w", r.Number, err)
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != r.SHA256 {
			return nil, fmt.Errorf("receipt %s: checksum mismatch", r.Number)
		}
		originals[r.ZipPath] = b
		p := filepath.Join(dir, filepath.FromSlash(r.ZipPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, b, 0o640); err != nil {
			return nil, err
		}
	}

	snapJSON, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	used := map[string]string{}
	for _, iso := range usedCountries(snap) {
		if n, ok := countries[iso]; ok {
			used[iso] = n
		}
	}
	data := map[string]any{
		"snapshot":    snap,
		"locale":      snap.Locale,
		"labels":      Labels(snap.Locale),
		"countries":   used,
		"sources":     sources(snap, Labels(snap.Locale)["rulesLine"].(string)),
		"generatedAt": formatStamp(snap.GeneratedAt, snap.Locale),
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	tpl, err := templates.ReadFile("templates/claim.typ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), dataJSON, 0o640); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "claim.typ"), tpl, 0o640); err != nil {
		return nil, err
	}
	pdfPath := filepath.Join(dir, "claim.pdf")
	if err := t.Compile(ctx, dir, "claim.typ", pdfPath, "pdf", 0); err != nil {
		return nil, err
	}
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, err
	}

	base := fmt.Sprintf("%s-v%d", snap.Claim.Number, snap.Claim.Version)
	payload := []struct {
		name string
		data []byte
	}{
		{base + ".pdf", pdf},
		{"snapshot-v1.json", snapJSON},
		{"expenses-v1.csv", expensesCSV(snap)},
		{"travel-days-v1.csv", daysCSV(snap)},
		{"mileage-v1.csv", mileageCSV(snap)},
		{"README.txt", []byte(readme)},
	}
	paths := make([]string, 0, len(originals))
	for p := range originals {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		payload = append(payload, struct {
			name string
			data []byte
		}{p, originals[p]})
	}
	var manifest strings.Builder
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range payload {
		sum := sha256.Sum256(f.data)
		fmt.Fprintf(&manifest, "%s  %s\n", hex.EncodeToString(sum[:]), f.name)
		if err := addZip(zw, f.name, f.data); err != nil {
			return nil, err
		}
	}
	if err := addZip(zw, "manifest-sha256.txt", []byte(manifest.String())); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	zipPath := filepath.Join(dir, base+".zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o640); err != nil {
		return nil, err
	}
	return &Artifacts{PDFPath: pdfPath, ZipPath: zipPath, PDFSHA: hexSum(pdf), ZipSHA: hexSum(buf.Bytes())}, nil
}

const readme = `Contents
- <number>-v<version>.pdf: claim summary with calculation details and all receipt pages
- snapshot-v1.json: frozen inputs, rates, results and evidence references (schema version 1)
- expenses-v1.csv, travel-days-v1.csv, mileage-v1.csv: tabular extracts
- evidence/: unchanged original receipt files
- manifest-sha256.txt: SHA-256 of every other file

CSV dialect: RFC 4180, comma separator, UTF-8 without BOM, header row,
ISO 8601 dates, decimal point, amounts in EUR unless a currency column says otherwise.
`

func addZip(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}

func hexSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func usedCountries(s *Snapshot) []string {
	set := map[string]bool{}
	for _, t := range s.Trips {
		for _, d := range t.Result.Days {
			set[d.Location.Country] = true
		}
		for _, n := range t.Result.Nights {
			set[n.Location.Country] = true
		}
		for _, st := range t.Stops {
			set[st.Location.Country] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

func sources(s *Snapshot, rulesLine string) []string {
	set := map[string]bool{}
	for _, r := range s.Rates {
		set[fmt.Sprintf("%s – %d, Rev. %d", r.Source, r.Year, r.Revision)] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return append(out, rulesLine)
}

func formatStamp(rfc string, locale string) string {
	if len(rfc) < 16 {
		return rfc
	}
	if locale == "de" {
		return rfc[8:10] + "." + rfc[5:7] + "." + rfc[0:4] + " " + rfc[11:16] + " UTC"
	}
	return rfc[0:10] + " " + rfc[11:16] + " UTC"
}

func dec(c calc.Cents) string { return c.String() }

func minorString(v int64, exp int) string {
	if exp == 0 {
		return fmt.Sprint(v)
	}
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	p := int64(1)
	for range exp {
		p *= 10
	}
	return fmt.Sprintf("%s%d.%0*d", sign, v/p, exp, v%p)
}

func writeCSV(rows [][]string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.WriteAll(rows)
	return buf.Bytes()
}

func expensesCSV(s *Snapshot) []byte {
	rows := [][]string{{"claim_number", "claim_version", "expense_id", "trip_id", "standalone", "date", "cost_type", "payer", "supplier", "description", "currency", "gross_original", "fx_rate", "eur_debit", "gross_eur", "reimbursable_eur", "zero_reason", "vat_eur", "receipts"}}
	add := func(e Expense, standalone bool) {
		var vat calc.Cents
		for _, v := range e.Result.VAT {
			vat += v.VAT
		}
		trip, debit := "", ""
		if e.TripID != 0 {
			trip = fmt.Sprint(e.TripID)
		}
		if e.EURDebit != nil {
			debit = calc.Cents(*e.EURDebit).String()
		}
		rows = append(rows, []string{s.Claim.Number, fmt.Sprint(s.Claim.Version), fmt.Sprint(e.ID), trip, fmt.Sprint(standalone), e.Date, string(e.CostType), string(e.Payer), e.Supplier, e.Description,
			e.Currency, minorString(e.GrossMinor, e.Exponent), e.FXRate, debit, dec(e.Result.GrossEUR), dec(e.Result.Reimbursable), e.Result.Reason, dec(vat), strings.Join(e.Receipts, ";")})
	}
	for _, t := range s.Trips {
		for _, e := range t.Expenses {
			add(e, false)
		}
	}
	for _, e := range s.Outlays {
		add(e, true)
	}
	return writeCSV(rows)
}

func daysCSV(s *Snapshot) []byte {
	rows := [][]string{{"claim_number", "claim_version", "trip_id", "date", "day_type", "country", "city", "absence_minutes", "allowance_eur", "deductions_eur", "after_deductions_eur", "payable_eur", "rate_source", "rate_revision", "rule_ids"}}
	for _, t := range s.Trips {
		for _, d := range t.Result.Days {
			var ded calc.Cents
			for _, x := range d.Deductions {
				ded += x.Amount
			}
			src, rev := "", ""
			if d.Rate != nil {
				src, rev = d.Rate.Source, fmt.Sprint(d.Rate.Revision)
			}
			rows = append(rows, []string{s.Claim.Number, fmt.Sprint(s.Claim.Version), fmt.Sprint(t.ID), d.Date.String(), string(d.Type), d.Location.Country, d.Location.City, fmt.Sprint(d.AbsenceMinutes),
				dec(d.Base), dec(ded), dec(d.AfterDeduction), dec(d.Payable), src, rev, strings.Join(d.RuleIDs, ";")})
		}
	}
	return writeCSV(rows)
}

func mileageCSV(s *Snapshot) []byte {
	rows := [][]string{{"claim_number", "claim_version", "trip_id", "journey_id", "date", "vehicle", "distance_km", "cents_per_km", "amount_eur"}}
	for _, t := range s.Trips {
		for _, j := range t.Result.Journeys {
			rows = append(rows, []string{s.Claim.Number, fmt.Sprint(s.Claim.Version), fmt.Sprint(t.ID), fmt.Sprint(j.JourneyID), j.Date.String(), string(j.Vehicle), minorString(j.DistanceMeters, 3), fmt.Sprint(j.CentsPerKm), dec(j.Amount)})
		}
	}
	return writeCSV(rows)
}
