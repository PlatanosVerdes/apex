// Package valhalla draws the motorcycle route between stops with a Valhalla server.
package valhalla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://valhalla1.openstreetmap.de"

// maxLocations is the public server's limit per request; longer routes go in chunks sharing their ends.
const maxLocations = 10

type Point struct{ Lat, Lng float64 }

type Avoid struct {
	Highways bool `json:"highways"`
	Tolls    bool `json:"tolls"`
	Ferries  bool `json:"ferries"`
}

// Options shape one request. Exclude holds closed rings of points the route must not cross.
type Options struct {
	Avoid   Avoid
	Exclude [][]Point
}

type Leg struct {
	Meters  float64      `json:"meters"`
	Seconds float64      `json:"seconds"`
	Path    [][2]float64 `json:"path"`
}

type Route struct {
	Meters  float64 `json:"meters"`
	Seconds float64 `json:"seconds"`
	Legs    []Leg   `json:"legs"`
	// Has says what the route still uses, which an avoid option only discourages.
	Has Avoid `json:"has"`
}

type Client struct {
	base      string
	userAgent string
	http      *http.Client
	// slots bound the calls in flight, since the public server answers 429 to a burst. Searches
	// have a lane of their own, so a fast leg never queues behind a curve search's many calls.
	slots, searchSlots chan struct{}
	// retryWait is the first pause after a 429; it doubles on each retry.
	retryWait time.Duration

	mu    sync.Mutex
	cache map[string]Route
}

func New(base, userAgent string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"), userAgent: userAgent,
		http:        &http.Client{Timeout: 30 * time.Second},
		cache:       map[string]Route{},
		slots:       make(chan struct{}, 2),
		searchSlots: make(chan struct{}, 2),
		retryWait:   500 * time.Millisecond,
	}
}

func (c *Client) Route(ctx context.Context, pts []Point, o Options) (Route, error) {
	if len(pts) < 2 {
		return Route{}, errors.New("a route needs two points")
	}
	var out Route
	for start := 0; start < len(pts)-1; start += maxLocations - 1 {
		part, err := c.chunk(ctx, pts[start:min(start+maxLocations, len(pts))], o)
		if err != nil {
			return Route{}, err
		}
		out.Meters += part.Meters
		out.Seconds += part.Seconds
		out.Legs = append(out.Legs, part.Legs...)
		out.Has.Highways = out.Has.Highways || part.Has.Highways
		out.Has.Tolls = out.Has.Tolls || part.Has.Tolls
		out.Has.Ferries = out.Has.Ferries || part.Has.Ferries
	}
	return out, nil
}

// chunk is cached, so moving one stop of a long route asks the server again only for its own chunk.
func (c *Client) chunk(ctx context.Context, pts []Point, o Options) (Route, error) {
	key := fmt.Sprint(pts, o)
	c.mu.Lock()
	cached, ok := c.cache[key]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}
	r, err := c.request(ctx, pts, o)
	if err != nil {
		return Route{}, err
	}
	c.mu.Lock()
	if len(c.cache) > 500 {
		c.cache = map[string]Route{}
	}
	c.cache[key] = r
	c.mu.Unlock()
	return r, nil
}

