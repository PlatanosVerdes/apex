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
