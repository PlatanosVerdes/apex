// Package server serves the route planner page and its JSON API.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PlatanosVerdes/apex/internal/auth"
	"github.com/PlatanosVerdes/apex/internal/limit"
	"github.com/PlatanosVerdes/apex/internal/metrics"
	"github.com/PlatanosVerdes/apex/internal/nominatim"
	"github.com/PlatanosVerdes/apex/internal/photon"
	"github.com/PlatanosVerdes/apex/internal/plan"
	"github.com/PlatanosVerdes/apex/internal/store"
	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

//go:embed web
var web embed.FS

type Planner interface {
	Plan(ctx context.Context, pts []valhalla.Point, legs []plan.LegOptions, avoid valhalla.Avoid, allow func() bool) (plan.Plan, error)
}

type Places interface {
	Name(ctx context.Context, lat, lng float64) (nominatim.Label, error)
}

type Search interface {
	Search(ctx context.Context, q string, near *[2]float64) ([]photon.Result, error)
}

type Server struct {
	Search   Search
	Users    *store.Users
	Sessions *auth.Sessions
	// Google signs riders in; nil leaves only DevLogin, or no accounts at all.
	Google *auth.Google
	// DevLogin lets this machine sign in as made-up riders, to try accounts without Google.
	DevLogin bool
	// Owner is the email that, on signing in, takes over what LegacyDir held from before accounts.
	Owner string
	// Donate invites riders to chip in after they take a route out; nil shows no invitation.
	Donate     *Donate
	LegacyDir  string
	ListSource ListSource
	Planner    Planner
	// Curves is each rider's budget of curve searches; Public each address's budget of requests
	// to the endpoints anyone can call. Nil means no limit.
	Curves *limit.Budget
	Public *limit.Budget
	// ClientIPHeader names the header the proxy in front puts the visitor's address in
	// (CF-Connecting-IP behind Cloudflare). Empty means the TCP peer is the visitor.
	ClientIPHeader string
	Places         Places
	// Metrics counts what happens; nil counts nothing.
	Metrics *metrics.Registry
	Version string
	Log     *slog.Logger
}

type Donate struct {
	URL string `json:"url"`
	// Chance is how likely taking a route out asks, from 0 to 1.
	Chance float64 `json:"chance"`
	// EverySeconds is the least time between two invitations to the same browser.
	EverySeconds int64 `json:"everySeconds"`
	// AtStops also asks when a route reaches this many stops; zero never does.
	AtStops int `json:"atStops"`
	// AfterUses is how many routes a browser takes out or draws long before it is first asked.
	AfterUses int `json:"afterUses"`
}

func (s *Server) Handler() http.Handler {
	static, _ := fs.Sub(web, "web")
	files := assets(static)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		a, ok := files[name]
		if !ok {
			// Cloudflare would otherwise keep the 404 for hours, past the deploy that adds the file.
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		serveAsset(w, r, name, a)
	})
	// A route link (/r/lat,lng/lat,lng/...?options) opens the page itself, which reads the route from it.
	mux.HandleFunc("GET /r/{points...}", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, r, "index.html", files["index.html"])
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok "+s.Version+"\n")
	})
	mux.HandleFunc("GET /auth/login", s.login)
	mux.HandleFunc("GET /auth/callback", s.callback)
	mux.HandleFunc("GET /auth/dev", s.devLogin)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("POST /api/plan", s.limited(s.plan))
	mux.HandleFunc("GET /api/place", s.limited(s.place))
	mux.HandleFunc("GET /api/search", s.limited(s.search))
	mux.HandleFunc("GET /api/routes", s.signedIn(s.listRoutes))
	mux.HandleFunc("POST /api/routes", s.signedIn(s.saveRoute))
	mux.HandleFunc("GET /api/routes/{id}", s.signedIn(s.getRoute))
	mux.HandleFunc("PUT /api/routes/{id}", s.signedIn(s.saveRoute))
	mux.HandleFunc("DELETE /api/routes/{id}", s.signedIn(s.deleteRoute))
	mux.HandleFunc("POST /api/donate", s.limited(s.donateEvent))
	mux.HandleFunc("GET /api/lists", s.signedIn(s.listLists))
	mux.HandleFunc("POST /api/lists", s.signedIn(s.addList))
	mux.HandleFunc("POST /api/lists/read", s.limited(s.readList))
	mux.HandleFunc("POST /api/lists/{id}/refresh", s.signedIn(s.refreshList))
	mux.HandleFunc("PATCH /api/lists/{id}", s.signedIn(s.setListColour))
	mux.HandleFunc("DELETE /api/lists/{id}", s.signedIn(s.deleteList))
	return secure(sameOrigin(gzipJSON(s.counted(mux))), s.Sessions.Secure)
}

