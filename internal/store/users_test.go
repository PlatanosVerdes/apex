package store

import (
	"errors"
	"testing"

	"github.com/PlatanosVerdes/apex/internal/gmaps"
)

func TestUsersAreApart(t *testing.T) {
	users, err := OpenUsers(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := users.For("aaaaaaaaaaaaaaaaaaaaaaaa")
	b, _ := users.For("bbbbbbbbbbbbbbbbbbbbbbbb")
	saved, err := a.Routes.Put(Route{Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := b.Routes.List(); len(list) != 0 {
		t.Errorf("another rider sees my routes: %+v", list)
	}
	if _, err := b.Routes.Get(saved.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another rider can open my route by its id: %v", err)
	}
	for _, id := range []string{"../aaaaaaaaaaaaaaaaaaaaaa", "AAAAAAAAAAAAAAAAAAAAAAAA", ""} {
		if _, err := users.For(id); err == nil {
			t.Errorf("user id %q accepted", id)
		}
	}
	n := 0
	_ = users.Each(func(*UserData) { n++ })
	if n != 2 {
		t.Errorf("Each saw %d riders", n)
	}
}

func TestAccountLimits(t *testing.T) {
	users, _ := OpenUsers(t.TempDir())
	u, _ := users.For("cccccccccccccccccccccccc")
	u.Routes.MaxRoutes, u.Lists.MaxLists = 2, 1
	first, _ := u.Routes.Put(Route{Name: "1"})
	_, _ = u.Routes.Put(Route{Name: "2"})
	if _, err := u.Routes.Put(Route{Name: "3"}); !errors.Is(err, ErrLimit) {
		t.Errorf("a third route over a cap of two: %v", err)
	}
	first.Name = "1 renamed"
	if _, err := u.Routes.Put(first); err != nil {
		t.Errorf("editing a saved route is not a new one: %v", err)
	}
	_, _ = u.Lists.Put(LinkedList{List: gmaps.List{ID: "a"}})
	if _, err := u.Lists.Put(LinkedList{List: gmaps.List{ID: "b"}}); !errors.Is(err, ErrLimit) {
		t.Errorf("a second list over a cap of one: %v", err)
	}
	if _, err := u.Lists.Put(LinkedList{List: gmaps.List{ID: "a", Name: "refreshed"}}); err != nil {
		t.Errorf("refreshing a linked list is not a new one: %v", err)
	}
}
