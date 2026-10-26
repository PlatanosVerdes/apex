package plan

import (
	"math"
	"sort"

	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

type latLng = [2]float64

func meters(a, b latLng) float64 {
	k := math.Cos(a[0] * math.Pi / 180)
	return math.Hypot((a[0]-b[0])*111320, (a[1]-b[1])*111320*k)
}

// resample returns points every step metres along path, so measures do not depend on how densely
// the router drew each road.
func resample(path []latLng, step float64) []latLng {
	if len(path) == 0 {
		return nil
	}
	out := []latLng{path[0]}
	need := step
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		d := meters(a, b)
		for d >= need {
			f := need / d
			a = latLng{a[0] + (b[0]-a[0])*f, a[1] + (b[1]-a[1])*f}
			out = append(out, a)
			d -= need
			need = step
		}
		need -= d
	}
	return out
}

const sampleStep = 30.0

// turns gives the heading change in degrees at each resampled point. Changes over 120° are a
// turn at a junction or a U-turn, not a bend, and count as nothing.
func turns(samples []latLng) []float64 {
	heading := func(a, b latLng) float64 {
		k := math.Cos(a[0] * math.Pi / 180)
		return math.Atan2(b[0]-a[0], (b[1]-a[1])*k) * 180 / math.Pi
	}
	out := make([]float64, 0, len(samples))
	for i := 2; i < len(samples); i++ {
		d := math.Abs(math.Mod(heading(samples[i-1], samples[i])-heading(samples[i-2], samples[i-1])+540, 360) - 180)
		if d >= 120 {
			d = 0
		}
		out = append(out, d)
	}
	return out
}

// Curvature is the degrees turned per kilometre: a motorway is near 0, a mountain pass several hundred.
func Curvature(path []latLng) float64 {
	samples := resample(path, sampleStep)
	if len(samples) < 3 {
		return 0
	}
	var sum float64
	for _, t := range turns(samples) {
		sum += t
	}
	return sum / (float64(len(samples)) * sampleStep / 1000)
}

// straightest returns the middles of the n straightest windows of path, keeping clear of both
// ends so the stops themselves stay reachable.
func straightest(path []latLng, n int, window, margin float64) []latLng {
	samples := resample(path, sampleStep)
	t := turns(samples)
	w, m := int(window/sampleStep), int(margin/sampleStep)
	type candidate struct {
		sum float64
		at  int
	}
	var cands []candidate
	for i := m; i+w+m <= len(t); i++ {
		var sum float64
		for _, v := range t[i : i+w] {
			sum += v
		}
		cands = append(cands, candidate{sum, i})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].sum < cands[j].sum })
	var picked []candidate
	for _, c := range cands {
		apart := true
		for _, p := range picked {
			if abs(c.at-p.at) <= w {
				apart = false
				break
			}
		}
		if apart {
			picked = append(picked, c)
		}
		if len(picked) == n {
			break
		}
	}
	out := make([]latLng, len(picked))
	for i, p := range picked {
		out[i] = samples[p.at+w/2+1]
	}
	return out
}

// square is a closed ring around c with half-side r metres.
func square(c latLng, r float64) []valhalla.Point {
	dLat := r / 111320
	dLng := r / (111320 * math.Cos(c[0]*math.Pi/180))
	return []valhalla.Point{
		{Lat: c[0] - dLat, Lng: c[1] - dLng}, {Lat: c[0] - dLat, Lng: c[1] + dLng},
		{Lat: c[0] + dLat, Lng: c[1] + dLng}, {Lat: c[0] + dLat, Lng: c[1] - dLng},
		{Lat: c[0] - dLat, Lng: c[1] - dLng},
	}
}

// offRoute returns the stretches of want that run farther than far metres from got, as index
// ranges into want's resampled points, ignoring stretches shorter than minLen metres.
func offRoute(want, got []latLng, far, minLen, step float64) (samples []latLng, ranges [][2]int) {
	samples = resample(want, step)
	cell := far
	key := func(p latLng) [2]int {
		return [2]int{int(math.Floor(p[0] * 111320 / cell)), int(math.Floor(p[1] * 111320 * math.Cos(p[0]*math.Pi/180) / cell))}
	}
	grid := map[[2]int][]latLng{}
	for _, p := range resample(got, step) {
		k := key(p)
		grid[k] = append(grid[k], p)
	}
	near := func(p latLng) bool {
		k := key(p)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				for _, q := range grid[[2]int{k[0] + dy, k[1] + dx}] {
					if meters(p, q) <= far {
						return true
					}
				}
			}
		}
		return false
	}
	start := -1
	flush := func(end int) {
		if start >= 0 && float64(end-start)*step >= minLen {
			ranges = append(ranges, [2]int{start, end})
		}
		start = -1
	}
	for i, p := range samples {
		if near(p) {
			flush(i)
		} else if start < 0 {
			start = i
		}
	}
	flush(len(samples))
	return samples, ranges
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
