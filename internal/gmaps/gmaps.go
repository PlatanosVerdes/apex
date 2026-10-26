// Package gmaps reads a shared Google Maps list from its link. There is no public API for lists:
// this is the endpoint Google Maps itself calls to show one, so it may change without notice.
package gmaps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Place struct {
	Name    string  `json:"name"`
	Address string  `json:"address,omitempty"`
	Note    string  `json:"note,omitempty"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

type List struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Icon is the emoji picked for the list in Google Maps, empty for the default icon.
	Icon   string  `json:"icon,omitempty"`
	Places []Place `json:"places"`
}

var ErrNotAList = errors.New("not a shared Google Maps list link")

// A list's id follows !2s in the data blob of the page the short link opens, or sits in its
// /placelists/list/ path.
var listID = regexp.MustCompile(`(?:!11m\d+!2s|/placelists/list/)([A-Za-z0-9_-]{20,})`)

type Client struct {
	base string
	http *http.Client
}

func New() *Client {
	return &Client{
		base: "https://www.google.com",
		http: &http.Client{
			Timeout: 20 * time.Second,
			// The short link's redirect target carries the id; following it would only load a page.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Resolve returns the list id behind a share link (maps.app.goo.gl/…) or a full list URL.
func (c *Client) Resolve(ctx context.Context, link string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "https" {
		return "", ErrNotAList
	}
	if m := listID.FindStringSubmatch(u.String()); m != nil && isGoogle(u.Host) {
		return m[1], nil
	}
	if u.Host != "maps.app.goo.gl" {
		return "", ErrNotAList
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if m := listID.FindStringSubmatch(resp.Header.Get("Location")); m != nil {
		return m[1], nil
	}
	return "", ErrNotAList
}

func isGoogle(host string) bool {
	return host == "www.google.com" || host == "google.com" || host == "maps.google.com"
}

func (c *Client) Fetch(ctx context.Context, id string) (List, error) {
	pb := "!1m4!1s" + url.QueryEscape(id) + "!2e1!3m1!1e1!2e2!3e2!4i500!16b1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/maps/preview/entitylist/getlist?authuser=0&hl=es&gl=es&pb="+pb, nil)
	if err != nil {
		return List{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return List{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return List{}, fmt.Errorf("google maps list: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return List{}, err
	}
	l, err := parse(body)
	if err != nil {
		return List{}, err
	}
	l.ID = id
	return l, nil
}

// parse reads the positional arrays the endpoint returns after its )]}' guard line. Only the
// fields used are looked up, each by its position: list name at [0][4], places at [0][8], emoji at [0][17], and per
// place the name at [2], the note at [3], the address at [1][4] and lat/lng at [1][5][2..3].
func parse(body []byte) (List, error) {
	_, payload, found := strings.Cut(string(body), "\n")
	if !found {
		return List{}, errors.New("google maps list: unexpected response")
	}
	var root any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return List{}, fmt.Errorf("google maps list: %w", err)
	}
	list := at(root, 0)
	if list == nil {
		return List{}, errors.New("google maps list: not found or not shared")
	}
	out := List{Name: str(at(list, 4)), Icon: str(at(list, 17)), Places: []Place{}}
	items, _ := at(list, 8).([]any)
	for _, item := range items {
		lat, okLat := at(item, 1, 5, 2).(float64)
		lng, okLng := at(item, 1, 5, 3).(float64)
		if !okLat || !okLng {
			continue
		}
		p := Place{Name: str(at(item, 2)), Note: str(at(item, 3)), Address: str(at(item, 1, 4)), Lat: lat, Lng: lng}
		// Dropped pins come back named "Marcador" (the UI language) or unnamed.
		if p.Name == "" || p.Name == "Marcador" {
			p.Name = strings.TrimPrefix(strings.TrimPrefix(p.Address, "Dirección cercana: "), "Cerca de ")
		}
		if p.Name == "" {
			p.Name = fmt.Sprintf("%.5f, %.5f", lat, lng)
		}
		out.Places = append(out.Places, p)
	}
	return out, nil
}

func at(v any, path ...int) any {
	for _, i := range path {
		a, ok := v.([]any)
		if !ok || i >= len(a) {
			return nil
		}
		v = a[i]
	}
	return v
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
