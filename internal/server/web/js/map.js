const L = window.L;

const point = ({ lat, lng }) => ({ lat, lng });

const BASE_KEY = 'apex.base';
const esri = path => `https://server.arcgisonline.com/ArcGIS/rest/services/${path}/MapServer/tile/{z}/{y}/{x}`;
const esriCredit = 'Tiles &copy; Esri';

function baseLayers() {
  return {
    Roads: L.tileLayer(esri('World_Street_Map'), { maxZoom: 19, attribution: `${esriCredit}, HERE, Garmin, &copy; OpenStreetMap` }),
    Satellite: L.layerGroup([
      L.tileLayer(esri('World_Imagery'), { maxZoom: 19, attribution: `${esriCredit}, Maxar, Earthstar Geographics` }),
      L.tileLayer(esri('Reference/World_Transportation'), { maxZoom: 19 }),
    ]),
  };
}

// One tile per base map (zoom 11 over the Llobregat valley) shown as its thumbnail in the picker.
export const BASE_PREVIEWS = {
  Roads: esri('World_Street_Map').replace('{z}/{y}/{x}', '11/763/1035'),
  Satellite: esri('World_Imagery').replace('{z}/{y}/{x}', '11/763/1035'),
};

function storedBase(layers) {
  try {
    const name = localStorage.getItem(BASE_KEY);
    if (name in layers) return name;
  } catch {}
  return 'Roads';
}

// RouteMap owns the Leaflet map; the app tells it what to draw and hears back what the rider did.
export class RouteMap {
  constructor(element, { add, insert, move, select, preview, neighbours }) {
    this.handlers = { add, insert, move, select, preview, neighbours };
    this.map = L.map(element, { zoomControl: false }).setView([39.62, 2.95], 9);
    L.control.zoom({ position: 'topright' }).addTo(this.map);
    this.bases = baseLayers();
    this.base = storedBase(this.bases);
    this.bases[this.base].addTo(this.map);
    this.places = L.layerGroup().addTo(this.map);
    this.lines = L.layerGroup().addTo(this.map);
    this.pins = L.layerGroup().addTo(this.map);
    this.markers = [];
    this.map.on('click', e => this.dragging || this.handlers.add(point(e.latlng)));
  }

  setStops(stops, selected) {
    this.pins.clearLayers();
    this.markers = stops.map((s, i) => {
      const classes = ['stop-pin', i === 0 && 'first', i === stops.length - 1 && i > 0 && 'last', i === selected && 'selected'];
      const marker = L.marker([s.lat, s.lng], {
        draggable: true,
        autoPan: true,
        // Above the places from the linked lists, which share the marker pane.
        zIndexOffset: 1000,
        icon: L.divIcon({ className: classes.filter(Boolean).join(' '), html: String(i + 1), iconSize: [26, 26] }),
      });
      // While a stop is dragged, the legs either side of it are redrawn along the roads as it moves.
      let live;
      marker.on('dragstart', () => {
        live = this.livePreview('fast');
        this.map.getContainer().classList.add('moving-stop');
      });
      marker.on('drag', () => {
        const { prev, next } = this.handlers.neighbours(i);
        live.update([prev, point(marker.getLatLng()), next].filter(Boolean));
      });
      marker.on('dragend', () => {
        live.stop();
        this.map.getContainer().classList.remove('moving-stop');
        this.handlers.move(i, point(marker.getLatLng()));
      });
      marker.on('click', () => this.handlers.select(i));
      return marker.addTo(this.pins);
    });
  }

  setBase(name) {
    if (!(name in this.bases) || name === this.base) return;
    this.bases[this.base].remove();
    this.bases[name].addTo(this.map);
    this.base = name;
    try {
      localStorage.setItem(BASE_KEY, name);
    } catch {}
  }

  // lists are the linked Google Maps lists to show, each drawn in its own colour and emoji.
  setPlaces(lists, onAdd) {
    this.places.clearLayers();
    for (const list of lists) {
      const colour = `colour-${list.colour || 'teal'}`;
      for (const place of list.places) {
        const marker = list.icon
          ? L.marker([place.lat, place.lng], { icon: L.divIcon({ className: '', html: poiIcon(list.icon, colour), iconSize: [26, 26] }), bubblingMouseEvents: false })
          : L.circleMarker([place.lat, place.lng], { className: `poi ${colour}`, radius: 7, bubblingMouseEvents: false });
        marker.bindTooltip(place.name, { direction: 'top', offset: [0, -6] });
        marker.bindPopup(() => placeCard(place, list.name, () => {
          marker.closePopup();
          onAdd(place);
        }));
        marker.on('popupopen', () => marker.closeTooltip());
        marker.addTo(this.places);
      }
    }
  }

  setHere(here) {
    this.hereMarker?.remove();
    this.hereMarker = here && L.circleMarker([here.lat, here.lng], { className: 'here', radius: 8, interactive: false }).addTo(this.map);
  }

  // styles[i] colours leg i; a leg's hidden via points show as small dots.
  setLegs(legs, styles) {
    this.lines.clearLayers();
    legs.forEach((leg, i) => {
      const line = L.polyline(leg.path, { className: `leg ${styles[i] ?? 'fast'} ${leg.limited ? 'limited' : ''}`, weight: 5, bubblingMouseEvents: false });
      line.on('click', e => {
        L.DomEvent.stop(e);
        if (!this.dragging) this.handlers.insert(i, point(e.latlng));
      });
      line.addTo(this.lines);
      line.getElement().addEventListener('pointerdown', ev => this.dragLeg(ev, i, leg, styles[i] ?? 'fast'));
      for (const [lat, lng] of leg.via ?? []) {
        L.circleMarker([lat, lng], { className: 'via', radius: 4, interactive: false }).addTo(this.lines);
      }
    });
  }

