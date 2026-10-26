package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/PlatanosVerdes/apex/internal/gmaps"
)

// Colours are the names the page has a style for; a list's markers take one.
var Colours = []string{"teal", "blue", "green", "amber", "red", "slate", "cyan", "brown"}

// LinkedList is a Google Maps list kept in sync from its share link.
type LinkedList struct {
	gmaps.List
	URL     string    `json:"url"`
	Colour  string    `json:"colour"`
	Fetched time.Time `json:"fetched"`
}

// Lists keeps every linked list in one JSON file; there are a handful at most.
type Lists struct {
	// MaxLists caps how many lists can be linked; zero means no cap.
	MaxLists int

	path string
	mu   sync.Mutex
}

func OpenLists(path string) (*Lists, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &Lists{path: path}, nil
}

func (s *Lists) Path() string { return s.path }

func (s *Lists) All() ([]LinkedList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Put adds l, or replaces the list with its ID keeping its place in the order and, when l has
// none, its colour. A new list without a colour gets the first one no other list uses.
func (s *Lists) Put(l LinkedList) (LinkedList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.read()
	if err != nil {
		return LinkedList{}, err
	}
	i := slices.IndexFunc(all, func(x LinkedList) bool { return x.ID == l.ID })
	if l.Colour == "" && i >= 0 {
		l.Colour = all[i].Colour
	}
	if l.Colour == "" {
		l.Colour = Colours[len(all)%len(Colours)]
		for _, c := range Colours {
			if !slices.ContainsFunc(all, func(x LinkedList) bool { return x.Colour == c }) {
				l.Colour = c
				break
			}
		}
	}
	if i >= 0 {
		all[i] = l
	} else {
		if s.MaxLists > 0 && len(all) >= s.MaxLists {
			return LinkedList{}, ErrLimit
		}
		all = append(all, l)
	}
	return l, s.write(all)
}

func (s *Lists) SetColour(id, colour string) (LinkedList, error) {
	if !slices.Contains(Colours, colour) {
		return LinkedList{}, errors.New("unknown colour " + colour)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.read()
	if err != nil {
		return LinkedList{}, err
	}
	i := slices.IndexFunc(all, func(x LinkedList) bool { return x.ID == id })
	if i < 0 {
		return LinkedList{}, ErrNotFound
	}
	all[i].Colour = colour
	return all[i], s.write(all)
}

func (s *Lists) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(all, func(x LinkedList) bool { return x.ID == id })
	if len(kept) == len(all) {
		return ErrNotFound
	}
	return s.write(kept)
}

func (s *Lists) read() ([]LinkedList, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []LinkedList{}, nil
	}
	if err != nil {
		return nil, err
	}
	var all []LinkedList
	return all, json.Unmarshal(data, &all)
}

func (s *Lists) write(all []LinkedList) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
