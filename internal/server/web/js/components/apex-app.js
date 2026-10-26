import { LitElement, html, nothing, svg } from '../../vendor/lit-3.3.3.js';
import { BASE_PREVIEWS, RouteMap } from '../map.js';
import { googleMapsLink, googleStopCount, routePoints } from '../link.js';
import { coords, curves, duration, km, shortDuration } from '../format.js';
import { fromGPX, toGPX } from '../gpx.js';
import { fromRouteLink, toRouteLink } from '../routelink.js';

const DRAFT_KEY = 'apex.draft';
const HIDDEN_KEY = 'apex.hiddenLists';
const COLOURS = ['teal', 'blue', 'green', 'amber', 'red', 'slate', 'cyan', 'brown'];
// The most "paradas" seen opening in the Google Maps app from one link.
const TESTED_GOOGLE_STOPS = 23;
const AVOIDS = [['highways', 'Motorways'], ['tolls', 'Tolls'], ['ferries', 'Ferries']];
const STYLES = [['fast', 'Fast'], ['curvy', 'Curvy'], ['twisty', 'Twisty']];
// A ring centred on 12,12 with its arrowhead in the gap; the ↩ character sits off-centre in most fonts.
const RETURN_ICON = svg`<svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M19 12a7 7 0 1 1-2.05-4.95"/><path d="M17 3.5V7h-3.5"/></svg>`;
const NO_MOTORWAY_ICON = svg`<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M8 3 5 21M16 3l3 18M12 4v3M12 11v3M12 18v2"/><path d="M3 3l18 18"/></svg>`;
const SAVE_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" aria-hidden="true"><path d="M6 3h12v18l-6-4-6 4V3Z"/></svg>`;
const CHECK_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m5 12 5 5 9-10"/></svg>`;
const SEARCH_ICON = svg`<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>`;
const PIN_ICON = svg`<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" aria-hidden="true"><path d="M12 21s-7-6.2-7-11.5A7 7 0 0 1 19 9.5C19 14.8 12 21 12 21Z"/><circle cx="12" cy="9.5" r="2.5"/></svg>`;
const GEAR_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/></svg>`;
const PLACES_ICON = svg`<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" aria-hidden="true"><path d="M12 21s-7-6.2-7-11.5A7 7 0 0 1 19 9.5C19 14.8 12 21 12 21Z"/><path d="m12 6.5 1 2.1 2.3.3-1.7 1.6.4 2.3-2-1.1-2 1.1.4-2.3-1.7-1.6 2.3-.3 1-2.1Z"/></svg>`;
const BASE_LABELS = { Roads: 'Map', Satellite: 'Satellite' };
// Colours a leg kind can take, each with its light and dark shade; neon (blue, lime, magenta) is the default.
const LEG_COLOURS = {
  blue: ['#1a73e8', '#5b9bff'], cyan: ['#0aa5c9', '#38d6ff'], green: ['#16a34a', '#4ade80'], lime: ['#6db800', '#a8f000'],
  yellow: ['#e0a800', '#ffd02e'], orange: ['#ff6a00', '#ff9a3d'], red: ['#e5262a', '#ff5a5a'], magenta: ['#d6249f', '#ff4fc3'],
  violet: ['#8b3dff', '#b18cff'], grey: ['#6b7079', '#b5b9c0'],
};
const DEFAULT_LEG_COLOURS = { fast: 'blue', curvy: 'lime', twisty: 'magenta' };
const LEG_VARS = { fast: '--route', curvy: '--curvy', twisty: '--twisty' };
const LEG_COLOURS_KEY = 'apex.legColours';
const darkScheme = window.matchMedia('(prefers-color-scheme: dark)');
const SHARE_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3v13"/><path d="m7 8 5-5 5 5"/><path d="M5 13v6a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-6"/></svg>`;

export class ApexApp extends LitElement {
  static properties = {
    route: { state: true },
    plan: { state: true },
    planning: { state: true },
    selected: { state: true },
    dirty: { state: true },
    saved: { state: true },
    notice: { state: true },
    collapsed: { state: true },
    here: { state: true },
    lists: { state: true },
    hiddenLists: { state: true },
    listBusy: { state: true },
    listError: { state: true },
    popover: { state: true },
    pickingColour: { state: true },
    colourEdit: { state: true },
    query: { state: true },
    curves: { state: true },
    account: { state: true },
    insertAt: { state: true },
    results: { state: true },
    active: { state: true },
  };

  constructor(api) {
    super();
    this.api = api;
    const { dirty = false, ...route } = loadDraft() ?? blankRoute();
    this.route = route;
    this.plan = null;
    this.planKey = '';
    this.planning = false;
    this.selected = -1;
    this.dirty = dirty;
    this.saved = [];
    this.notice = null;
    this.collapsed = false;
    this.here = null;
    this.lists = [];
    this.hiddenLists = new Set(readJSON(HIDDEN_KEY) ?? []);
    this.listBusy = false;
    this.popover = null; // 'options' or 'layers' while that popover is showing
    this.pickingColour = null;
    this.colourEdit = null; // the leg kind whose colour strip is open in a leg menu
    this.query = '';
    this.curves = null; // { left, max, nextIn } curve searches this user has, from the last plan
    this.account = { user: null, signIn: '' }; // who is signed in, and how one can sign in here
    this.insertAt = null; // where a found place goes; null lets it find its best gap
    this.results = [];
    this.active = 0;
    this.onOutside = e => {
      if (this.popover && !e.target.closest('.popover, .popover-toggle')) this.popover = null;
      if (this.results.length && !e.target.closest('.search')) this.results = [];
    };
    this.onKey = e => {
      if (e.key === 'Escape') this.popover = null;
      if (this.selected < 0 || e.target.closest('input, select')) return;
      if (e.key === 'Delete' || e.key === 'Backspace') this.removeStop(this.selected);
      if (e.altKey && e.key === 'ArrowUp' && this.selected > 0) this.shiftStop(this.selected, -1);
      if (e.altKey && e.key === 'ArrowDown' && this.selected < this.route.stops.length - 1) this.shiftStop(this.selected, 1);
    };
  }

