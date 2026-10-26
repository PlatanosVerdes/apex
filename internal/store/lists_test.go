package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/PlatanosVerdes/apex/internal/gmaps"
)

func TestLists(t *testing.T) {
	s, err := OpenLists(filepath.Join(t.TempDir(), "lists.json"))
	if err != nil {
		t.Fatal(err)
	}
	if all, err := s.All(); err != nil || len(all) != 0 {
		t.Fatalf("empty store: %v %v", all, err)
	}
	a := LinkedList{List: gmaps.List{ID: "a", Name: "Ruta moto"}}
	b := LinkedList{List: gmaps.List{ID: "b", Name: "Barcelona"}}
	first, _ := s.Put(a)
	second, _ := s.Put(b)
	if first.Colour == "" || first.Colour == second.Colour {
		t.Errorf("new lists get distinct colours: %q %q", first.Colour, second.Colour)
	}
	if _, err := s.SetColour("a", "red"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetColour("a", "#ff0000"); err == nil {
		t.Error("a colour outside the palette was accepted")
	}
	a.Places = []gmaps.Place{{Name: "Pedraforca", Lat: 42.24, Lng: 1.71}}
	if _, err := s.Put(a); err != nil {
		t.Fatal(err)
	}
	all, _ := s.All()
	if len(all) != 2 || all[0].ID != "a" || len(all[0].Places) != 1 || all[0].Colour != "red" {
		t.Fatalf("a refresh must replace in place and keep the colour: %+v", all)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}
