package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PlatanosVerdes/apex/internal/metrics"
	"github.com/PlatanosVerdes/apex/internal/store"
)

// counted records each request by the route it matched and the class of its answer.
func (s *Server) counted(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &status{ResponseWriter: w, code: http.StatusOK}
		mux.ServeHTTP(rec, r)
		_, pattern := mux.Handler(r)
		route := strings.TrimPrefix(pattern, r.Method+" ")
		if !strings.HasPrefix(route, "/api/") && !strings.HasPrefix(route, "/auth/") {
			route = "page" // static files and route links, one series for all of them
		}
		s.Metrics.Inc("apex_http_requests_total", "Requests by route and answer class.",
			"route", route, "method", r.Method, "code", strconv.Itoa(rec.code/100)+"xx")
		if strings.HasPrefix(route, "/api/") {
			s.Metrics.Seen(s.rider(r))
		}
	})
}

type status struct {
	http.ResponseWriter
	code int
}

func (s *status) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

// Gauges registers what is read from the data folder at each scrape: a handful of small files a rider.
func (s *Server) Gauges() {
	totals := func(read func(*store.UserData) int) func() float64 {
		return func() float64 {
			n := 0
			_ = s.Users.Each(func(d *store.UserData) { n += read(d) })
			return float64(n)
		}
	}
	s.Metrics.Gauge("apex_accounts", "Riders with an account.", totals(func(*store.UserData) int { return 1 }))
	s.Metrics.Gauge("apex_saved_routes", "Routes kept across every account.", totals(func(d *store.UserData) int {
		return d.Routes.Count()
	}))
	s.Metrics.Gauge("apex_linked_lists", "Google Maps lists linked across every account.", totals(func(d *store.UserData) int {
		lists, _ := d.Lists.All()
		return len(lists)
	}))
}

// Transport times every call Apex makes to another server and counts how it ended, by service.
type Transport struct {
	Next    http.RoundTripper
	Metrics *metrics.Registry
}

var services = map[string]string{
	"valhalla1.openstreetmap.de":  "valhalla",
	"nominatim.openstreetmap.org": "nominatim",
	"photon.komoot.io":            "photon",
	"www.google.com":              "google_maps_lists",
	"maps.app.goo.gl":             "google_maps_lists",
	"oauth2.googleapis.com":       "google_signin",
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	service := services[r.URL.Host]
	if service == "" {
		service = "other"
	}
	start := time.Now()
	resp, err := t.Next.RoundTrip(r)
	outcome := "error"
	switch {
	case err != nil && r.Context().Err() != nil:
		outcome = "cancelled"
	case err == nil && resp.StatusCode == http.StatusTooManyRequests:
		outcome = "throttled"
	case err == nil && resp.StatusCode < 500:
		outcome = "ok"
	}
	t.Metrics.Observe("apex_upstream_seconds", "Time taken by calls to other servers.", time.Since(start), "service", service)
	t.Metrics.Inc("apex_upstream_requests_total", "Calls to other servers by how they ended.", "service", service, "outcome", outcome)
	return resp, err
}

// donateEvent counts {"event": "shown" | "clicked" | "dismissed"} from the page's invitation to donate.
func (s *Server) donateEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Event string `json:"event"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch body.Event {
	case "shown", "clicked", "dismissed":
		s.Metrics.Inc("apex_donate_events_total", "What riders did with the invitation to donate.", "event", body.Event)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unknown event", http.StatusBadRequest)
	}
}
