// The /maps/dir/a/b/c/ path form, not ?api=1&waypoints=: the documented form caps waypoints at 9
// (3 on mobile browsers) while this one opened 25 stops in the iPhone app.
// plan, when given, adds each leg's hidden via points and the avoid settings it worked out; offset
// is 1 when the plan's first point is the rider's position.
export function googleMapsLink(route, plan, offset = 0) {
  const fmt = ([lat, lng]) => `${lat.toFixed(5)},${lng.toFixed(5)}`;
  const legs = plan?.legs ?? [];
  // A stop goes in where the route meets the road, not where it was clicked: Google snaps a raw
  // point to its own nearest road, which can be a track beside the one the route takes.
  const onRoad = (s, p) => legs[p]?.path[0] ?? legs[p - 1]?.path.at(-1) ?? [s.lat, s.lng];
  const points = route.fromHere ? [''] : []; // An empty origin is the phone's location.
  routePoints(route).forEach((s, k) => {
    const p = k + offset;
    if (p > 0) points.push(...(legs[p - 1]?.via ?? []).map(fmt));
    points.push(fmt(onRoad(s, p)));
  });
  return `https://www.google.com/maps/dir/${points.join('/')}/data=${routeOptions(plan?.link ?? route.avoid ?? {})}`;
}

// The data blob Google Maps writes when its avoid boxes are ticked: each !4m/!2m counts the entries
// nested under it, !1b1 motorways, !2b1 tolls, !3b1 ferries, !3e0 driving.
export function routeOptions({ highways, tolls, ferries }) {
  const flags = [highways && '!1b1', tolls && '!2b1', ferries && '!3b1'].filter(Boolean);
  if (flags.length === 0) return '!4m2!4m1!3e0';
  const n = flags.length;
  return `!4m${n + 3}!4m${n + 2}!2m${n}${flags.join('')}!3e0`;
}

export function routePoints(route) {
  const stops = route.stops;
  return route.roundTrip && stops.length > 1 ? [...stops, stops[0]] : stops;
}

// The "paradas" Google Maps will list: every point between origin and destination, hidden ones included.
export function googleStopCount(link) {
  return new URL(link).pathname.split('/').slice(4, -2).length;
}