// secure sets the headers a public page is expected to send. With HTTPS on, a visit that came in over
// plain HTTP (as the proxy reports it) is sent to HTTPS first: the session cookie only travels there.
func secure(next http.Handler, httpsOnly bool) http.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		// Google serves the rider's account picture from here.
		"img-src 'self' data: blob: https://server.arcgisonline.com https://lh3.googleusercontent.com",
		// OpenFreeMap's styles, vector tiles, fonts and sprites, and Esri's satellite imagery: MapLibre
		// fetches them all from a worker.
		"connect-src 'self' https://tiles.openfreemap.org https://server.arcgisonline.com",
		"worker-src blob:",
		"child-src blob:",
		// 'self' for the <base> that keeps relative URLs working under /r/....
		"base-uri 'self'",
		"form-action 'none'",
		"object-src 'none'",
		"frame-ancestors 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if httpsOnly {
			if r.Header.Get("X-Forwarded-Proto") == "http" {
				http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusMovedPermanently)
				return
			}
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		// Tile servers refuse requests that carry no Referer.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}

type planRequest struct {
	Points [][2]float64      `json:"points"`
	Avoid  valhalla.Avoid    `json:"avoid"`
	Legs   []plan.LegOptions `json:"legs"`
}

// plan routes the points; legs, one per pair of points, is present only in advanced mode.
func (s *Server) plan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Points) < 2 || len(req.Points) > store.MaxStops+1 {
		http.Error(w, "between 2 and "+strconv.Itoa(store.MaxStops+1)+" points", http.StatusBadRequest)
		return
	}
	pts := make([]valhalla.Point, len(req.Points))
	for i, p := range req.Points {
		if p[0] < -90 || p[0] > 90 || p[1] < -180 || p[1] > 180 {
			http.Error(w, "point out of range", http.StatusBadRequest)
			return
		}
		pts[i] = valhalla.Point{Lat: p[0], Lng: p[1]}
	}
	if req.Legs != nil && len(req.Legs) != len(pts)-1 {
		http.Error(w, "one leg option per pair of points", http.StatusBadRequest)
		return
	}
	for _, l := range req.Legs {
		if err := l.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	var allow func() bool
	user := s.rider(r)
	if s.Curves != nil {
		allow = func() bool {
			ok := s.Curves.Take(user)
			result := "spent"
			if !ok {
				result = "refused"
			}
			s.Metrics.Inc("apex_curve_searches_total", "Curve searches asked for, spent or refused for lack of budget.", "result", result)
			return ok
		}
	}
	mode := "simple"
	if req.Legs != nil {
		mode = "advanced"
	}
	started := time.Now()
	result, err := s.Planner.Plan(r.Context(), pts, req.Legs, req.Avoid, allow)
	s.Metrics.Observe("apex_plan_seconds", "Time taken to work a route out.", time.Since(started), "mode", mode)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	s.Metrics.Inc("apex_plans_total", "Routes worked out, by mode and outcome.", "mode", mode, "outcome", outcome)
	if err != nil {
		s.Log.Warn("plan", "err", err, "points", len(pts))
		http.Error(w, "no route: "+err.Error(), http.StatusBadGateway)
		return
	}
	out := struct {
		plan.Plan
		Curves *curves `json:"curves,omitempty"`
	}{Plan: result}
	if s.Curves != nil {
		left, next := s.Curves.Status(user)
		out.Curves = &curves{Left: left, Max: s.Curves.Capacity, NextIn: int(next.Seconds() + 0.999)}
	}
	writeJSON(w, out)
}

// curves tells the page how many curve searches its user has left and when the next comes back.
type curves struct {
	Left   int `json:"left"`
	Max    int `json:"max"`
	NextIn int `json:"nextIn"`
}

// search takes ?q=…&near=lat,lng and returns matching places, the ones near the route first.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 || len(q) > 120 {
		http.Error(w, "q must have 2 to 120 characters", http.StatusBadRequest)
		return
	}
	var near *[2]float64
	if lat, lng, ok := parsePoint(r.URL.Query().Get("near")); ok {
		near = &[2]float64{lat, lng}
	}
	results, err := s.Search.Search(r.Context(), q, near)
	if err != nil {
		s.Log.Warn("search", "err", err)
		http.Error(w, "search failed", http.StatusBadGateway)
		return
	}
	writeJSON(w, results)
}

func (s *Server) place(w http.ResponseWriter, r *http.Request) {
	lat, lng, ok := parsePoint(r.URL.Query().Get("lat") + "," + r.URL.Query().Get("lng"))
	if !ok {
		http.Error(w, "bad point", http.StatusBadRequest)
		return
	}
	label, err := s.Places.Name(r.Context(), lat, lng)
	if err != nil {
		s.Log.Warn("place", "err", err)
		http.Error(w, "no name", http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	writeJSON(w, label)
}

func (s *Server) listRoutes(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	routes, err := d.Routes.List()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, routes)
}

func (s *Server) getRoute(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	route, err := d.Routes.Get(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, route)
}

func (s *Server) saveRoute(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	var route store.Route
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&route); err != nil {
		http.Error(w, "bad route: "+err.Error(), http.StatusBadRequest)
		return
	}
	route.ID = r.PathValue("id")
	if route.ID != "" {
		if _, err := d.Routes.Get(route.ID); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := route.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	saved, err := d.Routes.Put(route)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, saved)
}

func (s *Server) deleteRoute(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	if err := d.Routes.Delete(r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if errors.Is(err, store.ErrLimit) {
		http.Error(w, "your account is full: delete something first", http.StatusConflict)
		return
	}
	s.Log.Error("store", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func parsePoint(s string) (lat, lng float64, ok bool) {
	a, b, found := strings.Cut(s, ",")
	if !found {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
