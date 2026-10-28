package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PlatanosVerdes/apex/internal/auth"
	"github.com/PlatanosVerdes/apex/internal/gmaps"
	"github.com/PlatanosVerdes/apex/internal/limit"
	"github.com/PlatanosVerdes/apex/internal/metrics"
	"github.com/PlatanosVerdes/apex/internal/nominatim"
	"github.com/PlatanosVerdes/apex/internal/photon"
	"github.com/PlatanosVerdes/apex/internal/plan"
	"github.com/PlatanosVerdes/apex/internal/store"
	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

type fakePlanner struct {
	got   []valhalla.Point
	legs  []plan.LegOptions
	avoid valhalla.Avoid
}

func (f *fakePlanner) Plan(_ context.Context, pts []valhalla.Point, legs []plan.LegOptions, avoid valhalla.Avoid, allow func() bool) (plan.Plan, error) {
	f.got, f.legs, f.avoid = pts, legs, avoid
	for _, l := range legs {
		if l.Style != plan.Fast && allow != nil {
			allow()
		}
	}
	return plan.Plan{Meters: 1000, Legs: make([]plan.Leg, len(pts)-1)}, nil
}

type fakeSearch struct{}

func (fakeSearch) Search(_ context.Context, q string, near *[2]float64) ([]photon.Result, error) {
	if near == nil {
		return nil, nil
	}
	return []photon.Result{{Name: q, Lat: near[0], Lng: near[1]}}, nil
}

type fakePlaces struct{}

func (fakePlaces) Name(context.Context, float64, float64) (nominatim.Label, error) {
	return nominatim.Label{Place: "Sóller", Road: "Ma-10"}, nil
}

type fakeSource struct{}

func (fakeSource) Resolve(_ context.Context, link string) (string, error) {
	if link != "https://maps.app.goo.gl/list" {
		return "", gmaps.ErrNotAList
	}
	return "LIST_ID", nil
}

func (fakeSource) ResolveRoute(_ context.Context, link string) (string, error) {
	if link == "https://maps.app.goo.gl/route" {
		return "https://www.google.com/maps/dir/Mi+ubicaci%C3%B3n/Viladrau/41.80,2.34/data=!4m6!4m5!2m3!1b1!2b0!3b1!3e0", nil
	}
	return link, nil
}

func (fakeSource) Fetch(context.Context, string) (gmaps.List, error) {
	return gmaps.List{ID: "LIST_ID", Name: "Ruta moto", Places: []gmaps.Place{{Name: "Pedraforca", Lat: 42.24, Lng: 1.71}}}, nil
}

func newServer(t *testing.T) (*httptest.Server, *fakePlanner) {
	srv, planner, _ := newServerWith(t, nil)
	return srv, planner
}

