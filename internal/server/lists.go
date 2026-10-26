package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/PlatanosVerdes/apex/internal/gmaps"
	"github.com/PlatanosVerdes/apex/internal/store"
)

type ListSource interface {
	Resolve(ctx context.Context, link string) (string, error)
	Fetch(ctx context.Context, id string) (gmaps.List, error)
}

func (s *Server) listLists(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	all, err := d.Lists.All()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, all)
}

// addList links the Google Maps list behind {"url": "..."}; adding one already linked refreshes it.
func (s *Server) addList(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, err := s.ListSource.Resolve(r.Context(), body.URL)
	if errors.Is(err, gmaps.ErrNotAList) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "could not open the link: "+err.Error(), http.StatusBadGateway)
		return
	}
	l, err := s.fetchList(r.Context(), d, id, body.URL)
	if errors.Is(err, store.ErrLimit) {
		s.fail(w, err)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, l)
}

func (s *Server) refreshList(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	all, err := d.Lists.All()
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, l := range all {
		if l.ID == r.PathValue("id") {
			fresh, err := s.fetchList(r.Context(), d, l.ID, l.URL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			writeJSON(w, fresh)
			return
		}
	}
	s.fail(w, store.ErrNotFound)
}

// setListColour takes {"colour": "teal"}, one of store.Colours.
func (s *Server) setListColour(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	var body struct {
		Colour string `json:"colour"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	l, err := d.Lists.SetColour(r.PathValue("id"), body.Colour)
	if errors.Is(err, store.ErrNotFound) {
		s.fail(w, err)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, l)
}

func (s *Server) deleteList(w http.ResponseWriter, r *http.Request, d *store.UserData) {
	if err := d.Lists.Delete(r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) fetchList(ctx context.Context, d *store.UserData, id, link string) (store.LinkedList, error) {
	gl, err := s.ListSource.Fetch(ctx, id)
	if err != nil {
		return store.LinkedList{}, err
	}
	return d.Lists.Put(store.LinkedList{List: gl, URL: link, Fetched: time.Now().UTC()})
}

// RefreshLists re-reads every rider's linked lists now and then every interval, so places added in
// Google Maps show up without pasting the link again. A list that fails keeps its last copy.
func (s *Server) RefreshLists(ctx context.Context, interval time.Duration) {
	for {
		err := s.Users.Each(func(d *store.UserData) {
			all, err := d.Lists.All()
			if err != nil {
				s.Log.Warn("lists", "err", err)
				return
			}
			for _, l := range all {
				if time.Since(l.Fetched) < interval/2 || ctx.Err() != nil {
					continue
				}
				if _, err := s.fetchList(ctx, d, l.ID, l.URL); err != nil {
					s.Log.Warn("refresh list", "list", l.ID, "err", err)
				}
			}
		})
		if err != nil {
			s.Log.Warn("lists", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
