package gmaps

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Waypoint is one stop of a Google Maps route. HasPoint is false when the link only names it; Here
// is the rider's own location ("My location"), which the link never pins.
type Waypoint struct {
	Name     string
	Lat, Lng float64
	HasPoint bool
	Here     bool
}

type Directions struct {
	Waypoints []Waypoint
	Avoid     struct{ Highways, Tolls, Ferries bool }
}

var ErrNotARoute = errors.New("not a Google Maps route link")

var latLng = regexp.MustCompile(`^\s*(-?\d{1,2}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)\s*$`)

// The rider's location, as Google writes it in the languages riders here use.
var hereNames = map[string]bool{"my location": true, "your location": true, "mi ubicación": true, "tu ubicación": true, "la meva ubicació": true}

// ResolveRoute returns the full route URL behind a share link (maps.app.goo.gl/…), or the link
// itself when it already is one.
func (c *Client) ResolveRoute(ctx context.Context, link string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "https" {
		return "", ErrNotARoute
	}
	for hop := 0; u.Host == "maps.app.goo.gl" || u.Host == "goo.gl"; hop++ {
		if hop == 3 {
			return "", ErrNotARoute
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
		if u, err = url.Parse(resp.Header.Get("Location")); err != nil || u.Scheme != "https" {
			return "", ErrNotARoute
		}
	}
	if !isGoogle(u.Host) {
		return "", ErrNotARoute
	}
	return u.String(), nil
}

// ParseDirections reads the stops and avoid options from a Google Maps route URL: the path form
// (/maps/dir/A/B/@…/data=…) the app and the web share, or the ?api=1 form.
func ParseDirections(link string) (Directions, error) {
	u, err := url.Parse(link)
	if err != nil || !isGoogle(u.Host) {
		return Directions{}, ErrNotARoute
	}
	var d Directions
	if q := u.Query(); q.Get("api") == "1" {
		names := append([]string{q.Get("origin")}, strings.Split(q.Get("waypoints"), "|")...)
		for _, n := range append(names, q.Get("destination")) {
			if strings.TrimSpace(n) != "" {
				d.Waypoints = append(d.Waypoints, waypoint(n))
			}
		}
		for _, a := range strings.Split(q.Get("avoid"), "|") {
			d.setAvoid(a)
		}
	} else {
		path, ok := strings.CutPrefix(u.EscapedPath(), "/maps/dir/")
		if !ok {
			return Directions{}, ErrNotARoute
		}
		var data string
		for _, seg := range strings.Split(path, "/") {
			if blob, ok := strings.CutPrefix(seg, "data="); ok {
				data = blob
				break
			}
			if strings.HasPrefix(seg, "@") {
				continue
			}
			name, err := url.PathUnescape(strings.ReplaceAll(seg, "+", " "))
			if err != nil || strings.TrimSpace(name) == "" {
				continue
			}
			d.Waypoints = append(d.Waypoints, waypoint(name))
		}
		d.readData(data)
	}
	if len(d.Waypoints) < 2 {
		return Directions{}, ErrNotARoute
	}
	return d, nil
}

func waypoint(text string) Waypoint {
	if m := latLng.FindStringSubmatch(text); m != nil {
		lat, _ := strconv.ParseFloat(m[1], 64)
		lng, _ := strconv.ParseFloat(m[2], 64)
		if lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 {
			return Waypoint{Lat: lat, Lng: lng, HasPoint: true}
		}
	}
	name := strings.TrimSpace(text)
	return Waypoint{Name: name, Here: hereNames[strings.ToLower(name)]}
}

func (d *Directions) setAvoid(option string) {
	switch strings.TrimSpace(option) {
	case "highways":
		d.Avoid.Highways = true
	case "tolls":
		d.Avoid.Tolls = true
	case "ferries":
		d.Avoid.Ferries = true
	}
}

// node is one entry of the data blob, Google's text form of a protocol buffer: !<field><type><value>,
// where an m (message) counts how many of the entries after it belong inside it.
type node struct {
	field    int
	kind     byte
	value    string
	children []node
}

func parseBlob(tokens []string) []node {
	var out []node
	for len(tokens) > 0 {
		n, rest := parseNode(tokens)
		if n.kind == 0 {
			tokens = tokens[1:]
			continue
		}
		out = append(out, n)
		tokens = rest
	}
	return out
}

func parseNode(tokens []string) (node, []string) {
	t := tokens[0]
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i == 0 || i == len(t) {
		return node{}, tokens[1:]
	}
	field, _ := strconv.Atoi(t[:i])
	n := node{field: field, kind: t[i], value: t[i+1:]}
	rest := tokens[1:]
	if n.kind == 'm' {
		count, err := strconv.Atoi(n.value)
		if err != nil || count > len(rest) {
			return node{}, tokens[1:]
		}
		n.children = parseBlob(rest[:count])
		rest = rest[count:]
	}
	return n, rest
}

func (n node) child(field int, kind byte) (node, bool) {
	for _, c := range n.children {
		if c.field == field && c.kind == kind {
			return c, true
		}
	}
	return node{}, false
}

// readData takes what the data blob adds to the path: under !4m…!4m… one !1m entry per stop, whose
// !2m holds its exact point (!1d lng, !2d lat) when the stop was named rather than pinned, and the
// avoid options in !2m (!1b motorways, !2b tolls, !3b ferries).
func (d *Directions) readData(data string) {
	for _, top := range parseBlob(strings.Split(strings.TrimPrefix(data, "!"), "!")) {
		if top.field != 4 || top.kind != 'm' {
			continue
		}
		dir, ok := top.child(4, 'm')
		if !ok {
			continue
		}
		var stops []node
		for _, c := range dir.children {
			if c.field == 1 && c.kind == 'm' {
				stops = append(stops, c)
			}
		}
		// Only when every stop has its entry can they be matched to the path in order.
		if len(stops) == len(d.Waypoints) {
			for i, s := range stops {
				pt, ok := s.child(2, 'm')
				if !ok || d.Waypoints[i].HasPoint {
					continue
				}
				lng, okLng := pt.child(1, 'd')
				lat, okLat := pt.child(2, 'd')
				if !okLng || !okLat {
					continue
				}
				if w := waypoint(lat.value + "," + lng.value); w.HasPoint {
					d.Waypoints[i].Lat, d.Waypoints[i].Lng, d.Waypoints[i].HasPoint = w.Lat, w.Lng, true
				}
			}
		}
		if opts, ok := dir.child(2, 'm'); ok {
			for f, name := range map[int]string{1: "highways", 2: "tolls", 3: "ferries"} {
				if b, ok := opts.child(f, 'b'); ok && b.value == "1" {
					d.setAvoid(name)
				}
			}
		}
	}
}
