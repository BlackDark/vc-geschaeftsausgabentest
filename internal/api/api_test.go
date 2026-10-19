package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/app"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/db"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/export"
)

type testServer struct {
	*httptest.Server
	app *app.App
}

func newServer(t *testing.T) *testServer {
	t.Helper()
	typst := os.Getenv("SPESEN_TYPST")
	if typst == "" {
		p, err := exec.LookPath("typst")
		if err != nil {
			t.Skip("typst not installed; set SPESEN_TYPST")
		}
		typst = p
	}
	dir := t.TempDir()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fonts := filepath.Join(dir, "fonts")
	if err := export.ExtractFonts(fonts); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(ctx, conn, app.Config{DataDir: filepath.Join(dir, "data"), Typst: typst, FontDir: fonts, MaxUploadBytes: 25 << 20, MaxPixels: 40e6, MaxPDFPages: 50}, log)
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(a, Config{BaseURL: "http://example.test", SetupToken: "setup-token", PasswordEnabled: true, Version: "test"}, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &testServer{Server: ts, app: a}
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (s *testServer) client(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: s.URL, http: &http.Client{Jar: jar}}
}

// do sends a JSON request and decodes the response into out (if not nil).
func (c *client) do(method, path string, in, out any) int {
	c.t.Helper()
	var body io.Reader
	if b, ok := in.([]byte); ok {
		body = bytes.NewReader(b)
	} else if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, body)
	if _, ok := in.([]byte); ok {
		req.Header.Set("Content-Type", "application/octet-stream")
	} else if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode
}

func receiptPNG(n int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	img.Set(n%40, n%60, color.Black)
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

type id struct {
	ID int64 `json:"id"`
}

// seed creates an employer, a trip with an expense and receipt, and a
// submitted claim for the logged-in client.
func seed(t *testing.T, c *client, n int) (trip, expense, receipt, claim int64) {
	t.Helper()
	if s := c.do("PUT", "/api/employer", map[string]any{"name": "Firma"}, nil); s != 200 {
		t.Fatalf("employer: %d", s)
	}
	var td struct {
		Trip id `json:"trip"`
	}
	if s := c.do("POST", "/api/trips", map[string]any{"purpose": "Termin", "start": "2026-05-04T07:00", "startTz": "Europe/Berlin", "end": "2026-05-04T18:00", "endTz": "Europe/Berlin"}, &td); s != 201 {
		t.Fatalf("trip: %d", s)
	}
	var r id
	if s := c.do("POST", "/api/receipts?filename=r.png", receiptPNG(n), &r); s != 201 {
		t.Fatalf("upload: %d", s)
	}
	c.do("POST", fmt.Sprintf("/api/receipts/%d/confirm", r.ID), nil, nil)
	var e id
	if s := c.do("POST", "/api/expenses", map[string]any{"tripId": td.Trip.ID, "date": "2026-05-04", "costType": "transport", "payer": "user", "currency": "EUR", "grossMinor": 2500, "vatUnknown": true, "receiptIds": []int64{r.ID}}, &e); s != 201 {
		t.Fatalf("expense: %d", s)
	}
	var cl id
	if s := c.do("POST", "/api/claims", map[string]any{"periodStart": "2026-05-01", "periodEnd": "2026-05-31"}, &cl); s != 201 {
		t.Fatalf("claim: %d", s)
	}
	var p struct {
		Revision string `json:"revision"`
	}
	c.do("GET", fmt.Sprintf("/api/claims/%d", cl.ID), nil, &p)
	if s := c.do("POST", fmt.Sprintf("/api/claims/%d/submit", cl.ID), map[string]any{"revision": p.Revision, "acknowledgeWarnings": true}, nil); s != 200 {
		t.Fatalf("submit: %d", s)
	}
	return td.Trip.ID, e.ID, r.ID, cl.ID
}

// TestA17PasswordLoginAndDeactivation covers setup, login limits, logout
// and deactivation ending existing sessions.
func TestA17PasswordLoginAndDeactivation(t *testing.T) {
	s := newServer(t)
	admin := s.client(t)
	if st := admin.do("POST", "/api/setup", map[string]any{"token": "wrong", "username": "admin", "name": "Admin", "password": "correct horse battery"}, nil); st != 422 {
		t.Fatalf("setup with wrong token: %d", st)
	}
	if st := admin.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "admin", "name": "Admin", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	if st := s.client(t).do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "x", "name": "X", "password": "correct horse battery"}, nil); st != 409 {
		t.Fatalf("second setup: %d", st)
	}
	var b id
	if st := admin.do("POST", "/api/admin/users", map[string]any{"username": "bob", "name": "Bob", "password": "bobs secret pw"}, &b); st != 201 {
		t.Fatalf("create user: %d", st)
	}
	bob := s.client(t)
	for i := 0; i < 5; i++ {
		if st := bob.do("POST", "/api/login", map[string]any{"username": "bob", "password": "wrong password"}, nil); st != 401 {
			t.Fatalf("wrong password %d: %d", i, st)
		}
	}
	if st := bob.do("POST", "/api/login", map[string]any{"username": "bob", "password": "bobs secret pw"}, nil); st != 429 {
		t.Fatalf("rate limit: %d", st)
	}
	s.app.DB.Exec(`DELETE FROM login_failures`)
	if st := bob.do("POST", "/api/login", map[string]any{"username": "bob", "password": "bobs secret pw"}, nil); st != 200 {
		t.Fatalf("login: %d", st)
	}
	if st := bob.do("GET", "/api/me", nil, nil); st != 200 {
		t.Fatalf("me: %d", st)
	}
	if st := admin.do("POST", fmt.Sprintf("/api/admin/users/%d/active", b.ID), map[string]any{"active": false, "reason": "left"}, nil); st != 204 {
		t.Fatalf("deactivate: %d", st)
	}
	if st := bob.do("GET", "/api/me", nil, nil); st != 401 {
		t.Fatalf("deactivated session still valid: %d", st)
	}
	if st := admin.do("POST", "/api/admin/users/1/active", map[string]any{"active": false}, nil); st != 422 {
		t.Fatalf("last admin deactivated: %d", st)
	}
	if st := admin.do("POST", "/api/logout", nil, nil); st != 200 {
		t.Fatalf("logout: %d", st)
	}
	if st := admin.do("GET", "/api/me", nil, nil); st != 401 {
		t.Fatalf("after logout: %d", st)
	}
}

