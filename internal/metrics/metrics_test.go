package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestWrite(t *testing.T) {
	r := New()
	clock := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }
	r.Inc("apex_plans_total", "Routes worked out.", "mode", "advanced")
	r.Inc("apex_plans_total", "Routes worked out.", "mode", "advanced")
	r.Inc("apex_plans_total", "Routes worked out.", "mode", "simple")
	r.Observe("apex_upstream_seconds", "Time.", 300*time.Millisecond, "service", "valhalla")
	r.Gauge("apex_accounts", "Accounts.", func() float64 { return 3 })
	r.Seen("user:a")
	r.Seen("ip:1.2.3.4")
	clock = clock.Add(2 * time.Hour)
	r.Seen("user:b")

	var b strings.Builder
	r.Write(&b)
	out := b.String()
	for _, want := range []string{
		"# TYPE apex_plans_total counter",
		`apex_plans_total{mode="advanced"} 2`,
		`apex_plans_total{mode="simple"} 1`,
		`apex_upstream_seconds_bucket{service="valhalla",le="0.25"} 0`,
		`apex_upstream_seconds_bucket{service="valhalla",le="0.5"} 1`,
		`apex_upstream_seconds_bucket{service="valhalla",le="+Inf"} 1`,
		`apex_upstream_seconds_count{service="valhalla"} 1`,
		"apex_accounts 3",
		`apex_riders{window="1h",kind="account"} 1`,
		`apex_riders{window="24h",kind="account"} 2`,
		`apex_riders{window="24h",kind="anonymous"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "1.2.3.4") || strings.Contains(out, "user:a") {
		t.Error("who the riders are must never reach the metrics")
	}
}

func TestNilRegistryIsQuiet(t *testing.T) {
	var r *Registry
	r.Inc("x", "y")
	r.Observe("x", "y", time.Second)
	r.Seen("user:a")
}
