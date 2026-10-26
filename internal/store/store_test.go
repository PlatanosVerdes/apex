package store

import (
	"errors"
	"testing"
	"time"
)

func TestPutGetListDelete(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { clock = clock.Add(time.Minute); return clock }

	first, err := s.Put(Route{Name: "Tramuntana", Stops: []Stop{{Lat: 39.57, Lng: 2.65, Name: "Palma"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !validID.MatchString(first.ID) {
		t.Fatalf("id %q", first.ID)
	}
	second, err := s.Put(Route{Name: "Llevant"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(first.ID)
	if err != nil || got.Stops[0].Name != "Palma" {
		t.Fatalf("get: %+v %v", got, err)
	}
	list, err := s.List()
	if err != nil || len(list) != 2 || list[0].ID != second.ID {
		t.Fatalf("list should be newest first: %+v %v", list, err)
	}

	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted route still there: %v", err)
	}
}

func TestRejects(t *testing.T) {
	s, _ := Open(t.TempDir())
	for name, r := range map[string]Route{
		"no name":    {Name: " "},
		"bad lat":    {Name: "x", Stops: []Stop{{Lat: 91}}},
		"foreign id": {Name: "x", ID: "../../etc/passwd"},
		"too many":   {Name: "x", Stops: make([]Stop, MaxStops+1)},
	} {
		if _, err := s.Put(r); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
	if _, err := s.Get("../x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("path id: %v", err)
	}
}
