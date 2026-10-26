// Package nominatim names a point on the map (Sóller, Ma-10) from OpenStreetMap data.
package nominatim

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
	"unicode"
	"unicode/utf8"
)

const DefaultBaseURL = "https://nominatim.openstreetmap.org"

// Label names a point by its town or village and, when known, the road it is on.
type Label struct {
	Place string `json:"place"`
	Road  string `json:"road"`
}

type Client struct {
	base      string
	userAgent string
	http      *http.Client
	gap       time.Duration

	mu    sync.Mutex
	next  time.Time
	cache map[string]Label
}

func New(base, userAgent string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"), userAgent: userAgent,
		http: &http.Client{Timeout: 10 * time.Second},
		// The public server's usage policy allows one request a second.
		gap:   time.Second,
		cache: map[string]Label{},
	}
}

func (c *Client) Name(ctx context.Context, lat, lng float64) (Label, error) {
	key := fmt.Sprintf("%.4f,%.4f", lat, lng)
	c.mu.Lock()
	if l, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return l, nil
	}
	now := time.Now()
	slot := c.next
	if slot.Before(now) {
		slot = now
	}
	c.next = slot.Add(c.gap)
	c.mu.Unlock()

	select {
	case <-time.After(slot.Sub(now)):
	case <-ctx.Done():
		return Label{}, ctx.Err()
	}

	q := url.Values{
		"format": {"jsonv2"}, "lat": {strconv.FormatFloat(lat, 'f', 6, 64)}, "lon": {strconv.FormatFloat(lng, 'f', 6, 64)},
		"zoom": {"17"}, "accept-language": {"es,en"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/reverse?"+q.Encode(), nil)
	if err != nil {
		return Label{}, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return Label{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Label{}, fmt.Errorf("nominatim: %s", resp.Status)
	}
	var body struct {
		Name    string            `json:"name"`
		Address map[string]string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Label{}, fmt.Errorf("nominatim: %w", err)
	}
	l := label(body.Name, body.Address)

	c.mu.Lock()
	if len(c.cache) > 10000 {
		c.cache = map[string]Label{}
	}
	c.cache[key] = l
	c.mu.Unlock()
	return l, nil
}

func label(name string, addr map[string]string) Label {
	road := addr["road"]
	if road == "" {
		road = name
	}
	var place string
	for _, k := range []string{"village", "town", "city", "hamlet", "municipality", "county"} {
		if addr[k] != "" {
			place = addr[k]
			break
		}
	}
	if road == place {
		road = ""
	}
	return Label{Place: place, Road: capitalize(road)}
}

// capitalize fixes OSM names that start lower case, as Catalan street types do ("avinguda de…").
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
