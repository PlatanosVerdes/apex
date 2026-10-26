// Package limit keeps a refilling budget per user, so nobody floods the public routing server
// with curve searches.
package limit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	at     time.Time
}

// Budget gives every key Capacity tokens that refill one every Refill.
type Budget struct {
	Capacity int
	Refill   time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

func New(capacity int, refill time.Duration) *Budget {
	return &Budget{Capacity: capacity, Refill: refill, buckets: map[string]*bucket{}, now: time.Now}
}

// Take spends one of key's tokens and reports whether there was one.
func (b *Budget) Take(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := b.fill(key)
	if k.tokens < 1 {
		return false
	}
	k.tokens--
	return true
}

// Status returns the whole tokens key has left and how long until the next one comes back.
func (b *Budget) Status(key string) (left int, next time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := b.fill(key)
	left = int(k.tokens)
	if left < b.Capacity {
		next = time.Duration((1 - (k.tokens - float64(left))) * float64(b.Refill))
	}
	return left, next
}

// fill brings key's bucket up to date; buckets left full are dropped now and then.
func (b *Budget) fill(key string) *bucket {
	now := b.now()
	if len(b.buckets) > 5000 {
		for k, v := range b.buckets {
			if now.Sub(v.at) > time.Duration(b.Capacity)*b.Refill {
				delete(b.buckets, k)
			}
		}
	}
	k, ok := b.buckets[key]
	if !ok {
		k = &bucket{tokens: float64(b.Capacity), at: now}
		b.buckets[key] = k
	}
	k.tokens = min(float64(b.Capacity), k.tokens+float64(now.Sub(k.at))/float64(b.Refill))
	k.at = now
	return k
}
