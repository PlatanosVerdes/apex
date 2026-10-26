export function km(meters) {
  return meters >= 10000 ? `${Math.round(meters / 1000)} km` : `${(meters / 1000).toFixed(1)} km`;
}

export function duration(seconds) {
  const minutes = Math.round(seconds / 60);
  const h = Math.floor(minutes / 60);
  return h ? `${h} h ${String(minutes % 60).padStart(2, '0')} min` : `${minutes} min`;
}

// shortDuration fits a leg's time on its one line: 1h15, 42 min.
export function shortDuration(seconds) {
  const minutes = Math.round(seconds / 60);
  const h = Math.floor(minutes / 60);
  return h ? `${h}h${String(minutes % 60).padStart(2, '0')}` : `${minutes} min`;
}

export function coords({ lat, lng }) {
  return `${lat.toFixed(4)}, ${lng.toFixed(4)}`;
}

const CURVES = [[40, 'Straight'], [120, 'A few bends'], [220, 'Winding'], [350, 'Mountain road'], [Infinity, 'Very twisty']];

// curves turns degrees per kilometre into a 1-5 level a rider can read at a glance.
export function curves(degreesPerKm) {
  const i = CURVES.findIndex(([below]) => degreesPerKm < below);
  return { level: i + 1, label: CURVES[i][1] };
}
