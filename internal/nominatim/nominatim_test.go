package nominatim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLabel(t *testing.T) {
	for _, c := range []struct {
		name string
		addr map[string]string
		want Label
	}{
		{"", map[string]string{"road": "Ma-10", "town": "Sóller"}, Label{"Sóller", "Ma-10"}},
		{"Sa Calobra", map[string]string{"village": "Escorca"}, Label{"Escorca", "Sa Calobra"}},
		{"", map[string]string{"city": "Palma"}, Label{"Palma", ""}},
		{"", map[string]string{"road": "Ma-2141"}, Label{"", "Ma-2141"}},
		{"Deià", map[string]string{"village": "Deià"}, Label{"Deià", ""}},
		{"", map[string]string{"road": "avinguda de sa Font Seca", "town": "Pollença"}, Label{"Pollença", "Avinguda de sa Font Seca"}},
		{"", map[string]string{}, Label{}},
	} {
		if got := label(c.name, c.addr); got != c.want {
			t.Errorf("label(%q, %v) = %+v, want %+v", c.name, c.addr, got, c.want)
		}
	}
}

func TestNameIsCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"name":"","address":{"road":"Ma-10","town":"Sóller"}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "test")
	c.gap = 0
	for range 2 {
		l, err := c.Name(context.Background(), 39.76671, 2.71502)
		if err != nil || l != (Label{"Sóller", "Ma-10"}) {
			t.Fatalf("%+v %v", l, err)
		}
	}
	if calls != 1 {
		t.Errorf("%d calls, the second should come from the cache", calls)
	}
}
