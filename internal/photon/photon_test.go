package photon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	calls := 0
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"features":[
			{"geometry":{"coordinates":[2.79985,39.85066]},"properties":{"name":"Sa Calobra","city":"Escorca","county":"Serra de Tramuntana","state":"Illes Balears","country":"España"}},
			{"geometry":{"coordinates":[2.1,41.4]},"properties":{"street":"Carrer de Mallorca","housenumber":"401","city":"Barcelona","state":"Catalunya"}},
			{"geometry":{"coordinates":[0,0]},"properties":{"country":"Nowhere"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test")
	near := [2]float64{41.6, 2.0}
	for range 2 {
		got, err := c.Search(context.Background(), "sa calobra", &near)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("results without a name must be dropped: %+v", got)
		}
		if got[0] != (Result{Name: "Sa Calobra", Detail: "Escorca, Serra de Tramuntana", Lat: 39.85066, Lng: 2.79985}) {
			t.Errorf("%+v", got[0])
		}
		if got[1].Name != "Carrer de Mallorca 401" || got[1].Detail != "Barcelona, Catalunya" {
			t.Errorf("an address is named by its street: %+v", got[1])
		}
	}
	if calls != 1 {
		t.Errorf("%d calls, the second search should come from the cache", calls)
	}
	if !strings.Contains(query, "lat=41.600") || !strings.Contains(query, "lon=2.000") {
		t.Errorf("results should be ranked near the route: %s", query)
	}
}