// TestA20Ownership checks that user B and admin A cannot read or change
// user C's data, including nested references and downloads.
func TestA20Ownership(t *testing.T) {
	s := newServer(t)
	admin := s.client(t)
	admin.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "admin", "name": "Admin", "password": "correct horse battery"}, nil)
	admin.do("POST", "/api/admin/users", map[string]any{"username": "carol", "name": "Carol", "password": "carols secret"}, nil)
	admin.do("POST", "/api/admin/users", map[string]any{"username": "bob", "name": "Bob", "password": "bobs secret pw"}, nil)
	carol, bob := s.client(t), s.client(t)
	carol.do("POST", "/api/login", map[string]any{"username": "carol", "password": "carols secret"}, nil)
	bob.do("POST", "/api/login", map[string]any{"username": "bob", "password": "bobs secret pw"}, nil)
	trip, expense, receipt, claim := seed(t, carol, 1)
	bob.do("PUT", "/api/employer", map[string]any{"name": "Bobs Firma"}, nil)

	for _, who := range []*client{bob, admin} {
		for _, p := range []string{
			fmt.Sprintf("/api/trips/%d", trip), fmt.Sprintf("/api/expenses/%d", expense), fmt.Sprintf("/api/receipts/%d", receipt),
			fmt.Sprintf("/api/receipts/%d/file", receipt), fmt.Sprintf("/api/receipts/%d/preview", receipt), fmt.Sprintf("/api/claims/%d", claim),
			fmt.Sprintf("/api/claims/%d/versions/1/pdf", claim), fmt.Sprintf("/api/claims/%d/versions/1/zip", claim),
		} {
			if st := who.do("GET", p, nil, nil); st != 404 {
				t.Errorf("GET %s: %d, want 404", p, st)
			}
		}
		var trips, receipts, claims []any
		who.do("GET", "/api/trips", nil, &trips)
		who.do("GET", "/api/receipts", nil, &receipts)
		who.do("GET", "/api/claims", nil, &claims)
		if len(trips)+len(receipts)+len(claims) != 0 {
			t.Errorf("lists leak: %d %d %d", len(trips), len(receipts), len(claims))
		}
		if st := who.do("DELETE", fmt.Sprintf("/api/trips/%d", trip), nil, nil); st != 404 {
			t.Errorf("delete foreign trip: %d", st)
		}
		if st := who.do("POST", fmt.Sprintf("/api/receipts/%d/void", receipt), map[string]any{"reason": "x"}, nil); st != 404 {
			t.Errorf("void foreign receipt: %d", st)
		}
		var audit []any
		who.do("GET", "/api/audit", nil, &audit)
		for _, e := range audit {
			if m := e.(map[string]any); m["entity"] == "trip" && int64(m["entityId"].(float64)) == trip {
				t.Error("audit leaks foreign trip")
			}
		}
	}
	// Nested references: Bob links Carol's receipt or adds an expense to her trip.
	if st := bob.do("POST", "/api/expenses", map[string]any{"tripId": trip, "date": "2026-05-04", "costType": "transport", "payer": "user", "currency": "EUR", "grossMinor": 100}, nil); st != 404 {
		t.Errorf("expense on foreign trip: %d", st)
	}
	if st := bob.do("POST", "/api/expenses", map[string]any{"date": "2026-05-04", "costType": "other_outlay", "payer": "user", "currency": "EUR", "grossMinor": 100, "receiptIds": []int64{receipt}}, nil); st != 422 {
		t.Errorf("link foreign receipt: %d", st)
	}
	var bc id
	bob.do("POST", "/api/claims", map[string]any{"periodStart": "2026-05-01", "periodEnd": "2026-05-31"}, &bc)
	if st := bob.do("POST", fmt.Sprintf("/api/claims/%d/items", bc.ID), map[string]any{"kind": "trip", "id": trip, "include": true}, nil); st != 404 {
		t.Errorf("add foreign trip to claim: %d", st)
	}
	// Carol's submitted trip is locked server-side.
	edit := map[string]any{"purpose": "changed", "start": "2026-05-04T07:00", "startTz": "Europe/Berlin", "end": "2026-05-04T18:00", "endTz": "Europe/Berlin"}
	if st := carol.do("PUT", fmt.Sprintf("/api/trips/%d", trip), edit, nil); st != 409 {
		t.Errorf("edit submitted trip: %d", st)
	}
}

func TestCrossOriginRequestRejected(t *testing.T) {
	s := newServer(t)
	req, _ := http.NewRequest("POST", s.URL+"/api/login", bytes.NewReader([]byte(`{"username":"a","password":"b"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-site POST: %d", resp.StatusCode)
	}
}