func newServerWith(t *testing.T, tune func(*Server)) (*httptest.Server, *fakePlanner, *Server) {
	users, err := store.OpenUsers(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sessions, _ := auth.OpenSessions(t.TempDir(), false)
	planner := &fakePlanner{}
	s := &Server{Search: fakeSearch{}, Users: users, Sessions: sessions, DevLogin: true, ListSource: fakeSource{}, RouteLinks: fakeSource{},
		Planner: planner, Places: fakePlaces{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if tune != nil {
		tune(s)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv, planner, s
}

// rider is a browser signed in as a made-up rider through the development sign-in.
func rider(t *testing.T, srv *httptest.Server, as string) *http.Client {
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	resp, err := c.Get(srv.URL + "/auth/dev?as=" + as)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("sign in as %s: %v %v", as, err, resp)
	}
	return c
}

func do(t *testing.T, c *http.Client, method, url, body string) *http.Response {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPlan(t *testing.T) {
	srv, planner := newServer(t)
	post := func(body string) *http.Response {
		return do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/plan", body)
	}

	resp := post(`{"points":[[39.57,2.65],[39.7667,2.715]],"avoid":{"highways":true,"ferries":true}}`)
	var p plan.Plan
	_ = json.NewDecoder(resp.Body).Decode(&p)
	if resp.StatusCode != 200 || len(p.Legs) != 1 || planner.got[1] != (valhalla.Point{Lat: 39.7667, Lng: 2.715}) ||
		planner.avoid != (valhalla.Avoid{Highways: true, Ferries: true}) || planner.legs != nil {
		t.Fatalf("%d %+v %+v %+v %+v", resp.StatusCode, p, planner.got, planner.avoid, planner.legs)
	}

	resp = post(`{"points":[[39.57,2.65],[39.7667,2.715]],"legs":[{"style":"twisty","avoidHighways":true}]}`)
	if resp.StatusCode != 200 || len(planner.legs) != 1 || planner.legs[0].Style != plan.Twisty {
		t.Fatalf("advanced: %d %+v", resp.StatusCode, planner.legs)
	}

	for _, body := range []string{
		`{"points":[[39.57,2.65]]}`,
		`{"points":[[39.57,2.65],[95,2]]}`,
		`{"points":[[39.57,2.65],[39.7,2.7]],"legs":[]}`,
		`{"points":[[39.57,2.65],[39.7,2.7]],"legs":[{"style":"scenic"}]}`,
	} {
		if resp := post(body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", body, resp.StatusCode)
		}
	}
}

func TestRoutesAPI(t *testing.T) {
	srv, _ := newServer(t)
	a := rider(t, srv, "a")
	resp := do(t, a, http.MethodPost, srv.URL+"/api/routes", `{"name":"Tramuntana","stops":[{"lat":39.57,"lng":2.65,"name":"Palma"}],"roundTrip":true}`)
	var saved store.Route
	_ = json.NewDecoder(resp.Body).Decode(&saved)
	if saved.ID == "" || !saved.RoundTrip {
		t.Fatalf("%d %+v", resp.StatusCode, saved)
	}
	if resp := do(t, a, http.MethodPut, srv.URL+"/api/routes/"+saved.ID, `{"name":"Renamed"}`); resp.StatusCode != 200 {
		t.Fatalf("put: %d", resp.StatusCode)
	}
	if resp := do(t, a, http.MethodPut, srv.URL+"/api/routes/aaaaaaaaaaaaaaaa", `{"name":"x"}`); resp.StatusCode != http.StatusNotFound {
		t.Errorf("put to an unknown id must not create it: %d", resp.StatusCode)
	}
	resp = do(t, a, http.MethodGet, srv.URL+"/api/routes", "")
	var list []store.Route
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 || list[0].Name != "Renamed" {
		t.Fatalf("%+v", list)
	}
}

func TestRidersOnlyReachTheirOwn(t *testing.T) {
	srv, _ := newServer(t)
	a, b := rider(t, srv, "a"), rider(t, srv, "b")
	resp := do(t, a, http.MethodPost, srv.URL+"/api/routes", `{"name":"Mine"}`)
	var saved store.Route
	_ = json.NewDecoder(resp.Body).Decode(&saved)

	resp = do(t, b, http.MethodGet, srv.URL+"/api/routes", "")
	var list []store.Route
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Errorf("rider b sees rider a's routes: %+v", list)
	}
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if resp := do(t, b, m, srv.URL+"/api/routes/"+saved.ID, `{"name":"stolen"}`); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s on another rider's route: %d", m, resp.StatusCode)
		}
	}
	for _, path := range []string{"/api/routes", "/api/lists"} {
		if resp := do(t, http.DefaultClient, http.MethodGet, srv.URL+path, ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s signed out: %d", path, resp.StatusCode)
		}
	}
	if resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/routes", `{"name":"x"}`); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("saving signed out: %d", resp.StatusCode)
	}
}

func TestMeAndLogout(t *testing.T) {
	srv, _ := newServer(t)
	var me struct {
		User   *auth.User
		SignIn string
	}
	resp := do(t, http.DefaultClient, http.MethodGet, srv.URL+"/api/me", "")
	_ = json.NewDecoder(resp.Body).Decode(&me)
	if me.User != nil || me.SignIn != "dev" {
		t.Errorf("signed out: %+v", me)
	}
	a := rider(t, srv, "a")
	resp = do(t, a, http.MethodGet, srv.URL+"/api/me", "")
	_ = json.NewDecoder(resp.Body).Decode(&me)
	if me.User == nil || me.User.Email != "a@dev.local" {
		t.Fatalf("signed in: %+v", me)
	}
	do(t, a, http.MethodPost, srv.URL+"/auth/logout", "")
	if resp := do(t, a, http.MethodGet, srv.URL+"/api/routes", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("after logging out: %d", resp.StatusCode)
	}
}

