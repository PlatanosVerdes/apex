// A route link carries the whole route, Google Maps style: one /lat,lng segment per stop, then
// ?n=name&m=a (advanced)&l=legs&h=leg from the rider&rt=1 (round trip)&here=1&av=letters.
// A leg is one letter: f fast, F fast without motorways, c curvy, t twisty.

const LEG_LETTERS = { f: ['fast', false], F: ['fast', true], c: ['curvy', true], t: ['twisty', true] };
const AVOID_LETTERS = { m: 'highways', t: 'tolls', f: 'ferries' };
const MAX_STOPS = 100;

function legLetter(leg = {}) {
  if (leg.style === 'curvy') return 'c';
  if (leg.style === 'twisty') return 't';
  return leg.avoidHighways ? 'F' : 'f';
}

function legFrom(letter) {
  const [style, avoidHighways] = LEG_LETTERS[letter] ?? LEG_LETTERS.f;
  return { style, avoidHighways };
}

export function toRouteLink(route, origin) {
  const points = route.stops.map(s => `${s.lat.toFixed(5)},${s.lng.toFixed(5)}`).join('/');
  const q = new URLSearchParams();
  if (route.name) q.set('n', route.name);
  if (route.mode === 'advanced') {
    q.set('m', 'a');
    q.set('l', route.stops.map(s => legLetter(s.leg)).join(''));
    if (route.fromHere && route.hereLeg) q.set('h', legLetter(route.hereLeg));
  }
  if (route.roundTrip) q.set('rt', '1');
  if (route.fromHere) q.set('here', '1');
  const avoid = Object.entries(AVOID_LETTERS).filter(([, key]) => route.avoid?.[key]).map(([letter]) => letter).join('');
  if (avoid) q.set('av', avoid);
  const query = q.toString();
  return `${origin}/r/${points}${query ? `?${query}` : ''}`;
}

// fromRouteLink reads a route back from a page address, or returns null when it holds none.
export function fromRouteLink({ pathname, search }) {
  const match = pathname.match(/^\/r\/(.+?)\/?$/);
  if (!match) return null;
  const stops = match[1].split('/').slice(0, MAX_STOPS).map(part => {
    const [lat, lng] = decodeURIComponent(part).split(',').map(Number);
    return { lat, lng };
  });
  if (!stops.length || stops.some(({ lat, lng }) => !(Math.abs(lat) <= 90 && Math.abs(lng) <= 180))) return null;
  const q = new URLSearchParams(search);
  const advanced = q.get('m') === 'a';
  const legs = q.get('l') ?? '';
  const avoid = Object.fromEntries([...(q.get('av') ?? '')].filter(l => l in AVOID_LETTERS).map(l => [AVOID_LETTERS[l], true]));
  return {
    name: (q.get('n') ?? '').slice(0, 100),
    mode: advanced ? 'advanced' : 'simple',
    stops: stops.map((s, i) => ({ ...s, name: '', ...(advanced ? { leg: legFrom(legs[i]) } : {}) })),
    roundTrip: q.get('rt') === '1',
    fromHere: q.get('here') === '1',
    ...(advanced && q.get('h') ? { hereLeg: legFrom(q.get('h')) } : {}),
    avoid,
  };
}
