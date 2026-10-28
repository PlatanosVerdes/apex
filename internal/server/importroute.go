package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PlatanosVerdes/apex/internal/gmaps"
)

type RouteLinks interface {
	ResolveRoute(ctx context.Context, link string) (string, error)
}

type importedStop struct {
	Name string  `json:"name,omitempty"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	// Approx marks a stop the link only named, found by searching that name near the others.
	Approx bool `json:"approx,omitempty"`
}

// importRoute reads the route behind a Google Maps link, {"url": "…"}. Stops come from the link's own
// coordinates; only one it merely names is searched for, near the rest, and flagged.
func (s *Server) importRoute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	link, err := s.RouteLinks.ResolveRoute(r.Context(), body.URL)
	if errors.Is(err, gmaps.ErrNotARoute) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "could not open the link: "+err.Error(), http.StatusBadGateway)
		return
	}
	d, err := gmaps.ParseDirections(link)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(d.Waypoints) > 100 {
		http.Error(w, "too many stops", http.StatusBadRequest)
		return
	}
	out := struct {
		Stops    []importedStop  `json:"stops"`
		FromHere bool            `json:"fromHere"`
		Avoid    map[string]bool `json:"avoid"`
	}{Stops: []importedStop{}, Avoid: map[string]bool{"highways": d.Avoid.Highways, "tolls": d.Avoid.Tolls, "ferries": d.Avoid.Ferries}}
	var near *[2]float64
	for _, wp := range d.Waypoints {
		if wp.HasPoint {
			near = &[2]float64{wp.Lat, wp.Lng}
			break
		}
	}
	for i, wp := range d.Waypoints {
		switch {
		case wp.Here:
			// Only a start from the rider's location means something to Apex; elsewhere it is dropped.
			out.FromHere = out.FromHere || i == 0
		case wp.HasPoint:
			out.Stops = append(out.Stops, importedStop{Name: wp.Name, Lat: wp.Lat, Lng: wp.Lng})
			near = &[2]float64{wp.Lat, wp.Lng}
		default:
			found, err := s.Search.Search(r.Context(), wp.Name, near)
			if err != nil || len(found) == 0 {
				http.Error(w, "could not find "+wp.Name, http.StatusUnprocessableEntity)
				return
			}
			out.Stops = append(out.Stops, importedStop{Name: wp.Name, Lat: found[0].Lat, Lng: found[0].Lng, Approx: true})
			near = &[2]float64{found[0].Lat, found[0].Lng}
		}
	}
	s.Metrics.Inc("apex_imports_total", "Routes imported from Google Maps links.")
	writeJSON(w, out)
}
