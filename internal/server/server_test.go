package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/martinsaul/lost/internal/config"
	"github.com/martinsaul/lost/internal/notify"
	"github.com/martinsaul/lost/internal/store"
)

// capturingNotifier records every message so tests can assert on delivery and
// pull the magic-link token out of the login email.
type capturingNotifier struct {
	mu   sync.Mutex
	msgs []notify.Message
}

func (c *capturingNotifier) Name() string { return "capture" }
func (c *capturingNotifier) Send(_ context.Context, m notify.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
	return nil
}
func (c *capturingNotifier) last() (notify.Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.msgs) == 0 {
		return notify.Message{}, false
	}
	return c.msgs[len(c.msgs)-1], true
}
func (c *capturingNotifier) waitFor(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		c.mu.Lock()
		got := len(c.msgs)
		c.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d messages", n)
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-]+)`)

func newTestServer(t *testing.T, mutators ...func(*config.Config)) (*httptest.Server, *capturingNotifier) {
	t.Helper()
	st, err := store.Open("sqlite://:memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cn := &capturingNotifier{}
	cfg := &config.Config{
		BaseURL:       "http://example.test",
		SessionSecret: "test",
		FromAddress:   "lost@example.test",
		FromName:      "Lost & Found",
		MagicLinkTTL:  15 * 60 * 1e9,
		SessionTTL:    24 * 60 * 60 * 1e9,
		Notifier:      "capture",
	}
	for _, m := range mutators {
		m(cfg)
	}
	spa := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html></html>")}}
	srv := New(cfg, st, cn, spa)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, cn
}

// signIn runs the magic-link flow and returns an authenticated client (cookie jar).
func signIn(t *testing.T, ts *httptest.Server, cn *capturingNotifier, email string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	before := len(cn.msgs)
	postJSON(t, client, ts.URL+"/api/auth/request", `{"email":"`+email+`"}`)
	cn.waitFor(t, before+1)
	login, _ := cn.last()
	m := tokenRe.FindStringSubmatch(login.Text + login.HTML)
	if m == nil {
		t.Fatalf("no token in login email for %s", email)
	}
	resp, err := client.Get(ts.URL + "/api/auth/verify?token=" + m[1])
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return client
}

func TestUserCap(t *testing.T) {
	ts, cn := newTestServer(t, func(c *config.Config) { c.MaxUsers = 1 })

	// First user registers fine.
	c1 := signIn(t, ts, cn, "a@example.test")
	var me struct{ Email string }
	getJSON(t, c1, ts.URL+"/api/me", &me)
	if me.Email != "a@example.test" {
		t.Fatalf("me %q", me.Email)
	}

	// A second, new user is blocked at verify (redirect carries the error, no session).
	jar, _ := cookiejar.New(nil)
	c2 := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	before := len(cn.msgs)
	postJSON(t, c2, ts.URL+"/api/auth/request", `{"email":"b@example.test"}`)
	cn.waitFor(t, before+1)
	login, _ := cn.last()
	m := tokenRe.FindStringSubmatch(login.Text + login.HTML)
	resp, _ := c2.Get(ts.URL + "/api/auth/verify?token=" + m[1])
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Location"), "registration") {
		t.Fatalf("capped user should be rejected, got location %q", resp.Header.Get("Location"))
	}
	r2, _ := c2.Get(ts.URL + "/api/me")
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("capped user should have no session, got %d", r2.StatusCode)
	}
	r2.Body.Close()

	// The existing user can still sign in.
	_ = signIn(t, ts, cn, "a@example.test")
}

func TestAdmin(t *testing.T) {
	ts, cn := newTestServer(t, func(c *config.Config) { c.AdminEmails = []string{"boss@example.test"} })

	// A normal user is not admin and is forbidden from admin endpoints.
	u := signIn(t, ts, cn, "user@example.test")
	var me struct {
		IsAdmin bool `json:"isAdmin"`
	}
	getJSON(t, u, ts.URL+"/api/me", &me)
	if me.IsAdmin {
		t.Fatal("normal user should not be admin")
	}
	resp, _ := u.Get(ts.URL + "/api/admin/users")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin /api/admin/users = %d, want 403", resp.StatusCode)
	}

	// The allowlisted user is admin and can list users + read stats.
	a := signIn(t, ts, cn, "boss@example.test")
	getJSON(t, a, ts.URL+"/api/me", &me)
	if !me.IsAdmin {
		t.Fatal("boss should be admin")
	}
	var list struct{ Users []map[string]any }
	getJSON(t, a, ts.URL+"/api/admin/users", &list)
	if len(list.Users) != 2 {
		t.Fatalf("admin users = %d, want 2", len(list.Users))
	}
	var stats map[string]any
	getJSON(t, a, ts.URL+"/api/admin/stats", &stats)
	if int(stats["users"].(float64)) != 2 {
		t.Fatalf("stats users = %v", stats["users"])
	}
}

func TestReReportThrottleAndLocation(t *testing.T) {
	ts, cn := newTestServer(t, func(c *config.Config) {
		c.FinderMinInterval = time.Hour
		c.FinderDailyCap = 6
	})
	owner := signIn(t, ts, cn, "owner@example.test")
	var tag tagDTO
	postJSONInto(t, owner, ts.URL+"/api/tags", `{"name":"bag"}`, &tag)

	// Finder keeps a cookie jar so the finder key persists across submits.
	jar, _ := cookiejar.New(nil)
	finder := &http.Client{Jar: jar}
	var pub foundPublicDTO
	getJSON(t, finder, ts.URL+"/api/found/"+tag.GUID, &pub) // sets finder cookie + records a scan

	before := len(cn.msgs)
	postJSON(t, finder, ts.URL+"/api/found/"+tag.GUID,
		`{"message":"found near the park","hasLocation":true,"lat":47.61,"lon":-122.33,"accuracy":12}`)
	cn.waitFor(t, before+1)
	report, _ := cn.last()
	if !strings.Contains(report.Text, "Precise location") || !strings.Contains(report.Text, "google.com/maps") {
		t.Fatalf("owner email missing precise location: %q", report.Text)
	}

	// An immediate second submit is throttled (within the 1h interval).
	resp, err := finder.Post(ts.URL+"/api/found/"+tag.GUID, "application/json",
		strings.NewReader(`{"message":"again"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second submit = %d, want 429", resp.StatusCode)
	}
}