  // Leaflet's stylesheet is global, so the map cannot live in a shadow root.
  createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('keydown', this.onKey);
    document.addEventListener('pointerdown', this.onOutside);
  }

  disconnectedCallback() {
    document.removeEventListener('keydown', this.onKey);
    document.removeEventListener('pointerdown', this.onOutside);
    super.disconnectedCallback();
  }

  firstUpdated() {
    this.map = new RouteMap(this.querySelector('.map'), {
      add: p => this.addStop(p, this.route.stops.length),
      insert: (leg, p) => this.addStop(p, leg + 1 - this.offset),
      move: (i, p) => this.relocateStop(i, p),
      select: i => this.select(i),
      neighbours: i => this.neighbours(i),
      preview: (points, signal) => this.api.plan({ points: points.map(p => [p.lat, p.lng]), avoid: this.route.avoid ?? {}, legs: null }, signal),
    });
    this.changed({ fit: true, edited: false });
    this.requestUpdate(); // The layers panel renders once the map exists.
    this.loadAccount();
    const linked = fromRouteLink(location);
    if (linked) this.openLinked(linked);
    if (this.route.fromHere) this.useMyLocation();
    this.nameUnnamed();
    this.sortable = window.Sortable.create(this.querySelector('.stops'), {
      handle: '.grip',
      draggable: 'li.item',
      animation: 150,
      ghostClass: 'ghost',
      forceFallback: true, // The same follow-the-finger drag with mouse and touch, no browser drag image.
      onStart: e => (this.dragHome = e.item.nextSibling),
      // Sortable moved the element; put it back where Lit left it and let the re-render move the stop.
      onEnd: e => {
        e.from.insertBefore(e.item, this.dragHome);
        if (e.oldDraggableIndex !== e.newDraggableIndex) this.moveStop(e.oldDraggableIndex, e.newDraggableIndex);
      },
    });
  }

  // Stops saved before names were split into place and road have no detail field yet.
  nameUnnamed() {
    for (const s of this.route.stops) if (!s.name || s.detail === undefined) this.nameStop(s);
  }

  // 1 while the line starts at the rider's position, which then comes before stop 1.
  get offset() {
    return this.route.fromHere && this.here ? 1 : 0;
  }

  get advanced() {
    return this.route.mode === 'advanced';
  }

  defaultLeg() {
    return { style: 'fast', avoidHighways: !!this.route.avoid?.highways };
  }

  legOf(stop) {
    return { ...this.defaultLeg(), ...stop?.leg };
  }

  // legAt is the leg leaving stop i; -1 is the leg from the rider's position to stop 1.
  legAt(i) {
    return i < 0 ? { ...this.defaultLeg(), ...this.route.hereLeg } : this.legOf(this.route.stops[i]);
  }

  planRequest() {
    const stops = routePoints(this.route);
    const points = [...(this.offset ? [this.here] : []), ...stops].map(s => [s.lat, s.lng]);
    const legs = this.advanced
      ? [...(this.offset ? [this.legAt(-1)] : []), ...stops.slice(0, -1).map(s => this.legOf(s))]
      : null;
    return { points, avoid: this.route.avoid ?? {}, legs };
  }

  changed({ fit = false, edited = true } = {}) {
    if (edited) this.dirty = true;
    try {
      localStorage.setItem(DRAFT_KEY, JSON.stringify({ ...this.route, dirty: this.dirty }));
    } catch {}
    this.map.setStops(this.route.stops, this.selected);
    if (fit) this.map.fit(this.route.stops, this.querySelector('.panel').getBoundingClientRect());
    clearTimeout(this.routeTimer);
    this.routeTimer = setTimeout(() => this.fetchPlan(), 200);
  }

  async fetchPlan() {
    const request = this.planRequest();
    const key = JSON.stringify(request);
    if (key === this.planKey || (this.planning && key === this.pendingKey)) return;
    this.pending?.abort();
    this.pendingKey = key;
    if (request.points.length < 2) {
      this.plan = null;
      this.planKey = key;
      this.map.setLegs([], []);
      return;
    }
    const pending = (this.pending = new AbortController());
    this.planning = true;
    this.map.setPending(true);
    try {
      this.plan = await this.api.plan(request, pending.signal);
      this.planKey = key;
      this.trackCurves(this.plan);
      this.map.setLegs(this.plan.legs, request.legs?.map(l => l.style) ?? []);
      if (this.notice?.planning) this.notice = null;
    } catch (err) {
      if (err.name !== 'AbortError') this.notice = { text: `Could not draw the route: ${err.message}`, bad: true, planning: true };
    } finally {
      if (this.pending === pending) {
        this.planning = false;
        this.map.setPending(false);
      }
    }
  }

  // trackCurves keeps the curve-search counter current and, when a leg had to be drawn fast for
  // lack of searches, asks again as soon as one comes back.
  trackCurves(plan) {
    if (!plan.curves) return;
    this.curves = { ...plan.curves, at: Date.now() };
    this.map.setQuota(this.quotaLevel());
    clearTimeout(this.curvesRetry);
    if (!plan.legs.some(l => l.limited)) return;
    this.curvesRetry = setTimeout(() => {
      this.planKey = '';
      this.fetchPlan();
    }, (plan.curves.nextIn + 1) * 1000);
  }

  // quotaLevel grades how few curve searches are left: '', 'low', 'very-low' or 'out'.
  quotaLevel() {
    const c = this.curves;
    if (!c) return '';
    if (this.plan?.legs.some(l => l.limited) || c.left === 0) return 'out';
    if (c.left <= c.max * 0.2) return 'very-low';
    return c.left <= c.max / 2 ? 'low' : '';
  }

  renderCurves() {
    const c = this.curves;
    if (!this.advanced || !c) return nothing;
    // Out of searches says nothing here: the beating line and "Fast for now" on the leg do.
    if (c.left >= c.max || this.quotaLevel() === 'out') return nothing;
    const level = this.quotaLevel();
    const text = level === 'very-low' ? `Only ${c.left} curve ${c.left === 1 ? 'search' : 'searches'} left` : `${c.left} of ${c.max} curve searches left`;
    return html`<div class="quota ${level}" title="Searching for curves asks the public routing server many times, so each rider gets a few at a time; they come back by themselves.">${text}</div>`;
  }

  update(changed) {
    if (changed.has('selected') && this.map) this.map.setStops(this.route.stops, this.selected);
    super.update(changed);
  }

  setStops(stops, options) {
    this.route = { ...this.route, stops };
    this.changed(options);
  }

  addStop(p, at) {
    // A stop dropped on a leg splits it, so both halves ride the way the leg did.
    const before = this.route.stops[at - 1];
    const stop = { ...p, name: '', detail: '', ...(before?.leg ? { leg: { ...before.leg } } : {}) };
    const stops = [...this.route.stops];
    stops.splice(at, 0, stop);
    this.selected = at;
    this.setStops(stops);
    this.nameStop(stop);
  }

  relocateStop(i, p) {
    const stop = { ...this.route.stops[i], ...p, name: '', detail: '' };
    this.setStops(this.route.stops.map((s, j) => (j === i ? stop : s)));
    this.nameStop(stop);
  }

  removeStop(i) {
    this.selected = -1;
    this.setStops(this.route.stops.filter((_, j) => j !== i));
  }

  shiftStop(i, by) {
    const stops = [...this.route.stops];
    [stops[i], stops[i + by]] = [stops[i + by], stops[i]];
    this.selected = i + by;
    this.setStops(stops);
  }

  moveStop(from, to) {
    const stops = [...this.route.stops];
    stops.splice(to, 0, ...stops.splice(from, 1));
    this.selected = to;
    this.setStops(stops);
  }

  reverse() {
    this.selected = -1;
    this.setStops([...this.route.stops].reverse());
  }

  setLeg(i, patch) {
    if (i < 0) return this.setOption('hereLeg', { ...this.legAt(-1), ...patch });
    this.setStops(this.route.stops.map((s, j) => (j === i ? { ...s, leg: { ...this.legOf(s), ...patch } } : s)));
  }

  // Fills the name in once it arrives, unless the stop has been moved or removed meanwhile.
  async nameStop(stop) {
    try {
      const { place, road } = await this.api.place(stop);
      if (!this.route.stops.includes(stop)) return;
      const label = { name: place || road, detail: place ? road : '' };
      this.route = { ...this.route, stops: this.route.stops.map(s => (s === stop ? { ...s, ...label } : s)) };
      this.changed({ edited: this.dirty });
    } catch {}
  }

  select(i) {
    this.selected = this.selected === i ? -1 : i;
    if (this.selected >= 0) this.map.focus(this.route.stops[i]);
  }

  setOption(key, value) {
    this.route = { ...this.route, [key]: value };
    this.changed();
  }

  setMode(mode) {
    if (mode === this.route.mode || (mode === 'simple' && !this.advanced)) return;
    this.setOption('mode', mode);
  }

  async useMyLocation() {
    this.notice = { text: 'Waiting for your location…' };
    try {
      const { coords } = await new Promise((resolve, reject) => {
        if (!navigator.geolocation) return reject(new Error('no geolocation'));
        navigator.geolocation.getCurrentPosition(resolve, reject, { timeout: 15000, maximumAge: 60000 });
      });
      this.setHere({ lat: coords.latitude, lng: coords.longitude });
      this.notice = null;
      if (this.route.fromHere) this.changed({ edited: false });
      else this.setOption('fromHere', true);
    } catch {
      this.setHere(null);
      if (this.route.fromHere) this.setOption('fromHere', false);
      this.notice = { text: 'Location not allowed, so the route starts at stop 1.', bad: true };
    }
  }

  setHere(here) {
    this.here = here;
    this.map.setHere(here);
  }

  setAvoid(key, value) {
    this.setOption('avoid', { ...this.route.avoid, [key]: value });
  }

  async loadLists() {
    try {
      this.setLists(await this.api.lists());
    } catch (err) {
      this.notice = { text: `Your Google lists are unavailable: ${err.message}`, bad: true };
    }
  }

  setLists(lists) {
    this.lists = lists;
    this.drawPlaces();
  }

  drawPlaces() {
    this.map.setPlaces(this.lists.filter(l => !this.hiddenLists.has(l.id)), place => this.addPlace(place));
  }

  async setListColour(id, colour) {
    this.pickingColour = null;
    try {
      const updated = await this.api.setListColour(id, colour);
      this.setLists(this.lists.map(l => (l.id === id ? updated : l)));
    } catch (err) {
      this.listError = `Could not change the colour: ${err.message}`;
    }
  }

  // Opens upwards when the pill sits low in the scrolling list, so the menu is not cut off below.
  openLegMenu(name, pill) {
    const list = pill.closest('.body').getBoundingClientRect();
    this.menuUp = list.bottom - pill.getBoundingClientRect().bottom < 240;
    this.togglePopover(name);
  }

  togglePopover(name) {
    this.popover = this.popover === name ? null : name;
    this.colourEdit = null;
  }

  async addList(form) {
    const input = form.elements.url;
    this.listBusy = true;
    this.listError = '';
    try {
      const list = await this.api.addList(input.value);
      this.setLists([...this.lists.filter(l => l.id !== list.id), list]);
      input.value = '';
      this.flash(`Linked "${list.name}" with ${list.places.length} places`);
    } catch (err) {
      this.listError = `Could not link the list: ${err.message}`;
    } finally {
      this.listBusy = false;
    }
  }

  async refreshList(id) {
    this.listBusy = true;
    this.listError = '';
    try {
      const fresh = await this.api.refreshList(id);
      this.setLists(this.lists.map(l => (l.id === id ? fresh : l)));
      this.flash(`"${fresh.name}" is up to date`);
    } catch (err) {
      this.listError = `Could not refresh: ${err.message}`;
    } finally {
      this.listBusy = false;
    }
  }

  async unlinkList(list) {
    if (!confirm(`Stop showing "${list.name}"? The list stays in Google Maps.`)) return;
    await this.api.removeList(list.id);
    this.setLists(this.lists.filter(l => l.id !== list.id));
  }

  toggleList(id, shown) {
    const hidden = new Set(this.hiddenLists);
    if (shown) hidden.delete(id);
    else hidden.add(id);
    this.hiddenLists = hidden;
    writeJSON(HIDDEN_KEY, [...hidden]);
    this.drawPlaces();
  }

  // A place joins the route in the gap where it adds the least distance, as a rider would slot it in.
  // addPlace adds a place as the next stop after the last one, or at the gap a "+" picked.
  addPlace(place, at = null) {
    const stops = this.route.stops;
    const index = at ?? stops.length;
    const before = stops[index - 1];
    const stop = {
      lat: place.lat,
      lng: place.lng,
      name: place.name,
      detail: place.detail ?? (place.address && place.address !== place.name ? place.address : ''),
      ...(before?.leg ? { leg: { ...before.leg } } : {}),
    };
    const next = [...stops];
    next.splice(index, 0, stop);
    this.selected = index;
    this.setStops(next);
    this.flash(`${place.name} added as stop ${index + 1}`);
  }

  // Places from the linked lists match first, as the rider's own; the geocoder fills the rest.
  async searchPlaces(q) {
    this.query = q;
    this.active = 0;
    clearTimeout(this.searchTimer);
    this.searchAbort?.abort();
    const text = fold(q.trim());
    if (text.length < 2) {
      this.results = [];
      return;
    }
    const own = this.lists.flatMap(l => l.places.filter(p => fold(p.name).includes(text)).map(p => ({ ...p, detail: p.address || '', list: l })));
    this.results = own.slice(0, 4);
    this.searchTimer = setTimeout(async () => {
      const abort = (this.searchAbort = new AbortController());
      const stops = this.route.stops;
      const anchor = this.insertAt !== null ? stops[Math.max(this.insertAt - 1, 0)] : stops[stops.length - 1];
      const near = anchor ?? this.map.map.getCenter();
      try {
        const found = await this.api.search(q.trim(), near, abort.signal);
        this.results = [...own.slice(0, 4), ...found].slice(0, 8);
      } catch {}
    }, 300);
  }

  choose(result) {
    if (!result) return;
    this.addPlace(result, this.insertAt);
    this.insertAt = null;
    this.query = '';
    this.results = [];
    this.map.focus(result);
  }

  searchKey(e) {
    if (e.key === 'ArrowDown') this.active = Math.min(this.active + 1, this.results.length - 1);
    else if (e.key === 'ArrowUp') this.active = Math.max(this.active - 1, 0);
    else if (e.key === 'Enter') this.choose(this.results[this.active]);
    else if (e.key === 'Escape') {
      this.results = [];
      this.insertAt = null;
    }
    else return;
    e.preventDefault();
  }


  renderSearch() {
    const stops = this.route.stops;
    const at = this.insertAt;
    const placeholder = at === null
      ? stops.length ? 'Search a place to add' : 'Search where to start'
      : at === 0 ? 'Add before stop 1' : at >= stops.length ? 'Add at the end' : `Add between ${at} and ${at + 1}`;
    return html`
      <div class="search">
        <span class="search-icon">${SEARCH_ICON}</span>
        <input type="search" .value=${this.query} placeholder=${placeholder} aria-label="Search a place"
          autocomplete="off" @input=${e => this.searchPlaces(e.target.value)} @keydown=${e => this.searchKey(e)}>
        ${at !== null ? html`<button class="icon" title="Cancel" @click=${() => (this.insertAt = null)}>✕</button>` : nothing}
        ${this.results.length ? html`
          <ul class="popover results" role="listbox">
            ${this.results.map((r, i) => html`
              <li role="option" aria-selected=${i === this.active} @pointerenter=${() => (this.active = i)}
                @click=${() => this.choose(r)}>
                ${r.list ? this.listBadge(r.list) : html`<span class="result-pin">${PIN_ICON}</span>`}
                <span class="label"><span class="name">${r.name}</span>${r.detail ? html`<span class="detail">${r.detail}</span>` : nothing}</span>
              </li>`)}
          </ul>` : nothing}
      </div>
    `;
  }

  async loadSaved() {
    try {
      this.saved = await this.api.routes();
    } catch (err) {
      this.notice = { text: `Saved routes unavailable: ${err.message}`, bad: true };
    }
  }

  async loadAccount() {
    try {
      this.account = await this.api.me();
    } catch {}
    if (new URLSearchParams(location.search).get('signin') === 'failed') {
      history.replaceState(null, '', '/');
      this.notice = { text: 'Signing in did not work. Try again.', bad: true };
    }
    if (!this.account.user) return;
    this.loadSaved();
    this.loadLists();
  }

  // signIn goes to the provider; the route being drawn waits in this browser for the way back.
  signIn(as) {
    location.href = this.account.signIn === 'dev' ? `/auth/dev?as=${as ?? 'a'}` : '/auth/login';
  }

  async signOut() {
    await this.api.logout();
    this.account = { ...this.account, user: null };
    this.saved = [];
    this.setLists([]);
    this.route = { ...this.route, id: undefined };
    this.flash('Signed out');
  }

  async save() {
    if (!this.account.user) {
      if (this.account.signIn) this.signIn();
      else this.notice = { text: 'Saving routes needs an account, and this server has no sign-in set up.', bad: true };
      return;
    }
    const route = { ...this.route, name: this.route.name.trim() || defaultName() };
    try {
      this.route = await this.api.save(route);
      this.dirty = false;
      this.changed({ edited: false });
      this.flash('Saved');
      this.loadSaved();
    } catch (err) {
      this.notice = { text: `Not saved: ${err.message}`, bad: true };
    }
  }

  open(route) {
    if (!this.confirmDiscard()) return;
    this.route = route;
    this.selected = -1;
    this.dirty = false;
    this.changed({ fit: true, edited: false });
    if (route.fromHere) this.useMyLocation();
    this.nameUnnamed();
  }

  startNew() {
    if (!this.confirmDiscard()) return;
    this.route = { ...blankRoute(), mode: this.route.mode };
    this.selected = -1;
    this.dirty = false;
    this.changed({ edited: false });
  }

  async remove(route) {
    if (!confirm(`Delete "${route.name}"?`)) return;
    await this.api.remove(route.id);
    if (this.route.id === route.id) this.route = { ...this.route, id: undefined };
    this.loadSaved();
  }

  confirmDiscard() {
    return !this.dirty || this.route.stops.length === 0 || confirm('Discard the unsaved changes to this route?');
  }

  // The link, or undefined while it cannot be built yet. Advanced mode needs the plan's via points.
  link() {
    const { stops, fromHere } = this.route;
    if (stops.length < 2 && !(fromHere && stops.length === 1)) return undefined;
    const fresh = this.plan && this.planKey === JSON.stringify(this.planRequest());
    if (this.advanced && !fresh) return undefined;
    return googleMapsLink(this.route, fresh ? this.plan : null, this.offset);
  }

  exportGPX() {
    const fresh = this.plan && this.planKey === JSON.stringify(this.planRequest());
    const blob = new Blob([toGPX(this.route, fresh ? this.plan : null, this.offset ? this.here : null)], { type: 'application/gpx+xml' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${(this.route.name || '').replace(/[^\p{L}\p{N} _-]+/gu, '').replace(/\s+/g, ' ').trim() || 'apex-route'}.gpx`;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  async importGPX(file) {
    if (!file || !this.confirmDiscard()) return;
    try {
      const gpx = fromGPX(await file.text());
      this.route = { ...blankRoute(), mode: this.route.mode, name: gpx.name || file.name.replace(/\.gpx$/i, ''), stops: gpx.stops };
      this.selected = -1;
      this.dirty = true;
      this.changed({ fit: true });
      this.nameUnnamed();
      this.flash(gpx.kind === 'track'
        ? `Track imported as ${gpx.stops.length} stops along it; check the line follows it`
        : `Imported ${gpx.stops.length} stops`);
    } catch (err) {
      this.notice = { text: `Could not import ${file.name}: ${err.message}`, bad: true };
    }
  }

  async copy(text, what) {
    try {
      await navigator.clipboard.writeText(text);
      this.flash(`${what} copied`);
    } catch {
      // The clipboard API needs a secure origin; plain http over Tailscale is not one.
      prompt(`Copy the ${what.toLowerCase()}`, text);
    }
  }

  async shareRouteLink(url) {
    try {
      await navigator.share({ title: this.route.name || 'Apex route', url });
    } catch {}
  }

  // openLinked opens a route read from a route link as an unsaved copy.
  openLinked(route) {
    history.replaceState(null, '', '/');
    if (!this.confirmDiscard()) return;
    this.route = { ...blankRoute(), ...route };
    this.selected = -1;
    this.dirty = true;
    this.changed({ fit: true });
    this.nameUnnamed();
    if (route.fromHere) this.useMyLocation();
    this.flash('Route opened from a link. Save it to keep your own copy.');
  }

  renderShare(link) {
    const apex = toRouteLink(this.route, location.origin);
    return html`
      <div class="popover share-popover">
        <div class="share-row">
          <span class="share-label"><b>Link to this route</b><small>${apex.replace(/^https?:\/\//, '')}</small></span>
          <button class="btn" @click=${() => this.copy(apex, 'Link')}>Copy</button>
          ${'share' in navigator ? html`<button class="btn" @click=${() => this.shareRouteLink(apex)}>Share</button>` : nothing}
        </div>
        <div class="share-row">
          <span class="share-label"><b>Google Maps link</b><small>To navigate with the phone</small></span>
          <button class="btn" ?disabled=${!link} @click=${() => this.copy(link, 'Google Maps link')}>Copy</button>
        </div>
      </div>
    `;
  }

  flash(text) {
    this.notice = { text };
    clearTimeout(this.noticeTimer);
    this.noticeTimer = setTimeout(() => (this.notice = null), 2500);
  }

  render() {
    const { stops, fromHere, roundTrip, avoid } = this.route;
    const plan = this.plan;
    const avoided = this.advanced ? AVOIDS.slice(1) : AVOIDS;
    const stillUses = avoided.filter(([key]) => avoid?.[key] && plan?.has?.[key]).map(([, label]) => label.toLowerCase());
    const link = this.link();
    const googleStops = link ? googleStopCount(link) : 0;

    return html`
      <div class="map"></div>
      <button class="locate" title="My location" @click=${() => this.map.locate()}>◎</button>
      <aside class="panel ${this.collapsed ? 'collapsed' : ''} ${this.planning ? 'planning' : ''}">
        <div class="progress" aria-hidden="true"></div>
        <header>
          <input .value=${this.route.name} placeholder="Untitled route" aria-label="Route name"
            @change=${e => this.setOption('name', e.target.value)}>
          ${this.route.id && !this.dirty
            ? html`<span class="icon saved-mark" title="Saved" aria-label="Saved">${CHECK_ICON}</span>`
            : html`<button class="icon save" title="Save route" aria-label="Save route" ?disabled=${stops.length === 0}
                @click=${() => this.save()}>${SAVE_ICON}</button>`}
          <span class="more">
            <button class="icon popover-toggle" title="More" aria-label="More" aria-haspopup="menu" aria-expanded=${this.popover === 'more'}
              @click=${() => this.togglePopover('more')}>⋯</button>
            ${this.popover === 'more' ? html`
              <div class="popover more-menu" role="menu">
                <button role="menuitem" @click=${() => ((this.popover = null), this.startNew())}>New route</button>
                <button role="menuitem" @click=${() => ((this.popover = null), this.querySelector('input.gpx-file').click())}>Import GPX…</button>
                <button role="menuitem" ?disabled=${stops.length === 0} @click=${() => ((this.popover = null), this.exportGPX())}>Export GPX</button>
              </div>` : nothing}
          </span>
          ${this.renderAccount()}
          <input class="gpx-file" type="file" accept=".gpx,application/gpx+xml" hidden
            @change=${e => (this.importGPX(e.target.files[0]), (e.target.value = ''))}>
          <button class="icon toggle" title=${this.collapsed ? 'Show' : 'Hide'}
            @click=${() => (this.collapsed = !this.collapsed)}>${this.collapsed ? '▴' : '▾'}</button>
        </header>
        <div class="body">
          <div class="mode" role="tablist">
            <button role="tab" aria-selected=${!this.advanced} @click=${() => this.setMode('simple')}>Simple</button>
            <button role="tab" aria-selected=${this.advanced} @click=${() => this.setMode('advanced')}>Advanced</button>
          </div>
          <div class="summary">
          <div class="totals">
            ${plan ? html`<b>${km(plan.meters)}</b><span>${duration(plan.seconds)}</span>` : nothing}
            <span>${stops.length} ${stops.length === 1 ? 'stop' : 'stops'}</span>
            ${this.planning ? html`<span class="working">${this.advanced ? 'Looking for curves' : 'Routing'}</span>` : nothing}
          </div>
          <button class="icon popover-toggle gear" title="Avoid motorways, tolls or ferries" aria-label="Avoid" aria-expanded=${this.popover === 'options'}
            @click=${() => this.togglePopover('options')}>${GEAR_ICON}</button>
          ${this.popover === 'options' ? this.renderRouteOptions() : nothing}
          </div>
          ${this.renderCurves()}
          ${this.activeOptions().length
            ? html`<div class="chips">${this.activeOptions().map(o => html`<button class="chip-tag popover-toggle" @click=${() => this.togglePopover('options')}>${o}</button>`)}</div>`
            : nothing}
          ${this.renderSearch()}
          <ol class="stops">
            ${this.renderStart()}
            ${this.offset && this.advanced && stops.length ? html`<li>${this.renderLeg(-1, plan?.legs[0])}</li>` : nothing}
            ${stops.map((s, i) => this.renderStop(s, i))}
            ${this.renderEnd()}
          </ol>
          <p class="hint">
            ${stops.length === 0
              ? 'Click the map to add the first stop.'
              : 'Click the map to add a stop at the end, or the line to add one in between. Drag a stop to move it.'}
            ${stops.length > 1 ? html` <a href="#" @click=${e => (e.preventDefault(), this.reverse())}>Reverse</a>` : nothing}
          </p>
          ${googleStops > TESTED_GOOGLE_STOPS
            ? html`<p class="notice">Google Maps will list ${googleStops} stops; links have been tested up to ${TESTED_GOOGLE_STOPS}.</p>`
            : nothing}
          ${stillUses.length
            ? html`<p class="notice">No way around it: the route still uses ${stillUses.join(' and ')}.</p>`
            : nothing}
          ${this.notice ? html`<p class="notice ${this.notice.bad ? 'bad' : ''}">${this.notice.text}</p>` : nothing}
          ${this.renderSaved()}
        </div>
        <div class="actions">
          <a class="btn primary" href=${link ?? '#'} target="_blank" rel="noopener" aria-disabled=${!link}>
            ${this.advanced && this.planning ? 'Working out the route…' : 'Open in Google Maps'}
          </a>
          <button class="btn square popover-toggle" title="Share" aria-label="Share" aria-expanded=${this.popover === 'share'}
            ?disabled=${stops.length === 0} @click=${() => this.togglePopover('share')}>${SHARE_ICON}</button>
          ${this.popover === 'share' ? this.renderShare(link) : nothing}
        </div>
      </aside>
      ${this.map ? this.renderLayers() : nothing}
      ${this.map ? this.renderBasemap() : nothing}
    `;
  }

  renderStop(s, i) {
    const stops = this.route.stops;
    const legs = this.plan?.legs ?? [];
    const arriving = legs[i - 1 + this.offset];
    // The leg leaving this stop: to the next one, or back to the start on a round trip.
    const leaving = i < stops.length - 1 || (this.route.roundTrip && stops.length > 1 && i === stops.length - 1);
    const line = [i > 0 || this.offset ? `in-${this.legStyle(i - 1)}` : '', leaving ? `out-${this.legStyle(i)}` : ''];
    return html`
      <li class="item">
        <div class="stop ${line.join(' ')} ${i === this.selected ? 'selected' : ''}" @click=${() => this.select(i)}>
          <span class="grip" title="Drag to reorder" @click=${e => e.stopPropagation()}>⠿</span>
          <span class="n">${i + 1}</span>
          <span class="label" title=${coords(s)}>
            <span class="name ${s.name ? '' : 'pending'}">${s.name || coords(s)}</span>
            ${s.detail ? html`<span class="detail">${s.detail}</span>` : nothing}
          </span>
          ${!this.advanced && arriving ? html`<span class="km">${km(arriving.meters)}</span>` : nothing}
          <button class="icon remove" title="Remove" @click=${e => (e.stopPropagation(), this.removeStop(i))}>✕</button>
        </div>
        ${this.advanced && leaving ? this.renderLeg(i, legs[i + this.offset]) : nothing}
        ${!this.advanced && leaving
          ? html`<div class="gap ${this.legStyle(i)}"><span class="line"></span>${this.insertButton(i + 1)}</div>`
          : nothing}
      </li>
    `;
  }

  // legStyle is how the leg leaving stop i is ridden; -1 is the leg from the rider's position.
  legStyle(i) {
    return this.advanced ? this.legAt(i).style : 'fast';
  }

  // The route can start where the rider is; off, stop 1 is the start.
  renderStart() {
    if (!this.route.fromHere) {
      return html`<li class="endpoint off"><button @click=${() => this.useMyLocation()}><span class="dot here"></span>Start from my location</button></li>`;
    }
    return html`
      <li class="endpoint ${this.route.stops.length ? `out-${this.legStyle(-1)}` : ''}">
        <span class="dot here"></span>
        <span class="label"><span class="name">My location</span><span class="detail">${this.here ? 'Wherever you are when you open the link' : 'Waiting for your location…'}</span></span>
        <button class="icon" title="Start at stop 1 instead" @click=${() => (this.setHere(null), this.setOption('fromHere', false))}>✕</button>
      </li>
    `;
  }

  // Coming back to the start is usually decided last, so it sits at the end of the list.
  renderEnd() {
    const stops = this.route.stops;
    if (stops.length < 2) return nothing;
    if (!this.route.roundTrip) {
      return html`<li class="endpoint off"><button @click=${() => this.setOption('roundTrip', true)}><span class="dot back">${RETURN_ICON}</span>Return to the start</button></li>`;
    }
    const back = this.plan?.legs[stops.length - 1 + this.offset];
    return html`
      <li class="endpoint in-${this.legStyle(stops.length - 1)}">
        <span class="dot back">${RETURN_ICON}</span>
        <span class="label"><span class="name">Back to ${stops[0].name || 'stop 1'}</span><span class="detail">Round trip</span></span>
        ${!this.advanced && back ? html`<span class="km">${km(back.meters)}</span>` : nothing}
        <button class="icon" title="End at the last stop" @click=${() => this.setOption('roundTrip', false)}>✕</button>
      </li>
    `;
  }

  // One line per leg: a pill with how it is ridden, opening a small menu, then its facts.
  renderLeg(i, planned) {
    const leg = this.legAt(i);
    const menu = `leg:${i}`;
    const label = STYLES.find(([value]) => value === leg.style)[1];
    const noMotorways = leg.avoidHighways && leg.style === 'fast';
    return html`
      <div class="leg ${leg.style}">
        <span class="line"></span>
        ${this.insertButton(i + 1)}
        <div class="leg-body">
          <span class="leg-pick">
            <button class="pill popover-toggle" aria-haspopup="menu" aria-expanded=${this.popover === menu}
              title="How to ride this leg" @click=${e => this.openLegMenu(menu, e.currentTarget)}>${label} ▾</button>
            ${noMotorways ? html`<span class="no-motorways" title="No motorways">${NO_MOTORWAY_ICON}</span>` : nothing}
            ${this.popover === menu ? html`
              <div class="popover leg-menu ${this.menuUp ? 'up' : ''}" role="menu">
                ${STYLES.map(([value, text]) => html`
                  <div class="menu-row">
                    <button class="dot-pick" title=${`Colour of ${text.toLowerCase()} legs`} aria-label=${`Colour of ${text.toLowerCase()} legs`}
                      aria-expanded=${this.colourEdit === value}
                      @click=${() => (this.colourEdit = this.colourEdit === value ? null : value)}><span class="style-dot ${value}"></span></button>
                    <button role="menuitemradio" aria-checked=${leg.style === value}
                      @click=${() => (this.setLeg(i, { style: value }), (this.popover = null))}>${text}</button>
                  </div>
                  ${this.colourEdit === value ? this.renderSwatches(value) : nothing}`)}
                ${leg.style === 'fast'
                  ? html`<label><input type="checkbox" .checked=${leg.avoidHighways}
                      @change=${e => this.setLeg(i, { avoidHighways: e.target.checked })}> No motorways</label>`
                  : nothing}
              </div>` : nothing}
          </span>
          ${planned ? this.legFacts(planned) : html`<span class="facts">…</span>`}
        </div>
      </div>
    `;
  }

  legFacts(planned) {
    const c = curves(planned.curvature);
    const hidden = planned.via?.length ?? 0;
    return html`
      <span class="facts">
        <span>${km(planned.meters)}</span>
        <span>${shortDuration(planned.seconds)}</span>
        <span class="curves" title=${`${Math.round(planned.curvature)}° turned per km`}>
          <span class="bars">${[1, 2, 3, 4, 5].map(n => html`<i class=${n <= c.level ? 'on' : ''}></i>`)}</span>${c.label}
        </span>
        ${planned.limited ? html`<span class="limited" title="Out of curve searches for a moment: drawn fast without motorways, and redone with curves by itself.">Fast for now</span>` : nothing}
        ${hidden
          ? html`<span class="hidden" title=${`${hidden} hidden ${hidden === 1 ? 'stop' : 'stops'} so Google Maps rides this leg as drawn; it lists them as stops`}>+${hidden}</span>`
          : nothing}
      </span>
    `;
  }

  listBadge(l) {
    const colour = `colour-${l.colour || 'teal'}`;
    return l.icon ? html`<span class="list-icon ${colour}">${l.icon}</span>` : html`<span class="swatch ${colour}"></span>`;
  }

  activeOptions() {
    const { avoid = {} } = this.route;
    return [
      !this.advanced && avoid.highways && 'No motorways',
      avoid.tolls && 'No tolls',
      avoid.ferries && 'No ferries',
    ].filter(Boolean);
  }

  renderRouteOptions() {
    const { avoid } = this.route;
    const avoided = this.advanced ? AVOIDS.slice(1) : AVOIDS;
    return html`
      <div class="popover options-popover">
        <span class="group">Avoid</span>
        ${avoided.map(([key, label]) => html`
          <label><input type="checkbox" .checked=${!!avoid?.[key]} @change=${e => this.setAvoid(key, e.target.checked)}> ${label}</label>`)}
        ${this.advanced ? html`<p class="hint">Motorways are chosen per leg in advanced mode.</p>` : nothing}
      </div>
    `;
  }

  // neighbours are the points joined to stop i by a leg: the stop or position before it and the one after,
  // which on a round trip wraps from the last stop back to the first.
  neighbours(i) {
    const { stops, roundTrip } = this.route;
    const loop = roundTrip && stops.length > 1;
    const prev = i > 0 ? stops[i - 1] : loop ? stops[stops.length - 1] : this.offset ? this.here : null;
    const next = i < stops.length - 1 ? stops[i + 1] : loop ? stops[0] : null;
    return { prev, next };
  }

  // insertButton sits in the middle of the line between two points and adds a found place right there.
  insertButton(at) {
    const where = at === 0 ? 'before stop 1' : at >= this.route.stops.length ? 'at the end' : `between ${at} and ${at + 1}`;
    return html`<button class=${`insert-here ${this.insertAt === at ? 'on' : ''}`} title=${`Add a stop ${where}`} aria-label=${`Add a stop ${where}`}
      @click=${() => this.insertHere(at)}>+</button>`;
  }

  insertHere(at) {
    this.insertAt = this.insertAt === at ? null : at;
    this.updateComplete.then(() => this.querySelector('.search input').focus());
  }

  // A strip of colours for one leg kind, opened from its dot in the leg menu.
  renderSwatches(style) {
    const chosen = legColours()[style];
    return html`
      <div class="swatch-strip" role="radiogroup" aria-label=${`Colour of ${style} legs`}>
        ${Object.keys(LEG_COLOURS).map(c => html`
          <button class="swatch-pick leg-colour-${c}" role="radio" aria-checked=${chosen === c} title=${c} aria-label=${c}
            @click=${() => (setLegColour(style, c), this.requestUpdate())}></button>`)}
      </div>
    `;
  }

  // As in Google Maps: a thumbnail of the other map type flips to it.
  renderBasemap() {
    const flip = this.map.base === 'Satellite' ? 'Roads' : 'Satellite';
    return html`
      <button class="basemap" title=${`Switch to ${BASE_LABELS[flip]}`}
        @click=${() => (this.map.setBase(flip), this.requestUpdate())}>
        <img src=${BASE_PREVIEWS[flip]} alt=""><span>${BASE_LABELS[flip]}</span>
      </button>
    `;
  }

  // renderAccount is the rider's picture in the header, opening who they are and Sign out, or a
  // Sign in button when nobody is signed in.
  renderAccount() {
    const { user, signIn } = this.account;
    if (!user) {
      if (!signIn) return nothing;
      return html`
        <span class="account">
          <button class="small popover-toggle" aria-expanded=${this.popover === 'account'}
            @click=${() => (signIn === 'dev' ? this.togglePopover('account') : this.signIn())}>Sign in</button>
          ${this.popover === 'account' ? html`
            <div class="popover account-menu" role="menu">
              <span class="menu-note">Test riders (development only)</span>
              <button role="menuitem" @click=${() => this.signIn('a')}>Rider A</button>
              <button role="menuitem" @click=${() => this.signIn('b')}>Rider B</button>
            </div>` : nothing}
        </span>`;
    }
    const initial = (user.name || user.email).trim().charAt(0).toUpperCase();
    return html`
      <span class="account">
        <button class="avatar popover-toggle" title=${user.email} aria-label="Account" aria-expanded=${this.popover === 'account'}
          @click=${() => this.togglePopover('account')}>
          ${user.picture ? html`<img src=${user.picture} alt="" referrerpolicy="no-referrer">` : initial}
        </button>
        ${this.popover === 'account' ? html`
          <div class="popover account-menu" role="menu">
            <div class="who">
              ${user.name ? html`<b>${user.name}</b>` : nothing}
              <span>${user.email}</span>
            </div>
            <button role="menuitem" @click=${() => ((this.popover = null), this.signOut())}>Sign out</button>
          </div>` : nothing}
      </span>`;
  }

  renderLayers() {
    return html`
      <button class="map-button places-toggle popover-toggle" title="My places" aria-label="My places" aria-expanded=${this.popover === 'places'}
        @click=${() => this.togglePopover('places')}>${PLACES_ICON}</button>
      ${this.popover === 'places' ? html`
        <div class="popover map-popover places-popover">
          <h3>My places</h3>
          ${!this.account.user ? html`
            <p class="hint">Sign in to link your Google Maps lists and keep them on every device.</p>
            ${this.account.signIn ? html`<button class="btn primary signin-btn" @click=${() => this.signIn()}>Sign in</button>` : nothing}
          ` : html`
          <ul class="layer-lists">
            ${this.lists.map(l => html`
              <li>
                <label class="switch" title=${this.hiddenLists.has(l.id) ? 'Show on the map' : 'Hide from the map'}>
                  <input type="checkbox" role="switch" .checked=${!this.hiddenLists.has(l.id)} @change=${e => this.toggleList(l.id, e.target.checked)}>
                  <span></span>
                </label>
                <button class="colour-button" title="Colour" @click=${() => (this.pickingColour = this.pickingColour === l.id ? null : l.id)}>${this.listBadge(l)}</button>
                <span class="list-text">
                  <span class="list-name">${l.name}</span>
                  <small>${l.places.length} places</small>
                </span>
                <button class="icon" title="Refresh from Google Maps" ?disabled=${this.listBusy} @click=${() => this.refreshList(l.id)}>↻</button>
                <button class="icon" title="Unlink" @click=${() => this.unlinkList(l)}>✕</button>
              </li>
              ${this.pickingColour === l.id ? html`
                <li class="palette">
                  ${COLOURS.map(c => html`<button class="swatch big colour-${c} ${(l.colour || 'teal') === c ? 'current' : ''}" title=${c} aria-label=${c}
                    @click=${() => this.setListColour(l.id, c)}></button>`)}
                </li>` : nothing}`)}
          </ul>
          <form class="add-list" @submit=${e => (e.preventDefault(), this.addList(e.target))}>
            <input name="url" type="url" required placeholder="Paste a Google Maps list link" aria-label="Google Maps list link">
            <button class="btn primary" ?disabled=${this.listBusy}>${this.listBusy ? 'Adding…' : 'Add'}</button>
          </form>
          ${this.listError ? html`<p class="notice bad">${this.listError}</p>` : nothing}
          ${this.lists.length === 0
            ? html`<p class="hint">In Google Maps open a list, tap Share and paste the link. It stays linked and refreshes by itself.</p>`
            : nothing}
          `}
        </div>` : nothing}
    `;
  }

  renderSaved() {
    if (this.saved.length === 0) return nothing;
    return html`
      <section class="saved">
        <h2>Saved routes</h2>
        <ul>
          ${this.saved.map(r => html`
            <li class=${r.id === this.route.id ? 'current' : ''}>
              <button class="open" @click=${() => this.open(r)}>${r.name} <small>${r.stops.length} stops</small></button>
              <button class="icon" title="Delete" @click=${() => this.remove(r)}>✕</button>
            </li>`)}
        </ul>
      </section>
    `;
  }
}

function blankRoute() {
  return { name: '', mode: 'simple', stops: [], fromHere: false, roundTrip: false, avoid: {} };
}

function defaultName() {
  return `Route ${new Date().toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}`;
}

function loadDraft() {
  try {
    const draft = JSON.parse(localStorage.getItem(DRAFT_KEY));
    return Array.isArray(draft?.stops) ? draft : null;
  } catch {
    return null;
  }
}

customElements.define('apex-app', ApexApp);

function readJSON(key) {
  try {
    return JSON.parse(localStorage.getItem(key));
  } catch {
    return null;
  }
}

function writeJSON(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {}
}



function legColours() {
  const saved = readJSON(LEG_COLOURS_KEY) ?? {};
  return Object.fromEntries(Object.entries(DEFAULT_LEG_COLOURS).map(([style, c]) => [style, saved[style] in LEG_COLOURS ? saved[style] : c]));
}

function setLegColour(style, colour) {
  writeJSON(LEG_COLOURS_KEY, { ...legColours(), [style]: colour });
  applyLegColours();
}


// Set through the CSSOM, which the page's CSP allows where a style attribute would be blocked.
export function applyLegColours() {
  const shade = darkScheme.matches ? 1 : 0;
  for (const [style, colour] of Object.entries(legColours())) {
    document.documentElement.style.setProperty(LEG_VARS[style], LEG_COLOURS[colour][shade]);
  }
}

darkScheme.addEventListener('change', applyLegColours);

// fold lower-cases and drops accents, so "soller" finds Sóller.
function fold(s) {
  return s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
}
