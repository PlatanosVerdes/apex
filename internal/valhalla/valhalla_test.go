package valhalla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	// Google's documented example, scaled: the same characters read with 6 decimals.
	got := decode("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	want := [][2]float64{{3.85, -12.02}, {4.07, -12.095}, {4.3252, -12.6453}}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestRouteChunksAndAvoids(t *testing.T) {
	var sizes []int
	var options []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Locations      []any                     `json:"locations"`
			CostingOptions map[string]map[string]any `json:"costing_options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sizes = append(sizes, len(body.Locations))
		options = append(options, body.CostingOptions["motorcycle"])
		legs := strings.TrimSuffix(strings.Repeat(`{"summary":{"length":1,"time":60},"shape":"_p~iF~ps|U"},`, len(body.Locations)-1), ",")
		fmt.Fprintf(w, `{"trip":{"summary":{"length":%d,"time":%d,"has_ferry":%v},"legs":[%s]}}`,
			len(body.Locations)-1, 60*(len(body.Locations)-1), len(sizes) == 2, legs)
	}))
	defer srv.Close()

	pts := make([]Point, 25)
	for i := range pts {
		pts[i] = Point{Lat: 39 + float64(i)/100, Lng: 2.6}
	}
	c := New(srv.URL, "test")
	r, err := c.Route(context.Background(), pts, Options{Avoid: Avoid{Highways: true, Tolls: true}})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(sizes) != "[10 10 7]" {
		t.Errorf("chunks must share their ends to cover 25 points: %v", sizes)
	}
	if len(r.Legs) != 24 || r.Meters != 24000 || r.Seconds != 1440 || !r.Has.Ferries {
		t.Errorf("%d legs, %v m, %v s, has %+v", len(r.Legs), r.Meters, r.Seconds, r.Has)
	}
	if o := options[0]; o["use_highways"] != 0.0 || o["use_tolls"] != 0.0 || o["use_ferry"] != nil {
		t.Errorf("options %v", o)
	}

	if _, err := c.Route(context.Background(), pts, Options{Avoid: Avoid{Highways: true, Tolls: true}}); err != nil || len(sizes) != 3 {
		t.Errorf("the same route again should come from the cache: %d requests, %v", len(sizes), err)
	}
}

func TestRouteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_code":442,"error":"No path could be found for input"}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "test").Route(context.Background(), []Point{{39.57, 2.65}, {38.9, 1.4}}, Options{})
	if err == nil || !strings.Contains(err.Error(), "No path") {
		t.Fatalf("got %v", err)
	}
}

func TestRetriesWhenBusyAndBoundsCalls(t *testing.T) {
	var calls, inFlight, most atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		if calls.Add(1)%3 == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("<html>busy</html>"))
			return
		}
		_, _ = w.Write([]byte(`{"trip":{"summary":{"length":1,"time":60},"legs":[{"summary":{"length":1,"time":60},"shape":"_p~iF~ps|U"}]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "test")
	c.retryWait = time.Millisecond
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = c.Route(context.Background(), []Point{{Lat: 39 + float64(i)/100, Lng: 2}, {Lat: 39.5, Lng: 2}}, Options{})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("route %d: %v", i, err)
		}
	}
	if most.Load() > 2 {
		t.Errorf("%d calls in flight at once, the cap is 2", most.Load())
	}
}

func TestFastCallsSkipTheSearchQueue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "exclude_polygons") {
			time.Sleep(300 * time.Millisecond)
		}
		_, _ = w.Write([]byte(`{"trip":{"summary":{"length":1,"time":60},"legs":[{"summary":{"length":1,"time":60},"shape":"_p~iF~ps|U"}]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "test")
	search := Searching(context.Background())
	closed := [][]Point{{{Lat: 39.1, Lng: 2}, {Lat: 39.2, Lng: 2}, {Lat: 39.2, Lng: 2.1}}}
	for i := range 4 {
		go func() {
			_, _ = c.Route(search, []Point{{Lat: 39 + float64(i)/100, Lng: 2}, {Lat: 39.5, Lng: 2}}, Options{Exclude: closed})
		}()
	}
	time.Sleep(20 * time.Millisecond)
	start := time.Now()
	if _, err := c.Route(context.Background(), []Point{{Lat: 40, Lng: 2}, {Lat: 40.5, Lng: 2}}, Options{}); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 150*time.Millisecond {
		t.Errorf("a fast call waited %v behind searches", took)
	}
}
