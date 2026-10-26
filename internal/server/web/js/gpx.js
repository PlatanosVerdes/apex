import { routePoints } from './link.js';

// Hidden via points are written as route points of this type, so an import can tell them from stops.
const VIA_TYPE = 'shaping';
// A track becomes this many stops at most, the most Google Maps has opened from one link.
const MAX_TRACK_STOPS = 25;

// toGPX writes the stops as waypoints, the stops plus hidden via points as a route a navigator can
// follow, and, when the plan is there, the drawn line as a track.
export function toGPX(route, plan, here) {
  const name = route.name || 'Apex route';
  const stops = routePoints(route);
  const start = route.fromHere && here ? [{ ...here, name: 'My location' }] : [];
  const legs = plan?.legs ?? [];
  const rte = [];
  [...start, ...stops].forEach((s, i) => {
    if (i > 0) for (const [lat, lng] of legs[i - 1]?.via ?? []) rte.push(point('rtept', { lat, lng }, '', VIA_TYPE));
    rte.push(point('rtept', s, s.name));
  });
  const track = legs.flatMap(l => l.path).map(([lat, lng]) => `<trkpt lat="${lat}" lon="${lng}"/>`);
  return `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="Apex" xmlns="http://www.topografix.com/GPX/1/1">
<metadata><name>${xml(name)}</name></metadata>
${route.stops.map(s => point('wpt', s, s.name)).join('\n')}
<rte><name>${xml(name)}</name>
${rte.join('\n')}
</rte>
${track.length ? `<trk><name>${xml(name)}</name><trkseg>\n${track.join('\n')}\n</trkseg></trk>` : ''}
</gpx>
`;
}

function point(tag, { lat, lng }, name, type) {
  const inner = (name ? `<name>${xml(name)}</name>` : '') + (type ? `<type>${type}</type>` : '');
  return `<${tag} lat="${lat.toFixed(6)}" lon="${lng.toFixed(6)}">${inner}</${tag}>`;
}

function xml(s) {
  return String(s).replace(/[<>&"']/g, c => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;', '"': '&quot;', "'": '&apos;' })[c]);
}

// fromGPX reads stops from a GPX file: its route if it has one, else its waypoints, else points
// spread along its track. kind says which, since a track only comes back approximately.
export function fromGPX(text) {
  // Checked first: Chrome builds its XML error page with inline styles, which the page's CSP reports.
  if (!/<gpx[\s>]/.test(text)) throw new Error('not a GPX file');
  const doc = new DOMParser().parseFromString(text, 'application/xml');
  if (doc.querySelector('parsererror')) throw new Error('not a valid GPX file');
  const read = el => ({
    lat: parseFloat(el.getAttribute('lat')),
    lng: parseFloat(el.getAttribute('lon')),
    name: child(el, 'name'),
    type: child(el, 'type'),
  });
  const valid = p => Number.isFinite(p.lat) && Number.isFinite(p.lng);
  const name = child(doc.querySelector('metadata'), 'name') || child(doc.querySelector('rte, trk'), 'name');
  const clean = ({ lat, lng, name }) => ({ lat, lng, name: name || '', detail: '' });

  const rte = [...doc.getElementsByTagName('rtept')].map(read).filter(valid).filter(p => p.type !== VIA_TYPE);
  if (rte.length >= 2) return { name, kind: 'route', stops: rte.map(clean) };
  const wpt = [...doc.getElementsByTagName('wpt')].map(read).filter(valid);
  if (wpt.length >= 2) return { name, kind: 'waypoints', stops: wpt.map(clean) };
  const trk = [...doc.getElementsByTagName('trkpt')].map(read).filter(valid);
  if (trk.length >= 2) return { name, kind: 'track', stops: spread(trk).map(clean) };
  throw new Error('the file has no route, waypoints or track');
}

function child(el, tag) {
  return el?.getElementsByTagName(tag)[0]?.textContent.trim() ?? '';
}

// spread keeps the track's ends and points at even distances in between, at least 5 km apart.
function spread(track) {
  const at = [0];
  for (let i = 1; i < track.length; i++) at.push(at[i - 1] + metres(track[i - 1], track[i]));
  const total = at[at.length - 1];
  const step = Math.max(5000, total / (MAX_TRACK_STOPS - 1));
  const out = [track[0]];
  let next = step;
  for (let i = 1; i < track.length - 1; i++) {
    if (at[i] >= next && total - at[i] >= step / 2) {
      out.push(track[i]);
      next = at[i] + step;
    }
  }
  out.push(track[track.length - 1]);
  return out;
}

function metres(a, b) {
  const k = Math.cos((a.lat * Math.PI) / 180);
  return Math.hypot((a.lat - b.lat) * 111320, (a.lng - b.lng) * 111320 * k);
}
