package rates

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/blackdark/vc-geschaeftsausgabentest/data"
)

func countries(t *testing.T) map[string]string {
	t.Helper()
	b, err := fs.ReadFile(data.Files, "country-codes.csv")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCountries(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInheritedDatasetsValidate(t *testing.T) {
	c := countries(t)
	want := map[int]int{2024: 223, 2025: 224, 2026: 214}
	for year, n := range want {
		rows, err := readForeign(year, c)
		if err != nil {
			t.Fatalf("%d: %v", year, err)
		}
		if len(rows) != n {
			t.Errorf("%d: %d rows, want %d", year, len(rows), n)
		}
	}
}

func TestImportRejectsMalformed(t *testing.T) {
	c := countries(t)
	head := "jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur\n"
	cases := map[string]string{
		"wrong year": head + "2027,Albanien,,33,22,116\n",
		"unknown":    head + "2026,Atlantis,,33,22,116\n",
		"duplicate":  head + "2026,Albanien,,33,22,116\n2026,Albanien,,33,22,116\n",
		"negative":   head + "2026,Albanien,,-3,22,116\n",
		"decimal":    head + "2026,Albanien,,3.5,22,116\n",
		"header":     "year,country\n",
	}
	for name, in := range cases {
		if _, err := ParseForeign(strings.NewReader(in), 2026, c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
