// Package plan works out each leg of a route the way the rider asked for it (fast, curvy, twisty)
// and the hidden via points that make Google Maps drive the same roads.
package plan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

type Style string

const (
	Fast   Style = "fast"
	Curvy  Style = "curvy"
	Twisty Style = "twisty"
)

// slack is how much longer than the fast way a curvy leg may take.
var slack = map[Style]float64{Curvy: 1.5, Twisty: 2.5}

type LegOptions struct {
	Style         Style `json:"style"`
	AvoidHighways bool  `json:"avoidHighways"`
}

func (o LegOptions) Validate() error {
	switch o.Style {
	case Fast, Curvy, Twisty:
		return nil
	}
	return fmt.Errorf("unknown style %q", o.Style)
}

type Router interface {
	Route(ctx context.Context, pts []valhalla.Point, o valhalla.Options) (valhalla.Route, error)
}

type Leg struct {
	Meters    float64  `json:"meters"`
	Seconds   float64  `json:"seconds"`
	Path      []latLng `json:"path"`
	Via       []latLng `json:"via"`
	Curvature float64  `json:"curvature"`
	// Limited marks a curvy leg drawn fast for now, because its rider ran out of curve searches.
	Limited bool `json:"limited,omitempty"`
}

type Plan struct {
	Meters  float64        `json:"meters"`
	Seconds float64        `json:"seconds"`
	Legs    []Leg          `json:"legs"`
	Has     valhalla.Avoid `json:"has"`
	// Link is what the Google Maps link must ask to avoid for the whole route.
	Link valhalla.Avoid `json:"link"`
}

type Planner struct {
	Router Router
	// Parallel bounds the requests in flight; the public Valhalla server is shared.
	Parallel int

	mu    sync.Mutex
	cache map[string]valhalla.Route
}

// simplifyMeters is how far a kept line may stray from Valhalla's: invisible on the map, and it
// leaves a fifth of the points or fewer to send, keep and draw.
const simplifyMeters = 5

const (
	// A via point goes wherever the leg strays farther than this from the way Google would go.
	strayMeters = 200
	strayMinLen = 300
	// Long detours get a via point every this many metres, so Google cannot cut back halfway.
	viaEvery     = 15000
	maxViaPerLeg = 8
	shapeRounds  = 3
	searchRounds = 3
)

// Plan routes pts. With legs nil every leg is the fast way under avoid, as in simple mode.
// allow, when given, is asked before each curve search not already cached; a leg it refuses is
// drawn fast without motorways and marked Limited.
func (p *Planner) Plan(ctx context.Context, pts []valhalla.Point, legs []LegOptions, avoid valhalla.Avoid, allow func() bool) (Plan, error) {
	if len(pts) < 2 {
		return Plan{}, errors.New("a route needs two points")
	}
	if legs == nil {
		return p.fastPlan(ctx, pts, avoid)
	}
	if len(legs) != len(pts)-1 {
		return Plan{}, fmt.Errorf("%d legs for %d points", len(legs), len(pts))
	}

	// Google takes one avoid setting for the whole link: motorways only when every leg avoids them.
	link := avoid
	link.Highways = true
	for _, l := range legs {
		link.Highways = link.Highways && l.AvoidHighways
	}

	out := Plan{Legs: make([]Leg, len(legs)), Link: link}
	has := make([]valhalla.Avoid, len(legs))
	errs := make([]error, len(legs))
	sem := make(chan struct{}, max(p.Parallel, 1))
	var wg sync.WaitGroup
	for i, o := range legs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out.Legs[i], has[i], errs[i] = p.cachedLeg(ctx, pts[i], pts[i+1], o, avoid, link, allow)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return Plan{}, err
	}
	for i, l := range out.Legs {
		out.Meters += l.Meters
		out.Seconds += l.Seconds
		out.Has.Highways = out.Has.Highways || has[i].Highways
		out.Has.Tolls = out.Has.Tolls || has[i].Tolls
		out.Has.Ferries = out.Has.Ferries || has[i].Ferries
	}
	return out, nil
}

