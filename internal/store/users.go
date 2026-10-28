package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// ErrLimit is a rider asking to keep more than their account allows.
var ErrLimit = errors.New("account limit reached")

const (
	MaxRoutesPerUser = 200
	MaxListsPerUser  = 10
)

var validUserID = regexp.MustCompile(`^[0-9a-f]{24}$`)

// UserData is one rider's folder: their saved routes and linked lists, out of every other rider's reach.
type UserData struct {
	Routes *Store
	Lists  *Lists
	dir    string
	mu     sync.Mutex
}

type Users struct {
	dir  string
	mu   sync.Mutex
	open map[string]*UserData
}

func OpenUsers(dir string) (*Users, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Users{dir: dir, open: map[string]*UserData{}}, nil
}

// For returns the rider's data, creating their folder the first time.
func (u *Users) For(id string) (*UserData, error) {
	if !validUserID.MatchString(id) {
		return nil, ErrNotFound
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if d, ok := u.open[id]; ok {
		return d, nil
	}
	dir := filepath.Join(u.dir, id)
	routes, err := Open(filepath.Join(dir, "routes"))
	if err != nil {
		return nil, err
	}
	routes.MaxRoutes = MaxRoutesPerUser
	lists, err := OpenLists(filepath.Join(dir, "lists.json"))
	if err != nil {
		return nil, err
	}
	lists.MaxLists = MaxListsPerUser
	d := &UserData{Routes: routes, Lists: lists, dir: dir}
	u.open[id] = d
	return d, nil
}

// SaveProfile keeps who the folder belongs to, for whoever looks after the server.
func (d *UserData) SaveProfile(profile any) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.dir, "profile.json"), data, 0o600)
}

// Exists reports whether the rider already has a folder, without creating one.
func (u *Users) Exists(id string) bool {
	if !validUserID.MatchString(id) {
		return false
	}
	_, err := os.Stat(filepath.Join(u.dir, id))
	return err == nil
}

// Each calls fn with every rider who has a folder.
func (u *Users) Each(fn func(*UserData)) error {
	entries, err := os.ReadDir(u.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || !validUserID.MatchString(e.Name()) {
			continue
		}
		d, err := u.For(e.Name())
		if err != nil {
			return err
		}
		fn(d)
	}
	return nil
}

// DonateState is how much a rider has used Apex and when they were last asked to chip in; the page
// decides from it, the server only keeps it so it survives a new browser.
type DonateState struct {
	// ActiveSeconds is time spent using the page with someone at it.
	ActiveSeconds int64 `json:"activeSeconds"`
	// DueSeconds is the active time from which the rider may be asked again.
	DueSeconds int64 `json:"dueSeconds"`
	// ShownAt is when they were last asked, in Unix milliseconds.
	ShownAt  int64 `json:"shownAt"`
	Declined int   `json:"declined"`
}

func (d *UserData) DonateState() (DonateState, error) {
	var s DonateState
	data, err := os.ReadFile(filepath.Join(d.dir, "donate.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

// MergeDonateState keeps the furthest of what is stored and s, field by field, so a browser that
// fell behind never winds another one back.
func (d *UserData) MergeDonateState(s DonateState) (DonateState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	old, err := d.DonateState()
	if err != nil {
		return old, err
	}
	merged := DonateState{
		ActiveSeconds: max(old.ActiveSeconds, s.ActiveSeconds),
		DueSeconds:    max(old.DueSeconds, s.DueSeconds),
		ShownAt:       max(old.ShownAt, s.ShownAt),
		Declined:      max(old.Declined, s.Declined),
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return old, err
	}
	return merged, os.WriteFile(filepath.Join(d.dir, "donate.json"), data, 0o600)
}
