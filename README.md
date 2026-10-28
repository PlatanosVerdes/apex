# Apex

Plan motorcycle routes on a map and ride them with Google Maps. Pick how each leg is ridden (fast,
curvy, twisty), and Apex finds the bendier roads and the hidden stops that make Google Maps take
them. Share a route as a link, export it as GPX, and keep your places from Google Maps lists on the
map.

Runs on the Raspberry from [rpi-services](https://github.com/PlatanosVerdes/rpi-services). Go on the
server, [Lit](https://lit.dev) and [Leaflet](https://leafletjs.com) on the page, CalVer tag on every
push to main.

## How a route reaches Google Maps

Google Maps opens a route from a link in the path form, one `/lat,lng` per stop:

```
https://www.google.com/maps/dir/41.60818,2.28605/41.68910,2.48987/.../data=!4m4!4m3!2m1!2b1!3e0
```

The documented `?api=1&waypoints=` form caps stops at 9 (3 on mobile browsers); the path form has
opened 25 in the iPhone app. The `data=` blob carries the avoid options exactly as Google Maps writes
them when its boxes are ticked: `!1b1` motorways, `!2b1` tolls, `!3b1` ferries, `!3e0` driving.

Between two stops Google takes its own way. For a curvy leg Apex adds hidden stops wherever its route
strays more than 200 m from the fast route under the same settings, which stands in for Google's.
Stops go in where the route meets the road, not where the map was clicked: Google snaps a raw point
to its own nearest road, which can be a track beside the one the route takes.

## Where the data comes from

| What | From |
|---|---|
| Routes and curve search | [Valhalla](https://github.com/valhalla/valhalla), FOSSGIS's public server, motorcycle costing |
| Place names | [Nominatim](https://nominatim.org) (OpenStreetMap), one request a second |
| Place search | [Photon](https://photon.komoot.io) (Komoot, OpenStreetMap) |
| Map tiles | Esri World Street Map and World Imagery |
| Google Maps lists | The endpoint Google Maps calls to show a shared list |

The Google Maps list endpoint is **not a published API**: it was read from the Maps page and can
change without notice. A list that fails to refresh keeps its last copy.

### Curvy and twisty legs

Valhalla has no "curvy" option, so Apex looks for one. It takes the fast way, closes off its
straightest 1.5 km stretches with `exclude_polygons` and asks again, three rounds, then keeps the
bendiest result within 1.5× (curvy) or 2.5× (twisty) of the fast time. Bendiness is degrees turned
per kilometre, measured on the path resampled every 30 m; turns over 120° count as junctions.

A curve search costs the public server five to ten requests where a fast leg costs one, so each
rider gets 20 that come back one every 30 seconds. Legs already worked out come from memory and cost
nothing. A leg asked for without a search left is drawn fast without motorways, marked, and redone
by itself once one is back.

## Accounts

Anyone can plan, share and export without an account. Saving routes and linking Google Maps lists
needs one: Sign in with Google (OpenID Connect, authorisation code with PKCE). The session is a
cookie signed with HMAC-SHA256, `HttpOnly`, `Secure` over HTTPS, `SameSite=Lax`, 30 days; the key is
`session.key` in the data folder, and replacing it signs everyone out.

Each rider has a folder named after a hash of their Google subject, holding at most 200 routes and
10 lists. Changes from another site's page are refused.

## Metrics

`/metrics` on its own address (`APEX_METRICS_ADDR`), never the public one: requests by route and
answer class, routes worked out by mode, curve searches spent and refused, sign-ins (new and
returning accounts), riders in the last hour and day, rate-limited requests, and the time and outcome
of every call to Valhalla, Photon, Nominatim and Google, plus accounts, saved routes and linked
lists. Riders are counted, never named.

## Running it

```
go run ./cmd/apex
```

| Variable | Default | |
|---|---|---|
| `APEX_PORT` | `8000` | |
| `APEX_DATA_DIR` | `./data` | riders' folders and the session key |
| `APEX_PUBLIC_URL` | | the address riders reach, `https://…`; sign-in callbacks go to `<url>/auth/callback` |
| `APEX_GOOGLE_CLIENT_ID`, `APEX_GOOGLE_CLIENT_SECRET` | | a Google OAuth web client; without them there is no sign-in |
| `APEX_CLIENT_IP_HEADER` | | the header a proxy puts the visitor's address in, `CF-Connecting-IP` behind Cloudflare |
| `APEX_OWNER_EMAIL` | | takes over routes and lists saved before accounts existed |
| `APEX_DEV_LOGIN` | | `1` lets this machine sign in as test riders A and B, for trying accounts locally |
| `APEX_CURVE_SEARCHES`, `APEX_CURVE_REFILL` | `20`, `30s` | each rider's curve-search budget |
| `APEX_PUBLIC_REQUESTS`, `APEX_PUBLIC_REFILL` | `120`, `500ms` | each address's budget on the open endpoints |
| `APEX_LISTS_EVERY` | `3h` | how often linked lists are read again |
| `APEX_DONATE` | | the invitation to chip in, e.g. `url=https://ko-fi.com/… first=1h later=2h most=32h thanks=10h gap=24h rounds=5 rest=96h stops=10`; empty never asks. Waits are active time (page in view, someone using it): `first` before the first ask, `later` after a "Not now" (doubled each time up to `most`), `thanks` after following the link; `gap` is the least wall-clock time between asks; after `rounds` (5) "Not now" in a row the rider rests `rest` (96h, wall clock) and the waits start over; `stops` makes reaching that many stops a moment to ask |
| `APEX_METRICS_ADDR` | | where `/metrics` is served for Prometheus, e.g. `:9100`; never the public address |
| `APEX_VALHALLA_URL`, `APEX_NOMINATIM_URL`, `APEX_PHOTON_URL` | public servers | |