// cachedLeg works a leg out in two parts kept apart in memory: which roads it takes, which depends
// only on its own ends and options, and the hidden stops for Google, which also depend on the whole
// route's avoid settings. Adding a stop can change those settings, but never sends an unchanged leg
// back to search for curves.
func (p *Planner) cachedLeg(ctx context.Context, a, b valhalla.Point, o LegOptions, avoid, link valhalla.Avoid, allow func() bool) (Leg, valhalla.Avoid, error) {
	own := avoid
	own.Highways = o.AvoidHighways || o.Style != Fast
	routeKey := fmt.Sprint("route", a, b, o.Style, own)
	r, ok := p.recall(routeKey)
	limited := false
	if !ok {
		var err error
		switch {
		case o.Style == Fast:
			r, err = p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: own})
		case allow != nil && !allow():
			// Not kept: the curvy leg is still owed once the budget refills.
			limited = true
			r, err = p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: own})
		default:
			r, err = p.curviest(valhalla.Searching(ctx), a, b, own, slack[o.Style])
		}
		if err != nil {
			return Leg{}, valhalla.Avoid{}, err
		}
		r.Legs[0].Path = Simplify(r.Legs[0].Path, simplifyMeters)
		if !limited {
			p.remember(routeKey, r)
		}
	}
	path := r.Legs[0].Path
	leg := Leg{Meters: r.Meters, Seconds: r.Seconds, Path: path, Via: []latLng{}, Curvature: Curvature(path), Limited: limited}
	// Google, asked with the link's settings, already drives a fast leg with the same settings.
	if o.Style == Fast && own == link {
		return leg, r.Has, nil
	}
	viaKey := fmt.Sprint("via", a, b, o.Style, own, link, limited)
	if v, ok := p.recall(viaKey); ok {
		leg.Via = v.Legs[0].Path
		return leg, r.Has, nil
	}
	via, err := p.shape(valhalla.Searching(ctx), a, b, path, link)
	if err != nil {
		return Leg{}, valhalla.Avoid{}, err
	}
	leg.Via = via
	p.remember(viaKey, valhalla.Route{Legs: []valhalla.Leg{{Path: via}}})
	return leg, r.Has, nil
}

// fastPlan routes every leg the fast way. Legs are kept one by one, so a stop added to a drawn
// route asks only for its two new legs; each run of legs not kept yet is still a single call.
func (p *Planner) fastPlan(ctx context.Context, pts []valhalla.Point, avoid valhalla.Avoid) (Plan, error) {
	n := len(pts) - 1
	key := func(i int) string { return fmt.Sprint("leg", pts[i], pts[i+1], avoid) }
	routes := make([]valhalla.Route, n)
	var runs [][2]int
	for i := range n {
		r, ok := p.recall(key(i))
		switch {
		case ok:
			routes[i] = r
		case len(runs) > 0 && runs[len(runs)-1][1] == i:
			runs[len(runs)-1][1] = i + 1
		default:
			runs = append(runs, [2]int{i, i + 1})
		}
	}
	errs := make([]error, len(runs))
	var wg sync.WaitGroup
	for k, run := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := p.Router.Route(ctx, pts[run[0]:run[1]+1], valhalla.Options{Avoid: avoid})
			if err != nil {
				errs[k] = err
				return
			}
			// Valhalla says what a call still uses, not each leg of it, so each leg carries its run's.
			for j, l := range r.Legs {
				l.Path = Simplify(l.Path, simplifyMeters)
				leg := valhalla.Route{Meters: l.Meters, Seconds: l.Seconds, Has: r.Has, Legs: []valhalla.Leg{l}}
				routes[run[0]+j] = leg
				p.remember(key(run[0]+j), leg)
			}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return Plan{}, err
	}
	out := Plan{Link: avoid}
	for _, r := range routes {
		path := r.Legs[0].Path
		out.Legs = append(out.Legs, Leg{Meters: r.Meters, Seconds: r.Seconds, Path: path, Via: []latLng{}, Curvature: Curvature(path)})
		out.Meters += r.Meters
		out.Seconds += r.Seconds
		out.Has.Highways = out.Has.Highways || r.Has.Highways
		out.Has.Tolls = out.Has.Tolls || r.Has.Tolls
		out.Has.Ferries = out.Has.Ferries || r.Has.Ferries
	}
	return out, nil
}