  // dragLeg lets the rider pull a leg onto other roads, as in Google Maps: a dashed guide follows the
  // pointer and dropping adds a stop there. A press that does not move stays a click.
  dragLeg(ev, i, leg, style) {
    if (ev.button !== 0) return;
    ev.stopPropagation();
    const from = leg.path[0];
    const to = leg.path[leg.path.length - 1];
    const start = [ev.clientX, ev.clientY];
    let ghost, guide, live;
    this.map.dragging.disable();
    const move = e => {
      if (!ghost && Math.hypot(e.clientX - start[0], e.clientY - start[1]) < 5) return;
      const at = this.map.mouseEventToLatLng(e);
      if (!ghost) {
        this.dragging = true;
        guide = L.polyline([from, at, to], { className: `drag-guide ${style}`, weight: 4, interactive: false }).addTo(this.map);
        live = this.livePreview(style, () => guide.setStyle({ opacity: 0 }));
        ghost = L.circleMarker(at, { className: `drag-ghost ${style}`, radius: 8, interactive: false }).addTo(this.map);
      }
      ghost.setLatLng(at);
      guide.setLatLngs([from, at, to]);
      live.update([{ lat: from[0], lng: from[1] }, point(at), { lat: to[0], lng: to[1] }]);
    };
    const end = e => {
      document.removeEventListener('pointermove', move);
      document.removeEventListener('pointerup', end);
      document.removeEventListener('pointercancel', end);
      this.map.dragging.enable();
      if (!ghost) return;
      live.stop();
      ghost.remove();
      guide.remove();
      ghost = null;
      if (e.type === 'pointerup') this.handlers.insert(i, point(this.map.mouseEventToLatLng(e)));
      // The click that follows the release must not add another stop.
      setTimeout(() => (this.dragging = false));
    };
    document.addEventListener('pointermove', move);
    document.addEventListener('pointerup', end);
    document.addEventListener('pointercancel', end);
  }

  // livePreview draws the road route through points that keep changing. It asks one request at a time;
  // while one is out only the newest points wait, so the line keeps up without flooding the server.
  livePreview(style, onFirst) {
    const road = L.polyline([], { className: `drag-road ${style}`, weight: 6, interactive: false }).addTo(this.map);
    let latest, inFlight, abort, stopped, shown;
    const send = async () => {
      if (inFlight || !latest || stopped) return;
      const points = latest;
      latest = null;
      inFlight = true;
      abort = new AbortController();
      try {
        const plan = await this.handlers.preview(points, abort.signal);
        if (!stopped) {
          road.setLatLngs(plan.legs.flatMap(l => l.path));
          if (!shown) onFirst?.();
          shown = true;
        }
      } catch {}
      inFlight = false;
      send();
    };
    return {
      update: points => {
        latest = points;
        send();
      },
      stop: () => {
        stopped = true;
        abort?.abort();
        road.remove();
      },
    };
  }

  // setQuota tells the map how short the rider is of curve searches: the route beats faster.
  setQuota(level) {
    const classes = this.map.getContainer().classList;
    classes.remove('quota-low', 'quota-very-low', 'quota-out');
    if (level) classes.add(`quota-${level}`);
  }

  // setPending marks the drawn route as being recalculated.
  setPending(pending) {
    this.map.getContainer().classList.toggle('planning', pending);
  }

  // covered is the panel's rectangle, kept clear so no stop ends up under it.
  fit(stops, covered) {
    if (stops.length === 0) return;
    const size = this.map.getSize();
    const beside = covered.width < size.x / 2;
    const options = {
      paddingTopLeft: [beside ? covered.right + 40 : 40, 40],
      paddingBottomRight: [40, beside ? 40 : size.y - covered.top + 40],
      maxZoom: 12,
    };
    this.map.fitBounds(L.latLngBounds(stops.map(s => [s.lat, s.lng])), options);
  }

  focus(stop) {
    this.map.panTo([stop.lat, stop.lng]);
  }

  locate() {
    this.map.locate({ setView: true, maxZoom: 12 });
  }
}

function poiIcon(emoji, colour) {
  const badge = document.createElement('span');
  badge.className = `poi-icon ${colour}`;
  badge.textContent = emoji;
  return badge;
}

// Built with DOM calls rather than an HTML string: names come from Google and the page allows no inline script.
function placeCard(place, listName, onAdd) {
  const card = document.createElement('div');
  card.className = 'place-card';
  const title = document.createElement('strong');
  title.textContent = place.name;
  card.append(title);
  for (const [text, cls] of [[place.address !== place.name && place.address, 'address'], [place.note, 'note'], [listName, 'from']]) {
    if (!text) continue;
    const line = document.createElement('span');
    line.className = cls;
    line.textContent = text;
    card.append(line);
  }
  const add = document.createElement('button');
  add.className = 'btn primary';
  add.textContent = 'Add to route';
  add.addEventListener('click', onAdd);
  card.append(add);
  return card;
}
