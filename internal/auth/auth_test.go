package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	s, err := OpenSessions(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	u := User{ID: IDFor("google", "123"), Email: "rider@example.com"}
	rec := httptest.NewRecorder()
	s.Issue(rec, u)
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: %+v", cookie)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	if got, ok := s.Read(req); !ok || got != u {
		t.Fatalf("%+v %v", got, ok)
	}

	// Edited, forged or expired cookies are refused.
	payload, mac, _ := strings.Cut(cookie.Value, ".")
	data, _ := base64.RawURLEncoding.DecodeString(payload)
	edited := strings.Replace(string(data), "rider@", "admin@", 1)
	for name, value := range map[string]string{
		"edited":  base64.RawURLEncoding.EncodeToString([]byte(edited)) + "." + mac,
		"no mac":  payload,
		"garbage": "x.y",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
		if _, ok := s.Read(req); ok {
			t.Errorf("%s cookie accepted", name)
		}
	}
	s.now = func() time.Time { return time.Now().Add(31 * 24 * time.Hour) }
	if _, ok := s.Read(req); ok {
		t.Error("an expired session was accepted")
	}

	other, _ := OpenSessions(t.TempDir(), true)
	if _, ok := other.Read(req); ok {
		t.Error("a cookie signed with another key was accepted")
	}
}

func TestSessionKeyPersists(t *testing.T) {
	dir := t.TempDir()
	a, _ := OpenSessions(dir, false)
	b, _ := OpenSessions(dir, false)
	rec := httptest.NewRecorder()
	a.Issue(rec, User{ID: "x", Email: "a@b.c"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	if _, ok := b.Read(req); !ok {
		t.Error("a restart must keep riders signed in")
	}
}

func idToken(claims map[string]any) string {
	data, _ := json.Marshal(claims)
	return "e30." + base64.RawURLEncoding.EncodeToString(data) + ".sig"
}

func TestGoogleFlow(t *testing.T) {
	sessions, _ := OpenSessions(t.TempDir(), false)
	var form url.Values
	claims := map[string]any{"iss": "https://accounts.google.com", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(),
		"sub": "42", "email": "rider@example.com", "email_verified": true, "name": "Rider"}
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": idToken(claims)})
	}))
	defer token.Close()
	g := NewGoogle("client", "secret", "https://apex.example/auth/callback", sessions)
	g.tokenURL = token.URL

	begin := httptest.NewRecorder()
	g.Begin(begin, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	to, _ := url.Parse(begin.Header().Get("Location"))
	if to.Host != "accounts.google.com" || to.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("redirect: %s", to)
	}
	state := to.Query().Get("state")
	flowCookie := begin.Result().Cookies()[0]

	back := func(state string, cookie *http.Cookie) (User, error) {
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=c0de&state="+url.QueryEscape(state), nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		return g.Finish(httptest.NewRecorder(), req)
	}
	if _, err := back("other", flowCookie); err == nil {
		t.Error("a different state was accepted")
	}
	if _, err := back(state, nil); err == nil {
		t.Error("a callback without the sign-in cookie was accepted")
	}
	u, err := back(state, flowCookie)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "rider@example.com" || u.ID != IDFor("google", "42") || form.Get("code_verifier") == "" {
		t.Errorf("%+v, verifier sent: %q", u, form.Get("code_verifier"))
	}

	claims["aud"] = "someone-else"
	if _, err := back(state, flowCookie); err == nil {
		t.Error("a token for another app was accepted")
	}
	claims["aud"], claims["email_verified"] = "client", false
	if _, err := back(state, flowCookie); err == nil {
		t.Error("an unverified email was accepted")
	}
}