func TestDevLoginIsOffUnlessAsked(t *testing.T) {
	srv, _, _ := newServerWith(t, func(s *Server) { s.DevLogin = false })
	if resp := do(t, http.DefaultClient, http.MethodGet, srv.URL+"/auth/dev?as=a", ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("dev sign-in answered while off: %d", resp.StatusCode)
	}
	if resp := do(t, http.DefaultClient, http.MethodGet, srv.URL+"/auth/login", ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("Google sign-in answered without being set up: %d", resp.StatusCode)
	}
}

func TestCrossSiteChangesRefused(t *testing.T) {
	srv, _ := newServer(t)
	a := rider(t, srv, "a")
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/routes", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Origin", "https://evil.example")
	resp, _ := a.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a post from another site: %d", resp.StatusCode)
	}
}

func TestAccountLimitAnswersConflict(t *testing.T) {
	srv, _, s := newServerWith(t, nil)
	a := rider(t, srv, "a")
	d, _ := s.Users.For(auth.IDFor("dev", "a"))
	d.Routes.MaxRoutes = 1
	do(t, a, http.MethodPost, srv.URL+"/api/routes", `{"name":"1"}`)
	if resp := do(t, a, http.MethodPost, srv.URL+"/api/routes", `{"name":"2"}`); resp.StatusCode != http.StatusConflict {
		t.Errorf("over the cap: %d", resp.StatusCode)
	}
}

func TestPublicBudgetPerAddress(t *testing.T) {
	srv, _, _ := newServerWith(t, func(s *Server) { s.Public = limit.New(2, time.Minute) })
	codes := []int{}
	for range 3 {
		codes = append(codes, do(t, http.DefaultClient, http.MethodGet, srv.URL+"/api/search?q=Sa+Calobra&near=39.8,2.8", "").StatusCode)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Errorf("%v", codes)
	}
}

func TestListsAPI(t *testing.T) {
	srv, _ := newServer(t)
	a := rider(t, srv, "a")
	if resp := do(t, a, http.MethodPost, srv.URL+"/api/lists", `{"url":"https://example.com/x"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a link that is not a list: %d", resp.StatusCode)
	}
	for range 2 {
		if resp := do(t, a, http.MethodPost, srv.URL+"/api/lists", `{"url":"https://maps.app.goo.gl/list"}`); resp.StatusCode != 200 {
			t.Fatalf("add: %d", resp.StatusCode)
		}
	}
	resp := do(t, a, http.MethodGet, srv.URL+"/api/lists", "")
	var all []store.LinkedList
	_ = json.NewDecoder(resp.Body).Decode(&all)
	if len(all) != 1 || all[0].Name != "Ruta moto" || all[0].URL != "https://maps.app.goo.gl/list" || len(all[0].Places) != 1 {
		t.Fatalf("adding the same list twice must keep one: %+v", all)
	}
	if resp := do(t, a, http.MethodPatch, srv.URL+"/api/lists/LIST_ID", `{"colour":"green"}`); resp.StatusCode != 200 {
		t.Errorf("colour: %d", resp.StatusCode)
	}
	if resp := do(t, a, http.MethodPatch, srv.URL+"/api/lists/LIST_ID", `{"colour":"url(evil)"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown colour: %d", resp.StatusCode)
	}
	b := rider(t, srv, "b")
	resp = do(t, b, http.MethodGet, srv.URL+"/api/lists", "")
	_ = json.NewDecoder(resp.Body).Decode(&all)
	if len(all) != 0 {
		t.Errorf("rider b sees rider a's lists: %+v", all)
	}
	if resp := do(t, a, http.MethodDelete, srv.URL+"/api/lists/LIST_ID", ""); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: %d", resp.StatusCode)
	}
}

func TestReadListSignedOut(t *testing.T) {
	srv, _ := newServer(t)
	resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/lists/read", `{"url":"https://maps.app.goo.gl/list"}`)
	var l store.LinkedList
	_ = json.NewDecoder(resp.Body).Decode(&l)
	if resp.StatusCode != 200 || l.Name != "Ruta moto" || l.URL != "https://maps.app.goo.gl/list" || len(l.Places) != 1 {
		t.Fatalf("%d %+v", resp.StatusCode, l)
	}
	if resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/lists/read", `{"url":"https://example.com/x"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a link that is not a list: %d", resp.StatusCode)
	}
}

