package server

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PlatanosVerdes/apex/internal/auth"
	"github.com/PlatanosVerdes/apex/internal/store"
)

// me tells the page who is signed in, if anyone, and how they can sign in.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	out := struct {
		User   *auth.User `json:"user"`
		SignIn string     `json:"signIn,omitempty"`
		Donate *Donate    `json:"donate,omitempty"`
	}{Donate: s.Donate}
	if u, ok := s.Sessions.Read(r); ok {
		out.User = &u
	}
	switch {
	case s.Google != nil:
		out.SignIn = "google"
	case s.DevLogin:
		out.SignIn = "dev"
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, out)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.Google == nil {
		http.NotFound(w, r)
		return
	}
	s.Google.Begin(w, r)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	if s.Google == nil {
		http.NotFound(w, r)
		return
	}
	u, err := s.Google.Finish(w, r)
	if err != nil {
		s.Metrics.Inc("apex_signins_total", "Sign-ins by outcome.", "outcome", "failed")
		s.Log.Warn("sign in", "err", err)
		http.Redirect(w, r, "/?signin=failed", http.StatusFound)
		return
	}
	s.signIn(w, r, u)
}

// devLogin signs in as a made-up rider (?as=a) so accounts can be tried without Google. It only
// answers when enabled and only to this machine.
func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	as := r.URL.Query().Get("as")
	if !s.DevLogin || !net.ParseIP(host).IsLoopback() || len(as) == 0 || len(as) > 20 {
		http.NotFound(w, r)
		return
	}
	s.signIn(w, r, auth.User{ID: auth.IDFor("dev", as), Email: as + "@dev.local", Name: "Rider " + strings.ToUpper(as)})
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request, u auth.User) {
	existed := s.Users.Exists(u.ID)
	d, err := s.Users.For(u.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	outcome := "returning"
	if !existed {
		outcome = "new_account"
	}
	s.Metrics.Inc("apex_signins_total", "Sign-ins by outcome.", "outcome", outcome)
	if err := d.SaveProfile(u); err != nil {
		s.Log.Warn("profile", "err", err)
	}
	if s.Owner != "" && strings.EqualFold(u.Email, s.Owner) {
		s.adopt(d)
	}
	s.Sessions.Issue(w, u)
	s.Log.Info("signed in", "user", u.ID)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.Sessions.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

// adopt moves what was saved before accounts existed into the owner's folder, once.
func (s *Server) adopt(d *store.UserData) {
	if s.LegacyDir == "" {
		return
	}
	routes, _ := filepath.Glob(filepath.Join(s.LegacyDir, "routes", "*.json"))
	for _, from := range routes {
		to := filepath.Join(d.Routes.Dir(), filepath.Base(from))
		if err := os.Rename(from, to); err != nil {
			s.Log.Warn("adopt route", "err", err)
		}
	}
	lists := filepath.Join(s.LegacyDir, "lists.json")
	if _, err := os.Stat(d.Lists.Path()); os.IsNotExist(err) {
		if err := os.Rename(lists, d.Lists.Path()); err != nil && !os.IsNotExist(err) {
			s.Log.Warn("adopt lists", "err", err)
		}
	}
}

// signedIn wraps a handler that needs the rider's own data.
func (s *Server) signedIn(h func(http.ResponseWriter, *http.Request, *store.UserData)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.Sessions.Read(r)
		if !ok {
			http.Error(w, "sign in to keep routes and lists", http.StatusUnauthorized)
			return
		}
		d, err := s.Users.For(u.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		h(w, r, d)
	}
}

// rider names who is asking, for budgets: their account, or their address when signed out.
func (s *Server) rider(r *http.Request) string {
	if u, ok := s.Sessions.Read(r); ok {
		return "user:" + u.ID
	}
	return "ip:" + s.clientIP(r)
}

// clientIP is the visitor's address. Behind the Cloudflare tunnel every request comes from the
// tunnel, so the address the proxy reports in ClientIPHeader is the one that counts.
func (s *Server) clientIP(r *http.Request) string {
	if s.ClientIPHeader != "" {
		if ip := r.Header.Get(s.ClientIPHeader); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sameOrigin refuses changes sent from another site's page. The session cookie is SameSite=Lax,
// which already keeps it off cross-site posts; this closes the gap for older browsers.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
					http.Error(w, "cross-site request", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// limited answers 429 to an address that asks more often than the public budget allows.
func (s *Server) limited(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Public != nil && !s.Public.Take(s.clientIP(r)) {
			s.Metrics.Inc("apex_rate_limited_total", "Requests refused for asking too often.")
			w.Header().Set("Retry-After", "5")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		h(w, r)
	}
}
