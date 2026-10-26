package gmaps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A trimmed, made-up response in the endpoint's shape: a named place, a dropped pin, an unnamed
// pin without address, and an item with no coordinates.
const sample = `)]}'
[[["LIST_ID_ABCDEFGHIJKLMNOP",1,[[1],[9]],1,1],4,[2,1,"https://www.google.com/maps/placelists/list/LIST_ID_ABCDEFGHIJKLMNOP"],["Owner","https://example.com/a.png","1"],"Ruta moto","",null,null,[
[null,[null,null,"22640 Portalet d'Aneu, Huesca",null,"22640, Huesca",[null,null,42.80514,-0.41867],["1","2"],"/g/x"],"Portalet d'Aneu","Mejor por la mañana"],
[null,[null,null,"",null,"Dirección cercana: BV-4243, 10, 08618 Castellar del Riu, Barcelona",[null,null,42.1367492,1.763023]],"Marcador",""],
[null,[null,null,"",null,"",[null,null,41.71101,2.02858]],"",""],
[null,[null,null,"Somewhere",null,"Nowhere"],"No coordinates",""]
],[],[1,0],[1,0],4,null,null,null,null,"🏍️"]]`

func TestParse(t *testing.T) {
	l, err := parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "Ruta moto" || l.Icon != "🏍️" || len(l.Places) != 3 {
		t.Fatalf("%+v", l)
	}
	want := []Place{
		{Name: "Portalet d'Aneu", Address: "22640, Huesca", Note: "Mejor por la mañana", Lat: 42.80514, Lng: -0.41867},
		{Name: "BV-4243, 10, 08618 Castellar del Riu, Barcelona", Address: "Dirección cercana: BV-4243, 10, 08618 Castellar del Riu, Barcelona", Lat: 42.1367492, Lng: 1.763023},
		{Name: "41.71101, 2.02858", Lat: 41.71101, Lng: 2.02858},
	}
	for i := range want {
		if l.Places[i] != want[i] {
			t.Errorf("place %d:\n got %+v\nwant %+v", i, l.Places[i], want[i])
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, body := range []string{"<html>", ")]}'\n[]", ")]}'\nnot json"} {
		if _, err := parse([]byte(body)); err == nil {
			t.Errorf("%q parsed", body)
		}
	}
}

func TestResolve(t *testing.T) {
	c := New()
	for link, want := range map[string]string{
		"https://www.google.com/maps/placelists/list/-Y8Nwb580PwAaTunDZCaY6GJkVlXeA":                "-Y8Nwb580PwAaTunDZCaY6GJkVlXeA",
		"https://www.google.com/maps/@/data=!3m1!4b1!4m3!11m2!2s-Y8Nwb580PwAaTunDZCaY6GJkVlXeA!3e3": "-Y8Nwb580PwAaTunDZCaY6GJkVlXeA",
	} {
		if got, err := c.Resolve(context.Background(), link); err != nil || got != want {
			t.Errorf("%s: %q %v", link, got, err)
		}
	}
	for _, link := range []string{
		"http://maps.app.goo.gl/x",
		"https://evil.example/placelists/list/-Y8Nwb580PwAaTunDZCaY6GJkVlXeA",
		"https://www.google.com/maps/place/Sóller",
		"not a url",
	} {
		if _, err := c.Resolve(context.Background(), link); err == nil {
			t.Errorf("%s: resolved", link)
		}
	}
}

func TestFetch(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()
	c := New()
	c.base = srv.URL
	l, err := c.Fetch(context.Background(), "-Y8Nwb580PwAaTunDZCaY6GJkVlXeA")
	if err != nil || l.ID != "-Y8Nwb580PwAaTunDZCaY6GJkVlXeA" || len(l.Places) != 3 {
		t.Fatalf("%+v %v", l, err)
	}
	if !strings.Contains(query, "!1s-Y8Nwb580PwAaTunDZCaY6GJkVlXeA!") {
		t.Errorf("list id missing from %s", query)
	}
}