func (p *Planner) recall(key string) (valhalla.Route, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r, ok := p.cache[key]
	return r, ok
}

func (p *Planner) remember(key string, r valhalla.Route) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache == nil || len(p.cache) > 1500 {
		p.cache = map[string]valhalla.Route{}
	}
	p.cache[key] = r
}

// curviest looks for bendier ways than the fast one, asking for every candidate in one go: each closes off more of the fast way's
// straightest stretches (two, four, six), so they run side by side instead of one after another.
func (p *Planner) curviest(ctx context.Context, a, b valhalla.Point, avoid valhalla.Avoid, slack float64) (valhalla.Route, error) {
	fast, err := p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: avoid})
	if err != nil {
		return valhalla.Route{}, err
	}
	mids := straightest(fast.Legs[0].Path, 2*searchRounds, 1500, 1000)
	routes := make([]valhalla.Route, searchRounds)
	ok := make([]bool, searchRounds)
	var wg sync.WaitGroup
	for i := range searchRounds {
		n := min(2*(i+1), len(mids))
		if n == 0 || (i > 0 && n == min(2*i, len(mids))) {
			continue
		}
		closed := make([][]valhalla.Point, n)
		for j, m := range mids[:n] {
			closed[j] = square(m, 120)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: avoid, Exclude: closed})
			routes[i], ok[i] = r, err == nil
		}()
	}
	wg.Wait()
	best, bestCurves := fast, Curvature(fast.Legs[0].Path)
	for i, r := range routes {
		if !ok[i] {
			continue
		}
		if c := Curvature(r.Legs[0].Path); c > bestCurves && r.Seconds <= fast.Seconds*slack {
			best, bestCurves = r, c
		}
	}
	return best, nil
}

// shape returns the via points that keep Google, routing a→b with the link's settings, on path.
// The fast route under those settings stands in for Google's choice.
func (p *Planner) shape(ctx context.Context, a, b valhalla.Point, path []latLng, link valhalla.Avoid) ([]latLng, error) {
	type via struct {
		at int
		p  latLng
	}
	var vias []via
	for range shapeRounds {
		pts := []valhalla.Point{a}
		for _, v := range vias {
			pts = append(pts, valhalla.Point{Lat: v.p[0], Lng: v.p[1]})
		}
		pts = append(pts, b)
		g, err := p.Router.Route(ctx, pts, valhalla.Options{Avoid: link})
		if err != nil {
			return nil, err
		}
		var got []latLng
		for _, l := range g.Legs {
			got = append(got, l.Path...)
		}
		samples, strays := offRoute(path, got, strayMeters, strayMinLen, 50)
		if len(strays) == 0 {
			break
		}
		for _, s := range strays {
			n := max(1, int(float64(s[1]-s[0])*50/viaEvery))
			for k := 1; k <= n; k++ {
				at := s[0] + (s[1]-s[0])*k/(n+1)
				vias = append(vias, via{at, samples[at]})
			}
		}
		slices.SortFunc(vias, func(x, y via) int { return x.at - y.at })
		if len(vias) >= maxViaPerLeg {
			vias = vias[:maxViaPerLeg]
			break
		}
	}
	out := make([]latLng, len(vias))
	for i, v := range vias {
		out[i] = latLng{math.Round(v.p[0]*1e5) / 1e5, math.Round(v.p[1]*1e5) / 1e5}
	}
	return out, nil
}