func TestFullFlow(t *testing.T) {
	ts, cn := newTestServer(t)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // don't follow the /app redirect
	}}

	// 1. Request a magic link.
	postJSON(t, client, ts.URL+"/api/auth/request", `{"email":"Owner@Example.test"}`)
	cn.waitFor(t, 1)
	login, _ := cn.last()
	m := tokenRe.FindStringSubmatch(login.Text + login.HTML)
	if m == nil {
		t.Fatalf("no token in login email: %q", login.Text)
	}
	rawToken := m[1]
	if login.To != "owner@example.test" {
		t.Fatalf("login sent to %q, want normalized lowercase", login.To)
	}

	// 2. Verify -> sets session cookie.
	resp, err := client.Get(ts.URL + "/api/auth/verify?token=" + rawToken)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("verify status %d", resp.StatusCode)
	}

	// 3. /api/me now works.
	var me struct{ Email string }
	getJSON(t, client, ts.URL+"/api/me", &me)
	if me.Email != "owner@example.test" {
		t.Fatalf("me email %q", me.Email)
	}

	// 4. Re-using the same token must fail.
	resp2, _ := client.Get(ts.URL + "/api/auth/verify?token=" + rawToken)
	resp2.Body.Close()
	// Redirects to /?auth_error=...
	if !strings.Contains(resp2.Header.Get("Location"), "auth_error") {
		t.Fatalf("token reuse should error, got location %q", resp2.Header.Get("Location"))
	}

	// 5. Create a tag with public phone opted in.
	var tag tagDTO
	postJSONInto(t, client, ts.URL+"/api/tags",
		`{"name":"Martin's large luggage","showPhone":true,"phone":"+1-555-0100"}`, &tag)
	if tag.GUID == "" || !tag.ShowPhone {
		t.Fatalf("bad tag: %+v", tag)
	}
	if !strings.HasSuffix(tag.FoundURL, "/found/"+tag.GUID) {
		t.Fatalf("found url %q", tag.FoundURL)
	}

	// 6. QR endpoints return the right content types.
	assertContentType(t, client, ts.URL+"/api/tags/"+tag.GUID+"/qr.svg", "image/svg+xml")
	assertContentType(t, client, ts.URL+"/api/tags/"+tag.GUID+"/qr.png", "image/png")

	// 7. Public found page shows the opted-in phone but never the owner email.
	var pub foundPublicDTO
	getJSON(t, http.DefaultClient, ts.URL+"/api/found/"+tag.GUID, &pub)
	if pub.Name != "Martin's large luggage" {
		t.Fatalf("public name %q", pub.Name)
	}
	if pub.OwnerPhone != "+1-555-0100" {
		t.Fatalf("public phone %q", pub.OwnerPhone)
	}
	if pub.OwnerEmail != "" {
		t.Fatalf("owner email leaked: %q", pub.OwnerEmail)
	}

	// 8. Finder submits -> owner gets a notification email, finder set as Reply-To.
	before := len(cn.msgs)
	postJSON(t, http.DefaultClient, ts.URL+"/api/found/"+tag.GUID,
		`{"message":"Found it at the airport!","email":"finder@example.test","name":"Sam"}`)
	cn.waitFor(t, before+1)
	report, _ := cn.last()
	if report.To != "owner@example.test" {
		t.Fatalf("report sent to %q", report.To)
	}
	if report.ReplyTo != "finder@example.test" {
		t.Fatalf("reply-to %q", report.ReplyTo)
	}
	if !strings.Contains(report.Text, "Found it at the airport!") {
		t.Fatalf("report missing message: %q", report.Text)
	}

	// 9. Honeypot submissions are silently dropped (no new email).
	before = len(cn.msgs)
	postJSON(t, http.DefaultClient, ts.URL+"/api/found/"+tag.GUID,
		`{"message":"spam","website":"http://spam"}`)
	time.Sleep(50 * time.Millisecond)
	if len(cn.msgs) != before {
		t.Fatalf("honeypot submission should not notify")
	}
}

// ---- tiny HTTP test helpers ----

func postJSON(t *testing.T, c *http.Client, url, body string) {
	t.Helper()
	resp, err := c.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s -> %d", url, resp.StatusCode)
	}
}

func postJSONInto(t *testing.T, c *http.Client, url, body string, v any) {
	t.Helper()
	resp, err := c.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s -> %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, c *http.Client, url string, v any) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("GET %s -> %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func assertContentType(t *testing.T, c *http.Client, url, want string) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s -> %d", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, want) {
		t.Fatalf("GET %s content-type %q, want %q", url, ct, want)
	}
}
