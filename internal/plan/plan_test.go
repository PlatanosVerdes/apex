package plan

import (
	"context"
	"math"
	"testing"

	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

// line runs from a to b in n straight steps.
func line(a, b latLng, n int) []latLng {
	out := make([]latLng, n+1)
	for i := range out {
		f := float64(i) / float64(n)
		out[i] = latLng{a[0] + (b[0]-a[0])*f, a[1] + (b[1]-a[1])*f}
	}
	return out
}

// zigzag runs from a to b swinging sideways by amp degrees every few hundred metres.
func zigzag(a, b latLng, n int, amp float64) []latLng {
	out := line(a, b, n)
	for i := 1; i < n; i++ {
		out[i][1] += amp * math.Sin(float64(i)*math.Pi/2)
	}
	return out
}

func TestCurvature(t *testing.T) {
	a, b := latLng{39.5, 2.6}, latLng{39.7, 2.6}
	straight := Curvature(line(a, b, 50))
	bendy := Curvature(zigzag(a, b, 100, 0.003))
	if straight > 1 {
		t.Errorf("a straight road turns %v°/km", straight)
	}
	if bendy < 100 {
		t.Errorf("a zigzag turns only %v°/km", bendy)
	}
}

func TestResampleSpacing(t *testing.T) {
	path := line(latLng{39.5, 2.6}, latLng{39.6, 2.6}, 7) // about 11.1 km in uneven pieces
	s := resample(path, 100)
	if n := len(s); n < 110 || n > 112 {
		t.Fatalf("%d samples for 11.1 km every 100 m", n)
	}
	for i := 1; i < len(s); i++ {
		if d := meters(s[i-1], s[i]); math.Abs(d-100) > 1 {
			t.Fatalf("sample %d is %v m from the previous", i, d)
		}
	}
}

func TestOffRoute(t *testing.T) {
	a, b := latLng{39.5, 2.6}, latLng{39.6, 2.6}
	// want bulges 2 km east in its middle third; got goes straight.
	want := []latLng{a, {39.533, 2.6}, {39.55, 2.625}, {39.567, 2.6}, b}
	_, strays := offRoute(want, line(a, b, 20), 200, 300, 50)
	if len(strays) != 1 {
		t.Fatalf("want one stray stretch, got %v", strays)
	}
	if _, none := offRoute(line(a, b, 20), line(a, b, 3), 200, 300, 50); len(none) != 0 {
		t.Errorf("the same road drawn differently strays: %v", none)
	}
}

// fakeRouter goes straight between the points it is given, except that once a stretch is closed
// off it takes a bendy detour, and with avoid.Highways it takes a slower straight road.
type fakeRouter struct {
	detourSeconds float64
	calls         int
}

func (f *fakeRouter) Route(_ context.Context, pts []valhalla.Point, o valhalla.Options) (valhalla.Route, error) {
	f.calls++
	var r valhalla.Route
	for i := 1; i < len(pts); i++ {
		a, b := latLng{pts[i-1].Lat, pts[i-1].Lng}, latLng{pts[i].Lat, pts[i].Lng}
		leg := valhalla.Leg{Meters: meters(a, b), Seconds: meters(a, b) / 25, Path: line(a, b, 40)}
		if len(o.Exclude) > 0 {
			// Another valley, 3 km to the east.
			mid := latLng{(a[0] + b[0]) / 2, (a[1]+b[1])/2 + 0.035}
			leg.Path = append(zigzag(a, mid, 100, 0.0005), zigzag(mid, b, 100, 0.0005)[1:]...)
			leg.Seconds = f.detourSeconds
		}
		r.Legs = append(r.Legs, leg)
		r.Meters += leg.Meters
		r.Seconds += leg.Seconds
	}
	return r, nil
}

var (
	palma  = valhalla.Point{Lat: 39.50, Lng: 2.60}
	soller = valhalla.Point{Lat: 39.62, Lng: 2.60}
)

func TestSimpleModeIsOneRoute(t *testing.T) {
	f := &fakeRouter{}
	p, err := (&Planner{Router: f}).Plan(context.Background(), []valhalla.Point{palma, soller}, nil, valhalla.Avoid{Tolls: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || len(p.Legs) != 1 || len(p.Legs[0].Via) != 0 || !p.Link.Tolls {
		t.Errorf("%d calls, %+v", f.calls, p)
	}
}

func TestCurvyLegWithinSlack(t *testing.T) {
	straightSeconds := meters(latLng{palma.Lat, palma.Lng}, latLng{soller.Lat, soller.Lng}) / 25
	f := &fakeRouter{detourSeconds: straightSeconds * 1.3}
	p, err := (&Planner{Router: f}).Plan(context.Background(), []valhalla.Point{palma, soller},
		[]LegOptions{{Style: Curvy}}, valhalla.Avoid{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	leg := p.Legs[0]
	if leg.Curvature < 100 {
		t.Errorf("the curvy leg kept the straight road: %v°/km", leg.Curvature)
	}
	if len(leg.Via) == 0 || len(leg.Via) > maxViaPerLeg {
		t.Errorf("a detour Google would not take needs via points, got %d", len(leg.Via))
	}
	if p.Link.Highways {
		t.Error("one curvy leg must not make the whole link avoid motorways")
	}
}

func TestCurvyLegOverSlackStaysFast(t *testing.T) {
	straightSeconds := meters(latLng{palma.Lat, palma.Lng}, latLng{soller.Lat, soller.Lng}) / 25
	f := &fakeRouter{detourSeconds: straightSeconds * 2}
	p, err := (&Planner{Router: f}).Plan(context.Background(), []valhalla.Point{palma, soller},
		[]LegOptions{{Style: Curvy}}, valhalla.Avoid{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Legs[0].Curvature > 1 || len(p.Legs[0].Via) != 0 {
		t.Errorf("a detour twice as long is beyond curvy's slack: %+v", p.Legs[0].Curvature)
	}
}

func TestLinkAvoidsMotorwaysOnlyWhenEveryLegDoes(t *testing.T) {
	three := []valhalla.Point{palma, soller, {Lat: 39.7, Lng: 2.7}}
	pl := &Planner{Router: &fakeRouter{}}
	all, _ := pl.Plan(context.Background(), three, []LegOptions{{Fast, true}, {Fast, true}}, valhalla.Avoid{}, nil)
	some, _ := pl.Plan(context.Background(), three, []LegOptions{{Fast, true}, {Fast, false}}, valhalla.Avoid{}, nil)
	if !all.Link.Highways || some.Link.Highways {
		t.Errorf("all: %v, some: %v", all.Link.Highways, some.Link.Highways)
	}
	if _, err := pl.Plan(context.Background(), three, []LegOptions{{Fast, true}}, valhalla.Avoid{}, nil); err == nil {
		t.Error("one leg option for two legs was accepted")
	}
}

func TestOutOfCurveSearches(t *testing.T) {
	straightSeconds := meters(latLng{palma.Lat, palma.Lng}, latLng{soller.Lat, soller.Lng}) / 25
	f := &fakeRouter{detourSeconds: straightSeconds * 1.3}
	pl := &Planner{Router: f}
	refuse := func() bool { return false }
	p, err := pl.Plan(context.Background(), []valhalla.Point{palma, soller}, []LegOptions{{Style: Curvy}}, valhalla.Avoid{}, refuse)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Legs[0].Limited || p.Legs[0].Curvature > 1 {
		t.Errorf("a refused curve search must fall back to a fast leg, marked: %+v", p.Legs[0].Limited)
	}

	spent := 0
	allow := func() bool { spent++; return true }
	for range 2 {
		p, _ = pl.Plan(context.Background(), []valhalla.Point{palma, soller}, []LegOptions{{Style: Curvy}}, valhalla.Avoid{}, allow)
	}
	if spent != 1 || p.Legs[0].Limited {
		t.Errorf("the same leg twice must spend one search, spent %d", spent)
	}
}