func TestSearchAPI(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := http.Get(srv.URL + "/api/search?q=Sa%20Calobra&near=39.8,2.8")
	var got []photon.Result
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if resp.StatusCode != 200 || len(got) != 1 || got[0].Name != "Sa Calobra" || got[0].Lat != 39.8 {
		t.Fatalf("%d %+v", resp.StatusCode, got)
	}
	if resp, _ := http.Get(srv.URL + "/api/search?q=a"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("one letter: %d", resp.StatusCode)
	}
}

func TestRouteLinkServesThePage(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := http.Get(srv.URL + "/r/41.60790,2.28760/41.68920,2.49000?n=Montseny&l=ft")
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "<base href=\"/\">") {
		t.Errorf("a route link must serve the page with its base: %d", resp.StatusCode)
	}
}

func TestCurveBudgetPerRider(t *testing.T) {
	srv, _, _ := newServerWith(t, func(s *Server) { s.Curves = limit.New(2, time.Minute) })
	ask := func(c *http.Client) curves {
		resp := do(t, c, http.MethodPost, srv.URL+"/api/plan", `{"points":[[39.57,2.65],[39.7,2.7]],"legs":[{"style":"curvy"}]}`)
		var out struct{ Curves curves }
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out.Curves
	}
	a, b := rider(t, srv, "a"), rider(t, srv, "b")
	if c := ask(a); c.Left != 1 || c.Max != 2 || c.NextIn != 60 {
		t.Errorf("after one search: %+v", c)
	}
	ask(a)
	if c := ask(a); c.Left != 0 {
		t.Errorf("spent: %+v", c)
	}
	if c := ask(b); c.Left != 1 {
		t.Errorf("another rider has their own budget: %+v", c)
	}
}

