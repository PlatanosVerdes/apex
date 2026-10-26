// Package metrics counts what Apex does and writes it in Prometheus's text format, without pulling
// in the client library for a dozen numbers.
package metrics

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

type key struct {
	name   string
	labels string
}

// Registry holds counters and latency histograms keyed by name and labels, and gauges read at
// scrape time.
type Registry struct {
	mu       sync.Mutex
	help     map[string]string
	kinds    map[string]string
	counters map[key]float64
	hists    map[key]*histogram
	gauges   map[string]func() float64
	riders   map[string]time.Time
	now      func() time.Time
}

// Buckets suit calls to the public routing and geocoding servers: tens of milliseconds to seconds.
var buckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

type histogram struct {
	counts []uint64
	sum    float64
	total  uint64
}

func New() *Registry {
	return &Registry{
		help: map[string]string{}, kinds: map[string]string{},
		counters: map[key]float64{}, hists: map[key]*histogram{}, gauges: map[string]func() float64{},
		riders: map[string]time.Time{}, now: time.Now,
	}
}

// Labels turns pairs into Prometheus's {a="b",c="d"}.
func Labels(pairs ...string) string {
	if len(pairs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		v := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(pairs[i+1])
		parts = append(parts, fmt.Sprintf("%s=%q", pairs[i], v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func (r *Registry) describe(name, kind, help string) {
	if _, ok := r.kinds[name]; !ok {
		r.kinds[name], r.help[name] = kind, help
	}
}

func (r *Registry) Add(name, help string, value float64, pairs ...string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.describe(name, "counter", help)
	r.counters[key{name, Labels(pairs...)}] += value
}

func (r *Registry) Inc(name, help string, pairs ...string) { r.Add(name, help, 1, pairs...) }

func (r *Registry) Observe(name, help string, d time.Duration, pairs ...string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.describe(name, "histogram", help)
	k := key{name, Labels(pairs...)}
	h := r.hists[k]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(buckets))}
		r.hists[k] = h
	}
	s := d.Seconds()
	for i, b := range buckets {
		if s <= b {
			h.counts[i]++
		}
	}
	h.sum += s
	h.total++
}

// Gauge registers a value read at every scrape.
func (r *Registry) Gauge(name, help string, read func() float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.describe(name, "gauge", help)
	r.gauges[name] = read
}

// Seen notes that a rider (an account or an address) used Apex now, for the riders-per-day gauge.
// Only the key is kept, never shown.
func (r *Registry) Seen(rider string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.riders[rider] = r.now()
}

func (r *Registry) ridersSince(window time.Duration, prefix string) float64 {
	cut := r.now().Add(-window)
	n := 0
	for k, at := range r.riders {
		if at.Before(cut.Add(-24 * time.Hour)) {
			delete(r.riders, k)
			continue
		}
		if at.After(cut) && strings.HasPrefix(k, prefix) {
			n++
		}
	}
	return float64(n)
}

func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := map[string]float64{}
	for name, read := range r.gauges {
		values[name] = read()
	}
	r.describe("apex_riders", "gauge", "Riders who used Apex in the window, signed in or by address.")
	for _, name := range slices.Sorted(maps.Keys(r.kinds)) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, r.help[name], name, r.kinds[name])
		switch r.kinds[name] {
		case "gauge":
			if name == "apex_riders" {
				for _, win := range []struct {
					label string
					d     time.Duration
				}{{"1h", time.Hour}, {"24h", 24 * time.Hour}} {
					fmt.Fprintf(w, "%s%s %g\n", name, Labels("window", win.label, "kind", "account"), r.ridersSince(win.d, "user:"))
					fmt.Fprintf(w, "%s%s %g\n", name, Labels("window", win.label, "kind", "anonymous"), r.ridersSince(win.d, "ip:"))
				}
				continue
			}
			fmt.Fprintf(w, "%s %g\n", name, values[name])
		case "counter":
			for _, k := range sortedKeys(r.counters, name) {
				fmt.Fprintf(w, "%s%s %g\n", name, k.labels, r.counters[k])
			}
		case "histogram":
			for _, k := range sortedKeys(r.hists, name) {
				h := r.hists[k]
				for i, b := range buckets {
					fmt.Fprintf(w, "%s_bucket%s %d\n", name, withLe(k.labels, fmt.Sprint(b)), h.counts[i])
				}
				fmt.Fprintf(w, "%s_bucket%s %d\n", name, withLe(k.labels, "+Inf"), h.total)
				fmt.Fprintf(w, "%s_sum%s %g\n%s_count%s %d\n", name, k.labels, h.sum, name, k.labels, h.total)
			}
		}
	}
}

func withLe(labels, le string) string {
	if labels == "" {
		return Labels("le", le)
	}
	return labels[:len(labels)-1] + fmt.Sprintf(`,le=%q}`, le)
}

func sortedKeys[V any](m map[key]V, name string) []key {
	var out []key
	for k := range m {
		if k.name == name {
			out = append(out, k)
		}
	}
	slices.SortFunc(out, func(a, b key) int { return strings.Compare(a.labels, b.labels) })
	return out
}