func (c *Client) request(ctx context.Context, pts []Point, o Options) (Route, error) {
	avoid := o.Avoid
	type filter struct {
		MinRoadClass string `json:"min_road_class"`
	}
	type location struct {
		Lat    float64 `json:"lat"`
		Lon    float64 `json:"lon"`
		Filter filter  `json:"search_filter"`
	}
	locations := make([]location, len(pts))
	for i, p := range pts {
		// A point lands on the nearest real road, never a track or service lane beside it: Google
		// would snap a point left on a track to the track and ride in and out of it.
		locations[i] = location{p.Lat, p.Lng, filter{MinRoadClass: "residential"}}
	}
	options := map[string]float64{}
	if avoid.Highways {
		options["use_highways"] = 0
	}
	if avoid.Tolls {
		options["use_tolls"] = 0
	}
	if avoid.Ferries {
		options["use_ferry"] = 0
	}
	request := map[string]any{
		"locations":       locations,
		"costing":         "motorcycle",
		"costing_options": map[string]any{"motorcycle": options},
		"directions_type": "none",
		"units":           "kilometers",
	}
	if len(o.Exclude) > 0 {
		rings := make([][][2]float64, len(o.Exclude))
		for i, ring := range o.Exclude {
			for _, p := range ring {
				rings[i] = append(rings[i], [2]float64{p.Lng, p.Lat})
			}
		}
		request["exclude_polygons"] = rings
	}
	body, _ := json.Marshal(request)
	resp, err := c.post(ctx, body)
	if err != nil {
		return Route{}, err
	}
	defer resp.Body.Close()

	var res struct {
		Error string `json:"error"`
		Trip  struct {
			Summary struct {
				Length     float64 `json:"length"`
				Time       float64 `json:"time"`
				HasHighway bool    `json:"has_highway"`
				HasToll    bool    `json:"has_toll"`
				HasFerry   bool    `json:"has_ferry"`
			} `json:"summary"`
			Legs []struct {
				Summary struct {
					Length float64 `json:"length"`
					Time   float64 `json:"time"`
				} `json:"summary"`
				Shape string `json:"shape"`
			} `json:"legs"`
		} `json:"trip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return Route{}, fmt.Errorf("valhalla: %s: %w", resp.Status, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Route{}, fmt.Errorf("valhalla: %s", res.Error)
	}

	s := res.Trip.Summary
	out := Route{
		Meters: s.Length * 1000, Seconds: s.Time,
		Has:  Avoid{Highways: s.HasHighway, Tolls: s.HasToll, Ferries: s.HasFerry},
		Legs: make([]Leg, len(res.Trip.Legs)),
	}
	for i, l := range res.Trip.Legs {
		out.Legs[i] = Leg{Meters: l.Summary.Length * 1000, Seconds: l.Summary.Time, Path: decode(l.Shape)}
	}
	return out, nil
}

// post sends a request through the slots and, when the server says it is too busy (429), waits and
// tries again, three times at most.
type searchKey struct{}

// Searching marks ctx as part of a search that asks many times, so its calls take the search lane.
func Searching(ctx context.Context) context.Context {
	return context.WithValue(ctx, searchKey{}, true)
}

func (c *Client) post(ctx context.Context, body []byte) (*http.Response, error) {
	slots := c.slots
	if ctx.Value(searchKey{}) != nil {
		slots = c.searchSlots
	}
	wait := c.retryWait
	for attempt := 0; ; attempt++ {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/route", bytes.NewReader(body))
		if err != nil {
			<-slots
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		resp, err := c.http.Do(req)
		<-slots
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || attempt == 3 {
			return resp, err
		}
		resp.Body.Close()
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		wait *= 2
	}
}

// decode reads Valhalla's shape: a Google encoded polyline with 6 decimals instead of 5.
func decode(shape string) [][2]float64 {
	path := [][2]float64{}
	var lat, lng int
	for i := 0; i < len(shape); {
		for _, v := range []*int{&lat, &lng} {
			var result, shift int
			for i < len(shape) {
				b := int(shape[i]) - 63
				i++
				result |= (b & 0x1f) << shift
				shift += 5
				if b < 0x20 {
					break
				}
			}
			if result&1 != 0 {
				*v += ^(result >> 1)
			} else {
				*v += result >> 1
			}
		}
		path = append(path, [2]float64{round(float64(lat) / 1e6), round(float64(lng) / 1e6)})
	}
	return path
}

// round keeps 5 decimals, about a metre, which is plenty for a line on a map.
func round(v float64) float64 { return math.Round(v*1e5) / 1e5 }
