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
	cache map[string]planned
}

type planned struct {
	leg Leg
	has valhalla.Avoid
}

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
		r, err := p.Router.Route(ctx, pts, valhalla.Options{Avoid: avoid})
		if err != nil {
			return Plan{}, err
		}
		out := Plan{Meters: r.Meters, Seconds: r.Seconds, Has: r.Has, Link: avoid}
		for _, l := range r.Legs {
			out.Legs = append(out.Legs, Leg{Meters: l.Meters, Seconds: l.Seconds, Path: l.Path, Via: []latLng{}, Curvature: Curvature(l.Path)})
		}
		return out, nil
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

// cachedLeg answers a leg worked out before from memory, so editing elsewhere on the route spends
// no curve searches on legs that did not change.
func (p *Planner) cachedLeg(ctx context.Context, a, b valhalla.Point, o LegOptions, avoid, link valhalla.Avoid, allow func() bool) (Leg, valhalla.Avoid, error) {
	key := fmt.Sprint(a, b, o, avoid, link)
	p.mu.Lock()
	hit, ok := p.cache[key]
	p.mu.Unlock()
	if ok {
		return hit.leg, hit.has, nil
	}
	if o.Style != Fast && allow != nil && !allow() {
		leg, has, err := p.leg(ctx, a, b, LegOptions{Style: Fast, AvoidHighways: true}, avoid, link)
		leg.Limited = true
		return leg, has, err // not cached: the curvy leg is still owed once the budget refills
	}
	leg, has, err := p.leg(ctx, a, b, o, avoid, link)
	if err != nil {
		return Leg{}, valhalla.Avoid{}, err
	}
	p.mu.Lock()
	if p.cache == nil || len(p.cache) > 2000 {
		p.cache = map[string]planned{}
	}
	p.cache[key] = planned{leg, has}
	p.mu.Unlock()
	return leg, has, nil
}

func (p *Planner) leg(ctx context.Context, a, b valhalla.Point, o LegOptions, avoid, link valhalla.Avoid) (Leg, valhalla.Avoid, error) {
	own := avoid
	own.Highways = o.AvoidHighways || o.Style != Fast
	var r valhalla.Route
	var err error
	if o.Style == Fast {
		r, err = p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: own})
	} else {
		r, err = p.curviest(ctx, a, b, own, slack[o.Style])
	}
	if err != nil {
		return Leg{}, valhalla.Avoid{}, err
	}
	path := r.Legs[0].Path
	leg := Leg{Meters: r.Meters, Seconds: r.Seconds, Path: path, Via: []latLng{}, Curvature: Curvature(path)}
	// Google, asked with the link's settings, already drives a fast leg with the same settings.
	if o.Style != Fast || own != link {
		if leg.Via, err = p.shape(ctx, a, b, path, link); err != nil {
			return Leg{}, valhalla.Avoid{}, err
		}
	}
	return leg, r.Has, nil
}

// curviest looks for bendier ways than the fast one by closing off the fast way's straightest
// stretches, round after round, and keeps the bendiest that stays within slack of the fast time.
func (p *Planner) curviest(ctx context.Context, a, b valhalla.Point, avoid valhalla.Avoid, slack float64) (valhalla.Route, error) {
	fast, err := p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: avoid})
	if err != nil {
		return valhalla.Route{}, err
	}
	best, bestCurves := fast, Curvature(fast.Legs[0].Path)
	current := fast
	var closed [][]valhalla.Point
	for range searchRounds {
		mids := straightest(current.Legs[0].Path, 2, 1500, 1000)
		if len(mids) == 0 {
			break
		}
		for _, m := range mids {
			closed = append(closed, square(m, 120))
		}
		r, err := p.Router.Route(ctx, []valhalla.Point{a, b}, valhalla.Options{Avoid: avoid, Exclude: closed})
		if err != nil {
			break // Closed off every way through; the rounds so far are what there is.
		}
		current = r
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
