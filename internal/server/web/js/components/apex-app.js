import { LitElement, html, nothing, svg } from '../../vendor/lit-3.3.3.js';
import { BASE_PREVIEWS, RouteMap } from '../map.js';
import { googleMapsLink, googleStopCount, routePoints } from '../link.js';
import { coords, curves, duration, km, shortDuration } from '../format.js';
import { lang, plural, t } from '../i18n.js';
import { fromGPX, toGPX } from '../gpx.js';
import { fromRouteLink, toRouteLink } from '../routelink.js';
import { STEPS, markTourSeen, placeBoxes, renderTour, tourSeen } from '../guide.js';

const DRAFT_KEY = 'apex.draft';
const HIDDEN_KEY = 'apex.hiddenLists';
const LOCAL_LISTS_KEY = 'apex.lists';
const DONATE_KEY = 'apex.donateState';
const ACTIVE_TICK_S = 15;
// Time counts as use while the page is in view and someone touched it within this.
const IDLE_MS = 120e3;
const LIST_STALE_MS = 3 * 3600e3;
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
const LAYERS_ICON = svg`<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" aria-hidden="true"><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 13 9 5 9-5"/></svg>`;
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
const NEW_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-6"/><path d="M17.5 3.5a2.1 2.1 0 0 1 3 3L12 15l-4 1 1-4Z"/></svg>`;
const HELP_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.3 2.4c-.5.2-.8.6-.8 1.1v.5"/><path d="M12 17h0"/></svg>`;
const GPX_ICON = svg`<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z"/><path d="M14 3v5h5"/><path d="M9 17.5c1.5 0 1.5-4 3-4s1.5 2.5 3 2.5"/></svg>`;
const ROUTE_ICON = svg`<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="6" cy="19" r="2"/><circle cx="18" cy="5" r="2"/><path d="M8 19h8.5a3.5 3.5 0 0 0 0-7h-9a3.5 3.5 0 0 1 0-7H16"/></svg>`;
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
    tall: { state: true },
    here: { state: true },
    lists: { state: true },
    copied: { state: true },
    donateAsk: { state: true },
    hiddenLists: { state: true },
    listBusy: { state: true },
    listError: { state: true },
    popover: { state: true },
    pickingColour: { state: true },
    colourEdit: { state: true },
    query: { state: true },
    tour: { state: true },
    curves: { state: true },
    account: { state: true },
    insertAt: { state: true },
    results: { state: true },
    active: { state: true },
  };

  constructor(api) {
    super();
    this.api = api;
    const firstVisit = !tourSeen() && !location.pathname.startsWith('/r/');
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
    this.tour = null;
    this.firstVisit = firstVisit;
    // Read before the address starts following the route.
    this.signinFailed = new URLSearchParams(location.search).get('signin') === 'failed';
    this.linked = fromRouteLink(location);
    // A reload of the page's own address is the draft itself, not a route opened from a link.
    if (this.linked && toRouteLink(this.route, '') === location.pathname + location.search) this.linked = null;
    this.curves = null; // { left, max, nextIn } curve searches this user has, from the last plan
    this.account = { user: null, signIn: '' }; // who is signed in, and how one can sign in here
    this.insertAt = null; // where a found place goes; null lets it find its best gap
    this.results = [];
    this.active = 0;
    // Back and Forward, or Ctrl+Z and Ctrl+Shift+Z, step through the route's earlier versions.
    this.onHistory = e => {
      if (!e.state?.route) return;
      this.route = e.state.route;
      this.dirty = e.state.dirty;
      this.selected = -1;
      this.changed({ edited: false });
      this.nameUnnamed();
    };
    this.onOutside = e => {
      if (this.popover && !e.target.closest('.popover, .popover-toggle')) this.popover = null;
      if (this.results.length && !e.target.closest('.search')) this.results = [];
    };
    this.onKey = e => {
      const typing = e.target.closest('input, select, textarea');
      if ((e.metaKey || e.ctrlKey) && !typing && (e.key.toLowerCase() === 'z' || e.key === 'y')) {
        e.preventDefault();
        if (e.shiftKey || e.key === 'y') history.forward();
        else if (history.state?.step > 0) history.back();
        return;
      }
      if (e.key === 'Escape') {
        this.popover = null;
        if (this.tour) this.closeTour();
      }
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
    window.addEventListener('popstate', this.onHistory);
  }

  disconnectedCallback() {
    document.removeEventListener('keydown', this.onKey);
    document.removeEventListener('pointerdown', this.onOutside);
    window.removeEventListener('popstate', this.onHistory);
    super.disconnectedCallback();
  }

  // A .gpx dropped anywhere on the page opens like an imported one.
  dropGPX(e) {
    const file = [...(e.dataTransfer?.files ?? [])].find(f => /\.gpx$/i.test(f.name));
    if (!file) return;
    e.preventDefault();
    this.importGPX(file);
  }

  firstUpdated() {
    this.lastInput = 0;
    this.donateSynced = 0;
    for (const type of ['pointerdown', 'pointermove', 'keydown', 'wheel']) {
      window.addEventListener(type, () => (this.lastInput = Date.now()), { passive: true });
    }
    setInterval(() => this.countActiveTime(), ACTIVE_TICK_S * 1000);
    this.addEventListener('dragover', e => e.dataTransfer?.types.includes('Files') && e.preventDefault());
    this.addEventListener('drop', e => this.dropGPX(e));
    this.map = new RouteMap(this.querySelector('.map'), {
      add: p => this.addStop(p, this.route.stops.length),
      insert: (leg, p) => this.addStop(p, leg + 1 - this.offset),
      move: (i, p) => this.relocateStop(i, p),
      select: i => this.pinClicked(i),
      remove: i => this.removeStop(i),
      copy: p => this.copy(`${p.lat.toFixed(5)}, ${p.lng.toFixed(5)}`),
      neighbours: i => this.neighbours(i),
      preview: (points, signal) => this.api.plan({ points: points.map(p => [p.lat, p.lng]), avoid: this.route.avoid ?? {}, legs: null }, signal),
    });
    this.changed({ fit: true, edited: false });
    this.requestUpdate(); // The layers panel renders once the map exists.
    this.loadAccount();
    if (this.linked) this.openLinked(this.linked);
    if (this.route.fromHere) this.useMyLocation();
    this.nameUnnamed();
    if (this.firstVisit) setTimeout(() => this.tourTo(0), 600);
    window.addEventListener('resize', () => this.tour && this.tourTo(this.tour.step));
    // The locate button rides just above the panel on a phone, however far it is pulled.
    new ResizeObserver(([entry]) => this.style.setProperty('--sheet', `${entry.borderBoxSize[0].blockSize}px`)).observe(this.querySelector('.panel'));
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

  // record makes the change a step of the browser's history, so Back and Ctrl+Z undo it.
  changed({ fit = false, edited = true, record = edited } = {}) {
    if (edited) this.dirty = true;
    try {
      localStorage.setItem(DRAFT_KEY, JSON.stringify({ ...this.route, dirty: this.dirty }));
    } catch {}
    this.map.setStops(this.route.stops, this.selected, this.closesLoop());
    // The address is always a link to the route on screen, ready to copy and pass on.
    const address = this.route.stops.length ? toRouteLink(this.route, '') : '/';
    const step = history.state?.step ?? 0;
    const same = JSON.stringify(history.state?.route) === JSON.stringify(this.route);
    if (record && !same) history.pushState({ route: this.route, dirty: this.dirty, step: step + 1 }, '', address);
    else history.replaceState({ route: this.route, dirty: this.dirty, step }, '', address);
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
    if (request.legs) this.drawProvisional(request, pending.signal);
    try {
      this.plan = await this.api.plan(request, pending.signal);
      this.planKey = key;
      this.trackCurves(this.plan);
      const styles = request.legs?.map(l => l.style) ?? [];
      this.map.setLegs(this.plan.legs, styles);
      this.lastDrawn = { points: request.points, legs: this.plan.legs, styles };
      if (this.notice?.planning) this.notice = null;
    } catch (err) {
      if (err.name !== 'AbortError') this.notice = { text: t('Could not draw the route: {err}', { err: err.message }), bad: true, planning: true };
    } finally {
      if (this.pending === pending) {
        this.planning = false;
        this.map.setPending(false);
      }
    }
  }

  // drawProvisional draws at once, with one fast call, the legs the last plan does not have yet (a
  // stop added or moved), next to the legs that did not change, while the full plan, curves and
  // hidden stops included, is still being worked out.
  async drawProvisional(request, signal) {
    const known = new Map();
    const last = this.lastDrawn;
    if (last) last.points.slice(1).forEach((p, i) => known.set(JSON.stringify([last.points[i], p]), { leg: last.legs[i], style: last.styles[i] }));
    const pairs = request.points.slice(1).map((p, i) => [request.points[i], p]);
    const fresh = pairs.map(pair => !known.has(JSON.stringify(pair)));
    if (!fresh.some(Boolean)) return;
    try {
      const quick = await this.api.plan({ points: request.points, avoid: request.avoid, legs: null }, signal);
      // The full plan may have come back first; the quick one must not draw over it.
      if (signal.aborted || !this.planning || this.pending?.signal !== signal) return;
      const legs = pairs.map((pair, i) => (fresh[i] ? { ...quick.legs[i], provisional: true } : known.get(JSON.stringify(pair)).leg));
      const styles = pairs.map((pair, i) => (fresh[i] ? request.legs[i].style : known.get(JSON.stringify(pair)).style));
      this.map.setLegs(legs, styles);
    } catch {}
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
    const text = level === 'very-low'
      ? plural(c.left, 'Only {n} curve search left', 'Only {n} curve searches left')
      : t('{left} of {max} curve searches left', { left: c.left, max: c.max });
    return html`<div class="quota ${level}" title=${t('Searching for curves asks the public routing server many times, so each rider gets a few at a time; they come back by themselves.')}>${text}</div>`;
  }

  update(changed) {
    if (changed.has('selected') && this.map) this.map.setStops(this.route.stops, this.selected, this.closesLoop());
    super.update(changed);
  }

  setStops(stops, options) {
    const atStops = this.account.donate?.atStops;
    if (atStops && this.route.stops.length < atStops && stops.length >= atStops) this.askDonate();
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
    const { approx, ...was } = this.route.stops[i];
    const stop = { ...was, ...p, name: '', detail: '' };
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
      this.changed({ edited: this.dirty, record: false });
    } catch {}
  }

  closesLoop() {
    const { stops, roundTrip, fromHere } = this.route;
    return stops.length > 1 && !roundTrip && !fromHere;
  }

  // On the map, clicking the first pin of an open route closes the loop, as when drawing a shape.
  pinClicked(i) {
    if (i > 0 || !this.closesLoop()) return this.select(i);
    this.setOption('roundTrip', true);
    this.flash(t('Back to the start'));
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
    this.notice = { text: t('Waiting for your location…') };
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
      this.notice = { text: t('Location not allowed, so the route starts at stop 1.'), bad: true };
    }
  }

  locateMe() {
    const failed = err =>
      (this.notice = { text: t(err?.code === 1 ? 'Location not allowed. Allow it in the browser settings.' : 'Could not find your location.'), bad: true });
    if (!navigator.geolocation) return failed();
    navigator.geolocation.getCurrentPosition(
      ({ coords }) => {
        const p = { lat: coords.latitude, lng: coords.longitude };
        if (this.route.fromHere) this.setHere(p);
        this.map.showPosition(p, this.querySelector('.panel').getBoundingClientRect());
      },
      failed,
      { enableHighAccuracy: true, timeout: 15000, maximumAge: 30000 },
    );
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
      this.notice = { text: t('Your Google lists are unavailable: {err}', { err: err.message }), bad: true };
    }
  }

  setLists(lists) {
    this.lists = lists;
    this.drawPlaces();
  }

  drawPlaces() {
    this.map.setPlaces(this.lists.filter(l => !this.hiddenLists.has(l.id)), place => this.addPlace(place));
  }

  // Signed out, the linked lists live in this browser; signed in, in the rider's account.
  get localLists() {
    return !this.account.user;
  }

  setLocalLists(lists) {
    writeJSON(LOCAL_LISTS_KEY, lists);
    this.setLists(lists);
  }

  // loadLocalLists shows this browser's lists and re-reads the ones older than the server's refresh.
  async loadLocalLists() {
    this.setLists(readJSON(LOCAL_LISTS_KEY) ?? []);
    for (const l of this.lists.filter(l => Date.now() - Date.parse(l.fetched) > LIST_STALE_MS)) {
      try {
        const fresh = await this.api.readList(l.url);
        this.setLocalLists(this.lists.map(x => (x.id === l.id ? { ...fresh, colour: x.colour } : x)));
      } catch {}
    }
  }

  // adoptLocalLists moves the lists linked while signed out into the account that just signed in.
  async adoptLocalLists() {
    const local = readJSON(LOCAL_LISTS_KEY) ?? [];
    for (const l of local) {
      try {
        await this.api.addList(l.url);
        if (l.colour) await this.api.setListColour(l.id, l.colour);
      } catch {}
    }
    if (local.length) writeJSON(LOCAL_LISTS_KEY, []);
  }

  nextColour() {
    return COLOURS.find(c => !this.lists.some(l => l.colour === c)) ?? COLOURS[this.lists.length % COLOURS.length];
  }

  async setListColour(id, colour) {
    this.pickingColour = null;
    if (this.localLists) {
      this.setLocalLists(this.lists.map(l => (l.id === id ? { ...l, colour } : l)));
      return;
    }
    try {
      const updated = await this.api.setListColour(id, colour);
      this.setLists(this.lists.map(l => (l.id === id ? updated : l)));
    } catch (err) {
      this.listError = t('Could not change the colour: {err}', { err: err.message });
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
      let list;
      if (this.localLists) {
        const read = await this.api.readList(input.value);
        list = { ...read, colour: this.lists.find(l => l.id === read.id)?.colour ?? this.nextColour() };
        this.setLocalLists([...this.lists.filter(l => l.id !== list.id), list]);
      } else {
        list = await this.api.addList(input.value);
        this.setLists([...this.lists.filter(l => l.id !== list.id), list]);
      }
      input.value = '';
      this.flash(t('Linked "{name}" with {n} places', { name: list.name, n: list.places.length }));
    } catch (err) {
      this.listError = t('Could not link the list: {err}', { err: err.message });
    } finally {
      this.listBusy = false;
    }
  }

  async refreshList(id) {
    this.listBusy = true;
    this.listError = '';
    try {
      const old = this.lists.find(l => l.id === id);
      const fresh = this.localLists ? { ...(await this.api.readList(old.url)), colour: old.colour } : await this.api.refreshList(id);
      (this.localLists ? this.setLocalLists : this.setLists).call(this, this.lists.map(l => (l.id === id ? fresh : l)));
      this.flash(t('"{name}" is up to date', { name: fresh.name }));
    } catch (err) {
      this.listError = t('Could not refresh: {err}', { err: err.message });
    } finally {
      this.listBusy = false;
    }
  }

  async unlinkList(list) {
    if (!confirm(t('Stop showing "{name}"? The list stays in Google Maps.', { name: list.name }))) return;
    const rest = this.lists.filter(l => l.id !== list.id);
    if (this.localLists) return this.setLocalLists(rest);
    await this.api.removeList(list.id);
    this.setLists(rest);
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
    this.flash(t('{name} added as stop {n}', { name: place.name, n: index + 1 }));
  }

  // Places from the linked lists match first, as the rider's own; the geocoder fills the rest.
  async searchPlaces(q) {
    this.query = q;
    this.active = 0;
    clearTimeout(this.searchTimer);
    this.searchAbort?.abort();
    if (this.searchLink(q.trim())) return;
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

  // searchLink turns a pasted route link into the one result that opens it: a Google Maps route,
  // read by the server, or an Apex route link.
  searchLink(text) {
    let u;
    try {
      u = new URL(text);
    } catch {
      return false;
    }
    const apex = u.host === location.host && fromRouteLink(u);
    if (apex) {
      this.results = [{ kind: 'apex', route: apex }];
      return true;
    }
    if (!/^(maps\.app\.goo\.gl|goo\.gl|(www\.|maps\.)?google\.[a-z.]+)$/.test(u.host)) return false;
    this.results = [{ kind: 'google', loading: true }];
    const abort = (this.searchAbort = new AbortController());
    this.api.importRoute(text, abort.signal).then(
      preview => this.query.trim() === text && (this.results = [{ kind: 'google', preview }]),
      err => err.name !== 'AbortError' && this.query.trim() === text && (this.results = [{ kind: 'google', error: err.message }]),
    );
    return true;
  }

  choose(result) {
    if (!result) return;
    if (result.kind) {
      if (result.kind === 'apex') this.openLinked(result.route);
      else if (result.preview) this.openImported(result.preview);
      else return;
      this.query = '';
      this.results = [];
      return;
    }
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
      ? t(stops.length ? 'Search a place to add' : 'Search where to start')
      : at === 0 ? t('Add before stop 1') : at >= stops.length ? t('Add at the end') : t('Add between {a} and {b}', { a: at, b: at + 1 });
    return html`
      <div class="search">
        <span class="search-icon">${SEARCH_ICON}</span>
        <input type="search" .value=${this.query} placeholder=${placeholder} aria-label=${t('Search a place')}
          autocomplete="off" @input=${e => this.searchPlaces(e.target.value)} @keydown=${e => this.searchKey(e)}>
        ${at !== null ? html`<button class="icon" title=${t('Cancel')} @click=${() => (this.insertAt = null)}>✕</button>` : nothing}
        ${this.results.length ? html`
          <ul class="popover results" role="listbox">
            ${this.results.map((r, i) => html`
              <li role="option" aria-selected=${i === this.active} @pointerenter=${() => (this.active = i)}
                @click=${() => this.choose(r)}>
                ${r.kind ? this.renderLinkResult(r) : html`
                  ${r.list ? this.listBadge(r.list) : html`<span class="result-pin">${PIN_ICON}</span>`}
                  <span class="label"><span class="name">${r.name}</span>${r.detail ? html`<span class="detail">${r.detail}</span>` : nothing}</span>`}
              </li>`)}
          </ul>` : nothing}
      </div>
    `;
  }

  renderLinkResult(r) {
    const icon = html`<span class="result-pin">${ROUTE_ICON}</span>`;
    if (r.kind === 'apex') {
      return html`${icon}<span class="label"><span class="name">${t('Open this route')}</span>
        <span class="detail">${plural(r.route.stops.length, '{n} stop', '{n} stops')}</span></span>`;
    }
    if (r.loading) return html`<span class="result-pin"><span class="spinner"></span></span><span class="label"><span class="name">${t('Reading the Google Maps link…')}</span></span>`;
    if (r.error) return html`${icon}<span class="label"><span class="name">${t('This link is not a route')}</span><span class="detail">${t('In Google Maps, share it from Directions')}</span></span>`;
    const { stops, fromHere, avoid } = r.preview;
    const names = [...(fromHere ? [t('My location')] : []), ...stops.map(s => s.name || coords(s))];
    const avoided = [['highways', 'No motorways'], ['tolls', 'No tolls'], ['ferries', 'No ferries']].filter(([k]) => avoid[k]).map(([, l]) => t(l));
    return html`${icon}<span class="label">
      <span class="name">${t('Import the Google Maps route')}</span>
      <span class="detail chain">${names.join(' → ')}</span>
      <span class="detail">${[plural(stops.length, '{n} stop', '{n} stops'), ...avoided].join(', ')}</span>
    </span>`;
  }

  async loadSaved() {
    try {
      this.saved = await this.api.routes();
    } catch (err) {
      this.notice = { text: t('Saved routes unavailable: {err}', { err: err.message }), bad: true };
    }
  }

  async loadAccount() {
    try {
      this.account = await this.api.me();
    } catch {}
    if (this.signinFailed) {
      this.notice = { text: t('Signing in did not work. Try again.'), bad: true };
    }
    if (!this.account.user) return this.loadLocalLists();
    this.loadSaved();
    this.loadDonateState();
    await this.adoptLocalLists();
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
    this.setLists(readJSON(LOCAL_LISTS_KEY) ?? []);
    this.route = { ...this.route, id: undefined };
    this.flash(t('Signed out'));
  }

  async save() {
    if (!this.account.user) {
      if (this.account.signIn) this.signIn();
      else this.notice = { text: t('Saving routes needs an account, and this server has no sign-in set up.'), bad: true };
      return;
    }
    const route = { ...this.route, name: this.route.name.trim() || defaultName() };
    try {
      this.route = await this.api.save(route);
      this.dirty = false;
      this.changed({ edited: false });
      this.flash(t('Saved'));
      this.loadSaved();
    } catch (err) {
      this.notice = { text: t('Not saved: {err}', { err: err.message }), bad: true };
    }
  }

  open(route) {
    if (!this.confirmDiscard()) return;
    this.route = route;
    this.selected = -1;
    this.dirty = false;
    this.changed({ fit: true, edited: false, record: true });
    if (route.fromHere) this.useMyLocation();
    this.nameUnnamed();
  }

  startNew() {
    if (!this.confirmDiscard()) return;
    this.route = { ...blankRoute(), mode: this.route.mode };
    this.selected = -1;
    this.dirty = false;
    this.changed({ edited: false, record: true });
  }

  async remove(route) {
    if (!confirm(t('Delete "{name}"?', { name: route.name }))) return;
    await this.api.remove(route.id);
    if (this.route.id === route.id) this.route = { ...this.route, id: undefined };
    this.loadSaved();
  }

  confirmDiscard() {
    return !this.dirty || this.route.stops.length === 0 || confirm(t('Discard the unsaved changes to this route?'));
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
        ? t('Track imported as {n} stops along it; check the line follows it', { n: gpx.stops.length })
        : plural(gpx.stops.length, 'Imported {n} stop', 'Imported {n} stops'));
    } catch (err) {
      this.notice = { text: t('Could not import {file}: {err}', { file: file.name, err: err.message }), bad: true };
    }
  }

  // key names the button that shows the text was copied.
  async copy(text, key) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // The clipboard API needs a secure origin; plain http over Tailscale is not one.
      prompt(t('Copy the link'), text);
    }
    if (key) {
      this.copied = key;
      clearTimeout(this.copiedTimer);
      this.copiedTimer = setTimeout(() => (this.copied = null), 1500);
    }
  }





  // share hands the Apex link to the system's share sheet; a browser without one copies it.
  async share() {
    const url = toRouteLink(this.route, location.origin);
    if ('share' in navigator) {
      try {
        await navigator.share({ title: this.route.name || t('Apex route'), url });
      } catch {
        return;
      }
    } else {
      await this.copy(url, 'apex');
    }
    this.askDonate();
  }

  openImported({ stops, fromHere, avoid }) {
    if (!this.confirmDiscard()) return;
    this.route = {
      ...blankRoute(),
      mode: this.route.mode,
      fromHere,
      avoid,
      stops: stops.map(s => ({ lat: s.lat, lng: s.lng, name: s.name ?? '', detail: '', ...(s.approx ? { approx: true } : {}) })),
    };
    this.selected = -1;
    this.dirty = true;
    this.changed({ fit: true });
    this.nameUnnamed();
    if (fromHere) this.useMyLocation();
    this.flash(plural(stops.length, 'Imported {n} stop from Google Maps', 'Imported {n} stops from Google Maps'));
  }

  // openLinked opens a route read from a route link as an unsaved copy.
  openLinked(route) {
    if (!this.confirmDiscard()) return;
    this.route = { ...blankRoute(), ...route };
    this.selected = -1;
    this.dirty = true;
    this.changed({ fit: true, record: false });
    this.nameUnnamed();
    if (route.fromHere) this.useMyLocation();
    this.flash(t('Route opened from a link. Save it to keep your own copy.'));
  }

  // askDonate, at a moment the rider took a route out or drew a long one, invites them to chip in once
  // they have used Apex long enough (APEX_DONATE). The wait is in active time, so who rides a lot is
  // asked sooner; each "Not now" doubles it.
  askDonate() {
    const donate = this.account.donate;
    if (!donate) return;
    const seen = donateSeen();
    const now = Date.now();
    if (seen.activeSeconds < (seen.dueSeconds || donate.firstSeconds) || now - seen.shownAt < donate.gapSeconds * 1000) return;
    this.keepDonate({ ...seen, shownAt: now }, true);
    const show = () => {
      this.popover = null;
      this.donateAsk = true;
      this.api.donateEvent('shown');
    };
    // Opening Google Maps hides this tab; the invitation waits for the rider to come back.
    setTimeout(() => {
      if (!document.hidden) return show();
      document.addEventListener('visibilitychange', function back() {
        if (document.hidden) return;
        document.removeEventListener('visibilitychange', back);
        setTimeout(show, 600);
      });
    }, 1200);
  }

  countActiveTime() {
    if (!this.account.donate || document.hidden || Date.now() - this.lastInput > IDLE_MS) return;
    const seen = donateSeen();
    this.keepDonate({ ...seen, activeSeconds: seen.activeSeconds + ACTIVE_TICK_S }, Date.now() - this.donateSynced > 60e3);
  }

  // keepDonate stores the donation state in this browser and, for a signed-in rider, on the server
  // too, so a new browser or a cleared one does not start from zero.
  keepDonate(seen, sync) {
    writeJSON(DONATE_KEY, seen);
    if (!sync || !this.account.user) return;
    this.donateSynced = Date.now();
    this.api.saveDonateState(seen).then(merged => writeJSON(DONATE_KEY, furthest(donateSeen(), merged)), () => {});
  }

  async loadDonateState() {
    if (!this.account.donate) return;
    try {
      this.keepDonate(furthest(donateSeen(), await this.api.donateState()), true);
    } catch {}
  }

  renderDonateAsk() {
    const close = event => {
      this.donateAsk = false;
      this.api.donateEvent(event);
      const donate = this.account.donate;
      const seen = donateSeen();
      if (event === 'clicked') {
        this.keepDonate({ ...seen, declined: 0, dueSeconds: seen.activeSeconds + donate.thanksSeconds }, true);
      } else {
        const declined = seen.declined + 1;
        const wait = Math.min(donate.laterSeconds * 2 ** (declined - 1), donate.mostSeconds);
        this.keepDonate({ ...seen, declined, dueSeconds: seen.activeSeconds + wait }, true);
      }
    };
    return html`
      <div class="tour" role="dialog" aria-modal="true" aria-labelledby="donate-title" @keydown=${e => e.key === 'Escape' && close('dismissed')}>
        <div class="tour-shade" @click=${() => close('dismissed')}></div>
        <div class="tour-bubble donate-dialog">
          <div class="donate-mark" aria-hidden="true">⛽</div>
          <h2 id="donate-title">${t('Has Apex been useful?')}</h2>
          <p>${t('Apex is free and has no ads. If it found you a good road, you can buy me some fuel.')}</p>
          <div class="tour-buttons">
            <button class="tour-skip" @click=${() => close('dismissed')}>${t('Not now')}</button>
            <a class="btn primary" href=${this.account.donate.url} target="_blank" rel="noopener" @click=${() => close('clicked')}>${t('Buy me some fuel')}</a>
          </div>
        </div>
      </div>`;
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
    const stillUses = avoided.filter(([key]) => avoid?.[key] && plan?.has?.[key]).map(([, label]) => t(label.toLowerCase()));
    const link = this.link();
    const googleStops = link ? googleStopCount(link) : 0;

    return html`
      <div class="map"></div>
      <button class="locate" title=${t('My location')} @click=${() => this.locateMe()}>◎</button>
      <aside class="panel ${this.collapsed ? 'collapsed' : ''} ${this.tall ? 'tall' : ''} ${this.planning ? 'planning' : ''}">
        <div class="progress" aria-hidden="true"></div>
        <div class="grabber" role="button" aria-label=${t(this.collapsed ? 'Show' : 'Hide')} @pointerdown=${e => this.dragSheet(e)}></div>
        <header>
          <input .value=${this.route.name} placeholder=${t('Untitled route')} aria-label=${t('Route name')}
            @change=${e => this.setOption('name', e.target.value)}>
          ${this.route.id && !this.dirty
            ? html`<span class="icon saved-mark" title=${t('Saved')} aria-label=${t('Saved')}>${CHECK_ICON}</span>`
            : html`<button class="icon save" title=${t('Save route')} aria-label=${t('Save route')} ?disabled=${stops.length === 0}
                @click=${() => this.save()}>${SAVE_ICON}</button>`}
          <button class="icon new" title=${t('New route')} aria-label=${t('New route')} @click=${() => this.startNew()}>${NEW_ICON}</button>
          <button class="icon help" title=${t('How it works')} aria-label=${t('How it works')} @click=${() => this.tourTo(0)}>${HELP_ICON}</button>
          ${this.renderAccount()}
          <input class="gpx-file" type="file" accept=".gpx,application/gpx+xml" hidden
            @change=${e => (this.importGPX(e.target.files[0]), (e.target.value = ''))}>
        </header>
        <div class="body">
          <div class="mode" role="tablist">
            <button role="tab" aria-selected=${!this.advanced} @click=${() => this.setMode('simple')}>${t('Simple')}</button>
            <button role="tab" aria-selected=${this.advanced} @click=${() => this.setMode('advanced')}>${t('Advanced')}</button>
          </div>
          <div class="summary">
          <div class="totals">
            ${plan ? html`<b>${km(plan.meters)}</b><span>${duration(plan.seconds)}</span>` : nothing}
            <span>${plural(stops.length, '{n} stop', '{n} stops')}</span>
            ${this.planning ? html`<span class="working">${t(this.advanced ? 'Looking for curves' : 'Routing')}</span>` : nothing}
          </div>
          <button class="icon popover-toggle gear" title=${t('Avoid motorways, tolls or ferries')} aria-label=${t('Avoid')} aria-expanded=${this.popover === 'options'}
            @click=${() => this.togglePopover('options')}>${GEAR_ICON}</button>
          ${this.popover === 'options' ? this.renderRouteOptions() : nothing}
          <button class="icon popover-toggle gpx" title=${t('GPX file')} aria-label=${t('GPX file')} aria-haspopup="menu" aria-expanded=${this.popover === 'gpx'}
            @click=${() => this.togglePopover('gpx')}>${GPX_ICON}</button>
          ${this.popover === 'gpx' ? html`
            <div class="popover gpx-menu" role="menu">
              <button role="menuitem" ?disabled=${stops.length === 0} @click=${() => ((this.popover = null), this.exportGPX())}>${t('Export GPX')}</button>
              <button role="menuitem" @click=${() => ((this.popover = null), this.querySelector('input.gpx-file').click())}>${t('Import GPX…')}</button>
            </div>` : nothing}
          </div>
          ${this.renderCurves()}
          ${this.activeOptions().length
            ? html`<div class="chips">${this.activeOptions().map(o => html`<button class="chip-tag popover-toggle" @click=${() => this.togglePopover('options')}>${t(o)}</button>`)}</div>`
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
              ? t('Click the map to add the first stop.')
              : t('Click the map to add a stop at the end, or the line to add one in between. Drag a stop to move it.')}
            ${stops.length > 1 ? html` <a href="#" @click=${e => (e.preventDefault(), this.reverse())}>${t('Reverse')}</a>` : nothing}
          </p>
          ${googleStops > TESTED_GOOGLE_STOPS
            ? html`<p class="notice">${t('Google Maps will list {n} stops; links have been tested up to {max}.', { n: googleStops, max: TESTED_GOOGLE_STOPS })}</p>`
            : nothing}
          ${stillUses.length
            ? html`<p class="notice">${t('No way around it: the route still uses {what}.', { what: stillUses.join(t(' and ')) })}</p>`
            : nothing}
          ${this.notice ? html`<p class="notice ${this.notice.bad ? 'bad' : ''}">${this.notice.text}</p>` : nothing}
          ${this.renderSaved()}
        </div>
        <div class="actions">
          <a class="btn primary" href=${link ?? '#'} target="_blank" rel="noopener" aria-disabled=${!link} @click=${() => link && this.askDonate()}>
            ${t(this.advanced && this.planning ? 'Working out the route…' : 'Open in Google Maps')}
          </a>
          <button class="btn square share" title=${t(this.copied === 'apex' ? 'Link copied' : 'Share')} aria-label=${t('Share')}
            ?disabled=${stops.length === 0} @click=${() => this.share()}>${this.copied === 'apex' ? CHECK_ICON : SHARE_ICON}</button>
        </div>
      </aside>
      ${this.map ? this.renderLayers() : nothing}
      ${this.map ? this.renderBasemap() : nothing}
      ${this.donateAsk ? this.renderDonateAsk() : nothing}
      ${this.tour ? renderTour(this.tour.step, this.tour.rect, {
        next: () => this.tourTo(this.tour.step + 1),
        back: () => this.tourTo(this.tour.step - 1),
        close: () => this.closeTour(),
      }) : nothing}
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
          <span class="grip" title=${t('Drag to reorder')} @click=${e => e.stopPropagation()}>⠿</span>
          <span class="n">${i + 1}</span>
          <span class="label" title=${coords(s)}>
            <span class="name ${s.name ? '' : 'pending'}">${s.name || coords(s)}</span>
            ${s.detail ? html`<span class="detail">${s.detail}</span>` : nothing}
            ${s.approx ? html`<span class="approx">${t('Found by name: check it is this one')}</span>` : nothing}
          </span>
          ${!this.advanced && arriving ? html`<span class="km">${km(arriving.meters)}</span>` : nothing}
          <button class="icon remove" title=${t('Remove')} @click=${e => (e.stopPropagation(), this.removeStop(i))}>✕</button>
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
      return html`<li class="endpoint off"><button @click=${() => this.useMyLocation()}><span class="dot here"></span>${t('Start from my location')}</button></li>`;
    }
    return html`
      <li class="endpoint ${this.route.stops.length ? `out-${this.legStyle(-1)}` : ''}">
        <span class="dot here"></span>
        <span class="label"><span class="name">${t('My location')}</span><span class="detail">${t(this.here ? 'Wherever you are when you open the link' : 'Waiting for your location…')}</span></span>
        <button class="icon" title=${t('Start at stop 1 instead')} @click=${() => (this.setHere(null), this.setOption('fromHere', false))}>✕</button>
      </li>
    `;
  }

  // Coming back to the start is usually decided last, so it sits at the end of the list.
  renderEnd() {
    const stops = this.route.stops;
    if (stops.length < 2) return nothing;
    if (!this.route.roundTrip) {
      return html`<li class="endpoint off"><button @click=${() => this.setOption('roundTrip', true)}><span class="dot back">${RETURN_ICON}</span>${t('Return to the start')}</button></li>`;
    }
    const back = this.plan?.legs[stops.length - 1 + this.offset];
    return html`
      <li class="endpoint in-${this.legStyle(stops.length - 1)}">
        <span class="dot back">${RETURN_ICON}</span>
        <span class="label"><span class="name">${t('Back to {name}', { name: stops[0].name || t('stop 1') })}</span><span class="detail">${t('Round trip')}</span></span>
        ${!this.advanced && back ? html`<span class="km">${km(back.meters)}</span>` : nothing}
        <button class="icon" title=${t('End at the last stop')} @click=${() => this.setOption('roundTrip', false)}>✕</button>
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
              title=${t('How to ride this leg')} @click=${e => this.openLegMenu(menu, e.currentTarget)}>${t(label)} ▾</button>
            ${noMotorways ? html`<span class="no-motorways" title=${t('No motorways')}>${NO_MOTORWAY_ICON}</span>` : nothing}
            ${this.popover === menu ? html`
              <div class="popover leg-menu ${this.menuUp ? 'up' : ''}" role="menu">
                ${STYLES.map(([value, text]) => html`
                  <div class="menu-row">
                    <button class="dot-pick" title=${t('Colour of {kind} legs', { kind: t(text) })} aria-label=${t('Colour of {kind} legs', { kind: t(text) })}
                      aria-expanded=${this.colourEdit === value}
                      @click=${() => (this.colourEdit = this.colourEdit === value ? null : value)}><span class="style-dot ${value}"></span></button>
                    <button role="menuitemradio" aria-checked=${leg.style === value}
                      @click=${() => (this.setLeg(i, { style: value }), (this.popover = null))}>${t(text)}</button>
                  </div>
                  ${this.colourEdit === value ? this.renderSwatches(value) : nothing}`)}
                ${leg.style === 'fast'
                  ? html`<label><input type="checkbox" .checked=${leg.avoidHighways}
                      @change=${e => this.setLeg(i, { avoidHighways: e.target.checked })}> ${t('No motorways')}</label>`
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
        <span class="curves" title=${t('{deg}° turned per km', { deg: Math.round(planned.curvature) })}>
          <span class="bars">${[1, 2, 3, 4, 5].map(n => html`<i class=${n <= c.level ? 'on' : ''}></i>`)}</span>${t(c.label)}
        </span>
        ${planned.limited ? html`<span class="limited" title=${t('Out of curve searches for a moment: drawn fast without motorways, and redone with curves by itself.')}>${t('Fast for now')}</span>` : nothing}
        ${hidden
          ? html`<span class="hidden" title=${plural(hidden, '{n} hidden stop so Google Maps rides this leg as drawn; it lists it as a stop', '{n} hidden stops so Google Maps rides this leg as drawn; it lists them as stops')}>+${hidden}</span>`
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
        <span class="group">${t('Avoid')}</span>
        ${avoided.map(([key, label]) => html`
          <label><input type="checkbox" .checked=${!!avoid?.[key]} @change=${e => this.setAvoid(key, e.target.checked)}> ${t(label)}</label>`)}
        ${this.advanced ? html`<p class="hint">${t('Motorways are chosen per leg in advanced mode.')}</p>` : nothing}
      </div>
    `;
  }

  // dragSheet lets the panel on a phone be pulled down to the header, to half the screen or up to
  // nearly all of it; a tap folds or unfolds it.
  dragSheet(e) {
    const panel = this.querySelector('.panel');
    const grabber = e.currentTarget;
    grabber.setPointerCapture(e.pointerId);
    const startY = e.clientY;
    const startHeight = panel.getBoundingClientRect().height;
    const folded = panel.querySelector('header').offsetHeight + panel.querySelector('.actions').offsetHeight;
    const top = window.innerHeight * 0.88;
    let height = startHeight;
    const move = ev => {
      if (Math.abs(ev.clientY - startY) < 6 && height === startHeight) return;
      if (this.collapsed) this.collapsed = false;
      height = Math.min(Math.max(startHeight - (ev.clientY - startY), folded), top);
      panel.style.setProperty('height', `${height}px`);
      panel.style.setProperty('max-height', `${height}px`);
    };
    const up = () => {
      grabber.removeEventListener('pointermove', move);
      grabber.removeEventListener('pointerup', up);
      grabber.removeEventListener('pointercancel', up);
      panel.style.removeProperty('height');
      panel.style.removeProperty('max-height');
      if (height === startHeight) {
        this.collapsed = !this.collapsed;
        this.tall = false;
        return;
      }
      const half = window.innerHeight * 0.55;
      this.collapsed = height < (folded + half) / 2;
      this.tall = !this.collapsed && height > (half + top) / 2;
    };
    grabber.addEventListener('pointermove', move);
    grabber.addEventListener('pointerup', up);
    grabber.addEventListener('pointercancel', up);
  }

  // tourTo moves the guide to a step: it opens what the step shows, then measures it once drawn.
  async tourTo(step) {
    if (step < 0 || step >= STEPS.length) return this.closeTour();
    const s = STEPS[step];
    this.popover = null;
    this.collapsed = false;
    if (s.advanced && !this.advanced) this.setMode('advanced');
    await this.updateComplete;
    let r;
    if (s.target === 'route') {
      // The whole line, not just the stops: a twisty leg can wander well past them.
      const line = (this.plan?.legs ?? []).flatMap(l => l.path).map(([lat, lng]) => ({ lat, lng }));
      // On a wide screen the bubble goes over the top of the map, so the route keeps clear of it; on
      // a phone it sits below the route, over the panel.
      const top = window.innerWidth > 720 ? 240 : 40;
      this.map.fit(line.length ? line : this.route.stops, this.querySelector('.panel').getBoundingClientRect(), top);
      await new Promise(done => setTimeout(done, 450));
      // The drawn route itself: the box around every leg on screen.
      const boxes = [...this.querySelectorAll('path.leg')].map(p => p.getBoundingClientRect()).filter(b => b.width > 0);
      if (boxes.length) {
        const left = Math.min(...boxes.map(b => b.left));
        const top = Math.min(...boxes.map(b => b.top));
        r = { left, top, width: Math.max(...boxes.map(b => b.right)) - left, height: Math.max(...boxes.map(b => b.bottom)) - top };
        r.right = r.left + r.width;
        r.bottom = r.top + r.height;
      } else {
        // Nothing drawn yet: the open map beside or above the panel, where the clicks go.
        const panel = this.querySelector('.panel').getBoundingClientRect();
        const beside = panel.width < window.innerWidth / 2;
        r = beside
          ? { left: panel.right + 16, top: 16, right: window.innerWidth - 16, bottom: window.innerHeight - 16 }
          : { left: 16, top: 16, right: window.innerWidth - 16, bottom: panel.top - 16 };
        r.width = r.right - r.left;
        r.height = r.bottom - r.top;
      }
    } else {
      // Every part the step names, as one box: saving goes with signing in.
      const targets = s.target.split(',').map(q => this.querySelector(q.trim())).filter(Boolean);
      targets[0]?.scrollIntoView?.({ block: 'nearest' });
      const boxes = targets.map(el => el.getBoundingClientRect()).filter(b => b.width > 0);
      if (boxes.length) {
        const left = Math.min(...boxes.map(b => b.left));
        const top = Math.min(...boxes.map(b => b.top));
        r = { left, top, right: Math.max(...boxes.map(b => b.right)), bottom: Math.max(...boxes.map(b => b.bottom)) };
        r.width = r.right - r.left;
        r.height = r.bottom - r.top;
      }
    }
    const rect = r && r.width > 0 ? clip(r) : null;
    this.tour = { step, rect };
  }

  closeTour() {
    this.tour = null;
    markTourSeen();
  }

  updated() {
    if (this.tour) placeBoxes(this);
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
    const where = at === 0 ? t('Add a stop before stop 1') : at >= this.route.stops.length ? t('Add a stop at the end') : t('Add a stop between {a} and {b}', { a: at, b: at + 1 });
    return html`<button class=${`insert-here ${this.insertAt === at ? 'on' : ''}`} title=${where} aria-label=${where}
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
      <div class="swatch-strip" role="radiogroup" aria-label=${t('Colour of {kind} legs', { kind: t(STYLES.find(([v]) => v === style)[1]) })}>
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
      <button class="basemap" title=${t('Switch to {name}', { name: t(BASE_LABELS[flip]) })}
        @click=${() => (this.map.setBase(flip), this.requestUpdate())}>
        <img src=${BASE_PREVIEWS[flip]} alt=""><span>${t(BASE_LABELS[flip])}</span>
      </button>
      <button class="layers-toggle popover-toggle" title=${t('Map type')} aria-label=${t('Map type')} aria-expanded=${this.popover === 'layers'}
        @click=${() => this.togglePopover('layers')}>${LAYERS_ICON}</button>
      ${this.popover === 'layers' ? html`
        <div class="popover map-popover layers-popover">
          <h3>${t('Map type')}</h3>
          <div class="layer-choices">
            ${Object.keys(BASE_LABELS).map(base => html`
              <button aria-pressed=${this.map.base === base} @click=${() => (this.map.setBase(base), (this.popover = null))}>
                <img src=${BASE_PREVIEWS[base]} alt=""><span>${t(BASE_LABELS[base])}</span>
              </button>`)}
          </div>
        </div>` : nothing}
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
            @click=${() => (signIn === 'dev' ? this.togglePopover('account') : this.signIn())}>${t('Sign in')}</button>
          ${this.popover === 'account' ? html`
            <div class="popover account-menu" role="menu">
              <span class="menu-note">${t('Test riders (development only)')}</span>
              <button role="menuitem" @click=${() => this.signIn('a')}>${t('Rider A')}</button>
              <button role="menuitem" @click=${() => this.signIn('b')}>${t('Rider B')}</button>
            </div>` : nothing}
        </span>`;
    }
    const initial = (user.name || user.email).trim().charAt(0).toUpperCase();
    return html`
      <span class="account">
        <button class="avatar popover-toggle" title=${user.email} aria-label=${t('Account')} aria-expanded=${this.popover === 'account'}
          @click=${() => this.togglePopover('account')}>
          ${user.picture ? html`<img src=${user.picture} alt="" referrerpolicy="no-referrer">` : initial}
        </button>
        ${this.popover === 'account' ? html`
          <div class="popover account-menu" role="menu">
            <div class="who">
              ${user.name ? html`<b>${user.name}</b>` : nothing}
              <span>${user.email}</span>
            </div>
            <a role="menuitem" href="/privacy" target="_blank" rel="noopener">${t('Privacy')}</a>
            <button role="menuitem" @click=${() => ((this.popover = null), this.signOut())}>${t('Sign out')}</button>
          </div>` : nothing}
      </span>`;
  }

  renderLayers() {
    return html`
      <button class="map-button places-toggle popover-toggle" title=${t('My places')} aria-label=${t('My places')} aria-expanded=${this.popover === 'places'}
        @click=${() => this.togglePopover('places')}>${PLACES_ICON}</button>
      ${this.popover === 'places' ? html`
        <div class="popover map-popover places-popover">
          <h3>${t('My places')}</h3>
          <ul class="layer-lists">
            ${this.lists.map(l => html`
              <li>
                <label class="switch" title=${t(this.hiddenLists.has(l.id) ? 'Show on the map' : 'Hide from the map')}>
                  <input type="checkbox" role="switch" .checked=${!this.hiddenLists.has(l.id)} @change=${e => this.toggleList(l.id, e.target.checked)}>
                  <span></span>
                </label>
                <button class="colour-button" title=${t('Colour')} @click=${() => (this.pickingColour = this.pickingColour === l.id ? null : l.id)}>${this.listBadge(l)}</button>
                <span class="list-text">
                  <span class="list-name">${l.name}</span>
                  <small>${plural(l.places.length, '{n} place', '{n} places')}</small>
                </span>
                <button class="icon" title=${t('Refresh from Google Maps')} ?disabled=${this.listBusy} @click=${() => this.refreshList(l.id)}>↻</button>
                <button class="icon" title=${t('Unlink')} @click=${() => this.unlinkList(l)}>✕</button>
              </li>
              ${this.pickingColour === l.id ? html`
                <li class="palette">
                  ${COLOURS.map(c => html`<button class="swatch big colour-${c} ${(l.colour || 'teal') === c ? 'current' : ''}" title=${c} aria-label=${c}
                    @click=${() => this.setListColour(l.id, c)}></button>`)}
                </li>` : nothing}`)}
          </ul>
          <form class="add-list" @submit=${e => (e.preventDefault(), this.addList(e.target))}>
            <input name="url" type="url" required placeholder=${t('Paste a Google Maps list link')} aria-label=${t('Google Maps list link')}>
            <button class="btn primary" ?disabled=${this.listBusy}>${t(this.listBusy ? 'Adding…' : 'Add')}</button>
          </form>
          ${this.listError ? html`<p class="notice bad">${this.listError}</p>` : nothing}
          ${this.lists.length === 0
            ? html`<p class="hint">${t('In Google Maps open a list, tap Share and paste the link. It stays linked and refreshes by itself.')}</p>`
            : nothing}
          ${this.localLists && this.lists.length && this.account.signIn
            ? html`<p class="hint">${t('Kept in this browser.')} <button class="link" @click=${() => this.signIn()}>${t('Sign in')}</button> ${t('to have them on every device.')}</p>`
            : nothing}
        </div>` : nothing}
    `;
  }

  renderSaved() {
    if (this.saved.length === 0) return nothing;
    return html`
      <section class="saved">
        <h2>${t('Saved routes')}</h2>
        <ul>
          ${this.saved.map(r => html`
            <li class=${r.id === this.route.id ? 'current' : ''}>
              <button class="open" @click=${() => this.open(r)}>${r.name} <small>${plural(r.stops.length, '{n} stop', '{n} stops')}</small></button>
              <button class="icon" title=${t('Delete')} @click=${() => this.remove(r)}>✕</button>
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
  return t('Route {date}', { date: new Date().toLocaleDateString(lang === 'es' ? 'es-ES' : 'en-GB', { day: 'numeric', month: 'short' }) });
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

function donateSeen() {
  return { activeSeconds: 0, dueSeconds: 0, shownAt: 0, declined: 0, ...readJSON(DONATE_KEY) };
}

// furthest keeps the larger of each count, as the server does, so neither copy winds the other back.
function furthest(a, b) {
  return Object.fromEntries(Object.keys(a).map(k => [k, Math.max(a[k] ?? 0, b?.[k] ?? 0)]));
}

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

// clip keeps a highlighted box on screen: the map's route layer can reach far past the edges.
function clip(r) {
  const left = Math.max(r.left, 8);
  const top = Math.max(r.top, 8);
  const right = Math.min(r.right, window.innerWidth - 8);
  const bottom = Math.min(r.bottom, window.innerHeight - 8);
  return { left, top, width: Math.max(0, right - left), height: Math.max(0, bottom - top) };
}
