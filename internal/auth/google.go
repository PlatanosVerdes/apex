package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	flowCookie     = "apex_login"
)

// Google signs riders in with OpenID Connect: authorisation code with PKCE, the state and the code
// verifier kept in a short-lived signed cookie, and the ID token taken straight from Google's
// token endpoint over TLS, which OpenID Connect accepts without checking its signature.
type Google struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Sessions     *Sessions

	tokenURL string
	http     *http.Client
}

func NewGoogle(clientID, secret, redirect string, sessions *Sessions) *Google {
	return &Google{
		ClientID: clientID, ClientSecret: secret, RedirectURL: redirect, Sessions: sessions,
		tokenURL: googleTokenURL, http: &http.Client{Timeout: 15 * time.Second},
	}
}

type flow struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
	Expires  int64  `json:"exp"`
}

// Begin sends the rider to Google's consent screen.
func (g *Google) Begin(w http.ResponseWriter, r *http.Request) {
	f := flow{State: random(), Verifier: random(), Expires: g.Sessions.now().Add(10 * time.Minute).Unix()}
	g.Sessions.set(w, flowCookie, g.Sessions.sign(f), time.Unix(f.Expires, 0))
	challenge := sha256.Sum256([]byte(f.Verifier))
	q := url.Values{
		"client_id":             {g.ClientID},
		"redirect_uri":          {g.RedirectURL},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {f.State},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusFound)
}

// Finish handles Google's redirect back and returns who signed in.
func (g *Google) Finish(w http.ResponseWriter, r *http.Request) (User, error) {
	c, err := r.Cookie(flowCookie)
	g.Sessions.set(w, flowCookie, "", time.Unix(0, 0))
	if err != nil {
		return User{}, errors.New("sign-in took too long or started elsewhere")
	}
	var f flow
	if !g.Sessions.verify(c.Value, &f) || g.Sessions.now().Unix() > f.Expires {
		return User{}, errors.New("sign-in took too long")
	}
	if r.URL.Query().Get("state") != f.State || f.State == "" {
		return User{}, errors.New("sign-in state does not match")
	}
	if e := r.URL.Query().Get("error"); e != "" {
		return User{}, fmt.Errorf("google: %s", e)
	}
	return g.exchange(r.Context(), r.URL.Query().Get("code"), f.Verifier)
}

func (g *Google) exchange(ctx context.Context, code, verifier string) (User, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"redirect_uri":  {g.RedirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.http.Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	var body struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.IDToken == "" {
		return User{}, fmt.Errorf("google token: %s %s", resp.Status, body.Error)
	}
	return g.user(body.IDToken)
}

func (g *Google) user(idToken string) (User, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return User{}, errors.New("google: malformed id token")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return User{}, err
	}
	var t struct {
		Iss           string `json:"iss"`
		Aud           string `json:"aud"`
		Exp           int64  `json:"exp"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return User{}, err
	}
	switch {
	case t.Iss != "https://accounts.google.com" && t.Iss != "accounts.google.com":
		return User{}, errors.New("google: wrong issuer")
	case t.Aud != g.ClientID:
		return User{}, errors.New("google: token for another app")
	case g.Sessions.now().Unix() > t.Exp:
		return User{}, errors.New("google: expired token")
	case t.Sub == "" || !t.EmailVerified:
		return User{}, errors.New("google: account without a verified email")
	}
	return User{ID: IDFor("google", t.Sub), Email: t.Email, Name: t.Name, Picture: t.Picture}, nil
}

func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