func TestHTTPSOnlyBehindAProxy(t *testing.T) {
	srv, _, _ := newServerWith(t, func(s *Server) { s.Sessions.Secure = true })
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/r/41.6,2.2?n=x", nil)
	req.Host = "apex.example"
	req.Header.Set("X-Forwarded-Proto", "http")
	resp, _ := c.Do(req)
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "https://apex.example/r/41.6,2.2?n=x" {
		t.Errorf("plain http: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 || resp.Header.Get("Strict-Transport-Security") == "" ||
		!strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Errorf("https: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestMetricsCountWithoutNaming(t *testing.T) {
	reg := metrics.New()
	srv, _, s := newServerWith(t, func(s *Server) { s.Metrics = reg; s.Curves = limit.New(1, time.Minute) })
	s.Gauges()
	a := rider(t, srv, "a")
	do(t, a, http.MethodPost, srv.URL+"/api/routes", `{"name":"1"}`)
	for range 2 {
		do(t, a, http.MethodPost, srv.URL+"/api/plan", `{"points":[[39.57,2.65],[39.7,2.7]],"legs":[{"style":"curvy"}]}`)
	}
	do(t, http.DefaultClient, http.MethodGet, srv.URL+"/api/routes", "")
	do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/donate", `{"event":"clicked"}`)
	if resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/donate", `{"event":"anything"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown donate event: %d", resp.StatusCode)
	}
	var b strings.Builder
	reg.Write(&b)
	out := b.String()
	for _, want := range []string{
		`apex_plans_total{mode="advanced",outcome="ok"} 2`,
		`apex_curve_searches_total{result="spent"} 1`,
		`apex_curve_searches_total{result="refused"} 1`,
		`apex_http_requests_total{route="/api/routes",method="GET",code="4xx"} 1`,
		"apex_accounts 1",
		"apex_saved_routes 1",
		`apex_riders{window="24h",kind="account"} 1`,
		`apex_donate_events_total{event="clicked"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "dev.local") || strings.Contains(out, "127.0.0.1") {
		t.Error("metrics must not name riders")
	}
}

func TestAssetsCompressedAndCached(t *testing.T) {
	srv, _ := newServer(t)
	get := func(path string, headers ...string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	lib := get("/vendor/leaflet-1.9.4.js", "Accept-Encoding", "gzip")
	if lib.Header.Get("Content-Encoding") != "gzip" || !strings.Contains(lib.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("library: %v", lib.Header)
	}
	page := get("/", "Accept-Encoding", "gzip")
	etag := page.Header.Get("ETag")
	if page.StatusCode != 200 || etag == "" || page.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("page: %d %v", page.StatusCode, page.Header)
	}
	if again := get("/", "If-None-Match", etag); again.StatusCode != http.StatusNotModified {
		t.Errorf("an unchanged page must answer 304, got %d", again.StatusCode)
	}
	if missing := get("/nothing.js"); missing.StatusCode != http.StatusNotFound || missing.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("missing file: %d", missing.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/plan", strings.NewReader(`{"points":[[39.57,2.65],[39.7,2.7]]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	plan, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(plan.Body)
	if err != nil || plan.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("plan not gzipped: %v %v", err, plan.Header)
	}
	var out map[string]any
	if err := json.NewDecoder(z).Decode(&out); err != nil {
		t.Errorf("gzipped plan does not decode: %v", err)
	}
}

func TestParseDonate(t *testing.T) {
	d, err := ParseDonate("url=https://ko-fi.com/x, first=30m later=1h stops=0 rounds=3")
	if err != nil || d.URL != "https://ko-fi.com/x" || d.FirstSeconds != 1800 || d.LaterSeconds != 3600 || d.AtStops != 0 ||
		d.GapSeconds != 24*3600 || d.Rounds != 3 || d.RestSeconds != 60*24*3600 {
		t.Fatalf("%+v %v", d, err)
	}
	if d, err := ParseDonate(""); d != nil || err != nil {
		t.Errorf("empty must mean no invitation: %+v %v", d, err)
	}
	for _, bad := range []string{"first=1h", "url=http://x", "url=https://x first=soon", "url=https://x colour=red", "url=https://x stops", "url=https://x rounds=0"} {
		if _, err := ParseDonate(bad); err == nil {
			t.Errorf("%q must fail", bad)
		}
	}
}

func TestDonateStateFollowsTheRider(t *testing.T) {
	srv, _ := newServer(t)
	if resp := do(t, http.DefaultClient, http.MethodPut, srv.URL+"/api/donate/state", `{"activeSeconds":60}`); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", resp.StatusCode)
	}
	a := rider(t, srv, "a")
	do(t, a, http.MethodPut, srv.URL+"/api/donate/state", `{"activeSeconds":600,"dueSeconds":3600,"shownAt":5,"declined":1}`)
	// A browser that fell behind must not wind the count back.
	resp := do(t, a, http.MethodPut, srv.URL+"/api/donate/state", `{"activeSeconds":120}`)
	var got store.DonateState
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got != (store.DonateState{ActiveSeconds: 600, DueSeconds: 3600, ShownAt: 5, Declined: 1}) {
		t.Fatalf("%+v", got)
	}
	if resp := do(t, a, http.MethodPut, srv.URL+"/api/donate/state", `{"activeSeconds":-1}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("negative time: %d", resp.StatusCode)
	}
	b := rider(t, srv, "b")
	resp = do(t, b, http.MethodGet, srv.URL+"/api/donate/state", "")
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got != (store.DonateState{}) {
		t.Errorf("rider b sees rider a's state: %+v", got)
	}
}

func TestImportRoute(t *testing.T) {
	srv, _ := newServer(t)
	resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/import", `{"url":"https://maps.app.goo.gl/route"}`)
	var got struct {
		Stops    []importedStop
		FromHere bool
		Avoid    map[string]bool
	}
	_ = json.NewDecoder(resp.Body).Decode(&got)
	// Viladrau is only named in the link: it is searched for near the pinned stop and flagged.
	if resp.StatusCode != 200 || !got.FromHere || len(got.Stops) != 2 || !got.Stops[0].Approx || got.Stops[0].Name != "Viladrau" ||
		got.Stops[1].Approx || got.Stops[1].Lat != 41.80 || !got.Avoid["highways"] || got.Avoid["tolls"] || !got.Avoid["ferries"] {
		t.Fatalf("%d %+v", resp.StatusCode, got)
	}
	if resp := do(t, http.DefaultClient, http.MethodPost, srv.URL+"/api/import", `{"url":"https://www.google.com/maps/place/Viladrau"}`); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a place is not a route: %d", resp.StatusCode)
	}
}
