// Package photon searches places by name with Photon, an OpenStreetMap geocoder built for
// search-as-you-type (Nominatim's usage policy forbids autocomplete).
package photon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://photon.komoot.io"

type Result struct {
	Name   string  `json:"name"`
	Detail string  `json:"detail"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
}

type Client struct {
	base      string
	userAgent string
	http      *http.Client

	mu    sync.Mutex
	cache map[string][]Result
}

func New(base, userAgent string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"), userAgent: userAgent,
		http:  &http.Client{Timeout: 10 * time.Second},
		cache: map[string][]Result{},
	}
}

// Search returns up to 6 places matching q, ranked nearer to near when it is given.
func (c *Client) Search(ctx context.Context, q string, near *[2]float64) ([]Result, error) {
	params := url.Values{"q": {q}, "limit": {"6"}}
	if near != nil {
		params.Set("lat", strconv.FormatFloat(near[0], 'f', 3, 64))
		params.Set("lon", strconv.FormatFloat(near[1], 'f', 3, 64))
		// Photon's default lets a famous namesake far away outrank the one by the route.
		params.Set("location_bias_scale", "0.1")
	}
	key := params.Encode()
	c.mu.Lock()
	cached, ok := c.cache[key]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/?"+key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photon: %s", resp.Status)
	}
	var body struct {
		Features []struct {
			Geometry struct {
				Coordinates [2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("photon: %w", err)
	}
	out := []Result{}
	for _, f := range body.Features {
		name, detail := label(f.Properties)
		if name == "" {
			continue
		}
		out = append(out, Result{Name: name, Detail: detail, Lat: f.Geometry.Coordinates[1], Lng: f.Geometry.Coordinates[0]})
	}

	c.mu.Lock()
	if len(c.cache) > 2000 {
		c.cache = map[string][]Result{}
	}
	c.cache[key] = out
	c.mu.Unlock()
	return out, nil
}

// label names a result and places it: "Sa Calobra" / "Escorca, Illes Balears".
func label(p map[string]any) (string, string) {
	get := func(k string) string {
		s, _ := p[k].(string)
		return s
	}
	name := get("name")
	if name == "" && get("street") != "" {
		name = strings.TrimSpace(get("street") + " " + get("housenumber"))
	}
	var parts []string
	for _, k := range []string{"city", "county", "state", "country"} {
		v := get(k)
		if v != "" && v != name && (len(parts) == 0 || parts[len(parts)-1] != v) {
			parts = append(parts, v)
		}
		if len(parts) == 2 {
			break
		}
	}
	return name, strings.Join(parts, ", ")
}
