// Package store keeps saved routes as one JSON file each.
package store

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/PlatanosVerdes/apex/internal/plan"
)

// MaxStops bounds a saved route. Google Maps has opened 25 from a link; this leaves room to find its limit.
const MaxStops = 100

var ErrNotFound = errors.New("route not found")

var validID = regexp.MustCompile(`^[a-z2-7]{16}$`)

type Stop struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Name string  `json:"name,omitempty"`
	// Detail is the road the stop is on, shown under the name.
	Detail string `json:"detail,omitempty"`
	// Leg is how to ride from this stop to the next one, in advanced mode.
	Leg *plan.LegOptions `json:"leg,omitempty"`
}

type Avoid struct {
	Highways bool `json:"highways,omitempty"`
	Tolls    bool `json:"tolls,omitempty"`
	Ferries  bool `json:"ferries,omitempty"`
}

type Route struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Stops    []Stop `json:"stops"`
	Mode     string `json:"mode,omitempty"`
	FromHere bool   `json:"fromHere,omitempty"`
	// HereLeg is how to ride from the rider's position to stop 1, in advanced mode.
	HereLeg   *plan.LegOptions `json:"hereLeg,omitempty"`
	RoundTrip bool             `json:"roundTrip,omitempty"`
	Avoid     Avoid            `json:"avoid"`
	Updated   time.Time        `json:"updated"`
}

func (r Route) Validate() error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 100 {
		return errors.New("name must have 1 to 100 characters")
	}
	if r.Mode != "" && r.Mode != "simple" && r.Mode != "advanced" {
		return fmt.Errorf("unknown mode %q", r.Mode)
	}
	if r.HereLeg != nil {
		if err := r.HereLeg.Validate(); err != nil {
			return err
		}
	}
	if len(r.Stops) > MaxStops {
		return fmt.Errorf("at most %d stops", MaxStops)
	}
	for _, s := range r.Stops {
		if s.Lat < -90 || s.Lat > 90 || s.Lng < -180 || s.Lng > 180 {
			return fmt.Errorf("stop out of range: %v,%v", s.Lat, s.Lng)
		}
		if len(s.Name) > 200 || len(s.Detail) > 200 {
			return errors.New("stop name over 200 characters")
		}
		if s.Leg != nil {
			if err := s.Leg.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

type Store struct {
	// MaxRoutes caps how many routes the store keeps; zero means no cap.
	MaxRoutes int

	dir string
	mu  sync.Mutex
	now func() time.Time
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, now: time.Now}, nil
}

func (s *Store) List() ([]Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	routes := make([]Route, 0, len(files))
	for _, f := range files {
		r, err := read(f)
		if err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	slices.SortFunc(routes, func(a, b Route) int { return b.Updated.Compare(a.Updated) })
	return routes, nil
}

func (s *Store) Get(id string) (Route, error) {
	if !validID.MatchString(id) {
		return Route{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := read(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return Route{}, ErrNotFound
	}
	return r, err
}

// Put saves r, giving it an ID when it has none.
func (s *Store) Put(r Route) (Route, error) {
	if err := r.Validate(); err != nil {
		return Route{}, err
	}
	if r.ID == "" {
		if s.MaxRoutes > 0 {
			if files, _ := filepath.Glob(filepath.Join(s.dir, "*.json")); len(files) >= s.MaxRoutes {
				return Route{}, ErrLimit
			}
		}
		r.ID = newID()
	} else if !validID.MatchString(r.ID) {
		return Route{}, ErrNotFound
	}
	r.Updated = s.now().UTC()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return Route{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path(r.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return Route{}, err
	}
	return r, os.Rename(tmp, s.path(r.ID))
}

func (s *Store) Delete(id string) error {
	if !validID.MatchString(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) path(id string) string { return filepath.Join(s.dir, id+".json") }

func read(path string) (Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Route{}, err
	}
	var r Route
	return r, json.Unmarshal(data, &r)
}

func newID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}
