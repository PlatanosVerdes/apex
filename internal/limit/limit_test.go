package limit

import (
	"testing"
	"time"
)

func TestBudget(t *testing.T) {
	clock := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	b := New(3, 30*time.Second)
	b.now = func() time.Time { return clock }

	for i := range 3 {
		if !b.Take("rider") {
			t.Fatalf("token %d refused", i+1)
		}
	}
	if b.Take("rider") {
		t.Fatal("a fourth token in a row was given")
	}
	if !b.Take("friend") {
		t.Error("another user's budget is their own")
	}
	if left, next := b.Status("rider"); left != 0 || next != 30*time.Second {
		t.Errorf("empty: %d left, next in %s", left, next)
	}

	clock = clock.Add(40 * time.Second)
	if left, next := b.Status("rider"); left != 1 || next != 20*time.Second {
		t.Errorf("after 40s: %d left, next in %s", left, next)
	}
	clock = clock.Add(time.Hour)
	if left, next := b.Status("rider"); left != 3 || next != 0 {
		t.Errorf("refills up to its capacity only: %d left, next in %s", left, next)
	}
}
