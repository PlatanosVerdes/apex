// Package auth signs riders in with Google and keeps them signed in with a signed cookie.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type User struct {
	// ID names the rider's folder: a hash of Google's subject, so it is stable and path-safe.
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Picture string `json:"picture,omitempty"`
}

// IDFor turns an identity provider's subject into a user ID.
func IDFor(provider, subject string) string {
	sum := sha256.Sum256([]byte(provider + ":" + subject))
	return hex.EncodeToString(sum[:12])
}

const (
	sessionCookie = "apex_session"
	sessionLife   = 30 * 24 * time.Hour
)

// Sessions issues and reads the session cookie: the user and an expiry, signed with HMAC-SHA256.
// Nothing is kept on the server, so a stolen cookie works until it expires; the key in the data
// folder can be rotated to sign everyone out.
type Sessions struct {
	key    []byte
	Secure bool
	now    func() time.Time
}

// OpenSessions loads the signing key from dir, creating one the first time.
func OpenSessions(dir string, secure bool) (*Sessions, error) {
	path := filepath.Join(dir, "session.key")
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		err = os.WriteFile(path, key, 0o600)
	}
	if err != nil {
		return nil, err
	}
	if len(key) < 32 {
		return nil, errors.New("session key under 32 bytes")
	}
	return &Sessions{key: key, Secure: secure, now: time.Now}, nil
}

type claims struct {
	User
	Expires int64 `json:"exp"`
}

func (s *Sessions) Issue(w http.ResponseWriter, u User) {
	expires := s.now().Add(sessionLife)
	s.set(w, sessionCookie, s.sign(claims{u, expires.Unix()}), expires)
}

func (s *Sessions) Read(r *http.Request) (User, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return User{}, false
	}
	var cl claims
	if !s.verify(c.Value, &cl) || s.now().Unix() > cl.Expires || cl.ID == "" {
		return User{}, false
	}
	return cl.User, true
}

func (s *Sessions) Clear(w http.ResponseWriter) {
	s.set(w, sessionCookie, "", time.Unix(0, 0))
}

func (s *Sessions) set(w http.ResponseWriter, name, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode,
	})
}

// sign encodes v as base64url JSON followed by "." and its HMAC.
func (s *Sessions) sign(v any) string {
	data, _ := json.Marshal(v)
	payload := base64.RawURLEncoding.EncodeToString(data)
	return payload + "." + s.mac(payload)
}

func (s *Sessions) verify(token string, v any) bool {
	payload, mac, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(mac), []byte(s.mac(payload))) {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	return err == nil && json.Unmarshal(data, v) == nil
}

func (s *Sessions) mac(payload string) string {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
