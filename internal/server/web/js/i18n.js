// The page speaks Spanish to a browser set to Spanish and English to every other. Texts are keyed by
// their English, with {name} for what is filled in.

export const lang = (navigator.languages?.[0] || navigator.language || 'en').toLowerCase().startsWith('es') ? 'es' : 'en';

export function t(text, vars = {}) {
  const s = (lang === 'es' && es[text]) || text;
  return s.replace(/\{(\w+)\}/g, (_, k) => (k in vars ? String(vars[k]) : `{${k}}`));
}

// plural picks the singular or plural text for n, both keyed by their English.
export function plural(n, one, many, vars = {}) {
  return t(n === 1 ? one : many, { n, ...vars });
}

const es = {
  // Mode, totals and legs
  Simple: 'Simple',
  Advanced: 'Avanzado',
  '{n} stop': '{n} parada',
  '{n} stops': '{n} paradas',
  Routing: 'Calculando',
  'Looking for curves': 'Buscando curvas',
  Fast: 'Rápida',
  Curvy: 'Curvas',
  Twisty: 'Muchas curvas',
  'How to ride this leg': 'Cómo hacer este tramo',
  'No motorways': 'Sin autopistas',
  'No tolls': 'Sin peajes',
  'No ferries': 'Sin ferris',
  Motorways: 'Autopistas',
  Tolls: 'Peajes',
  Ferries: 'Ferris',
  Avoid: 'Evitar',
  'Avoid motorways, tolls or ferries': 'Evitar autopistas, peajes o ferris',
  'Motorways are chosen per leg in advanced mode.': 'En modo avanzado las autopistas se eligen en cada tramo.',
  'Colour of {kind} legs': 'Color de los tramos «{kind}»',
  Straight: 'Recta',
  'A few bends': 'Algunas curvas',
  Winding: 'Con curvas',
  'Mountain road': 'De montaña',
  'Very twisty': 'Muy revirada',
  '{deg}° turned per km': '{deg}° de giro por km',
  '{n} hidden stop so Google Maps rides this leg as drawn; it lists it as a stop': '{n} parada oculta para que Google Maps haga este tramo tal cual; la verás como parada',
  '{n} hidden stops so Google Maps rides this leg as drawn; it lists them as stops': '{n} paradas ocultas para que Google Maps haga este tramo tal cual; las verás como paradas',
  'Fast for now': 'Rápida de momento',
  'Out of curve searches for a moment: drawn fast without motorways, and redone with curves by itself.':
    'Sin búsquedas de curvas por un momento: va por la rápida sin autopistas y se rehará con curvas sola.',
  'Only {n} curve search left': 'Solo queda {n} búsqueda de curvas',
  'Only {n} curve searches left': 'Solo quedan {n} búsquedas de curvas',
  '{left} of {max} curve searches left': 'Quedan {left} de {max} búsquedas de curvas',
  'Searching for curves asks the public routing server many times, so each rider gets a few at a time; they come back by themselves.':
    'Buscar curvas consulta muchas veces al servidor público de rutas, así que cada uno tiene unas pocas; se recargan solas.',

  // Stops
  'Start from my location': 'Salir desde mi ubicación',
  'My location': 'Mi ubicación',
  'Wherever you are when you open the link': 'Donde estés al abrir el enlace',
  'Waiting for your location…': 'Esperando tu ubicación…',
  'Start at stop 1 instead': 'Salir desde la parada 1',
  'Return to the start': 'Volver al inicio',
  'Back to {name}': 'Vuelta a {name}',
  'stop 1': 'la parada 1',
  'Round trip': 'Ida y vuelta',
  'End at the last stop': 'Terminar en la última parada',
  'Drag to reorder': 'Arrastra para reordenar',
  Remove: 'Quitar',
  'Add a stop before stop 1': 'Añadir una parada antes de la 1',
  'Add a stop at the end': 'Añadir una parada al final',
  'Add a stop between {a} and {b}': 'Añadir una parada entre la {a} y la {b}',
  'Click the map to add the first stop.': 'Haz clic en el mapa para añadir la primera parada.',
  'Click the map to add a stop at the end, or the line to add one in between. Drag a stop to move it.':
    'Haz clic en el mapa para añadir una parada al final, o en la línea para meterla en medio. Arrastra una parada para moverla.',
  Reverse: 'Invertir',
  '{name} added as stop {n}': '{name} añadida como parada {n}',
  'Location not allowed, so the route starts at stop 1.': 'Sin permiso de ubicación: la ruta sale desde la parada 1.',
  'Location not allowed. Allow it in the browser settings.': 'Sin permiso de ubicación. Actívalo en los ajustes del navegador.',
  'Could not find your location.': 'No se ha podido obtener tu ubicación.',

  // Search
  'Search a place': 'Buscar un sitio',
  'Search a place to add': 'Busca un sitio para añadir',
  'Search where to start': 'Busca por dónde empezar',
  'Add before stop 1': 'Añadir antes de la parada 1',
  'Add at the end': 'Añadir al final',
  'Add between {a} and {b}': 'Añadir entre la {a} y la {b}',
  Cancel: 'Cancelar',

  // Header, saving, sharing
  'Untitled route': 'Ruta sin nombre',
  'Route name': 'Nombre de la ruta',
  Saved: 'Guardada',
  'Save route': 'Guardar ruta',
  'New route': 'Ruta nueva',
  'GPX file': 'Archivo GPX',
  'For a GPS or another app': 'Para un GPS u otra app',
  Export: 'Exportar',
  'Import…': 'Importar…',
  Show: 'Mostrar',
  Hide: 'Ocultar',
  'Open in Google Maps': 'Abrir en Google Maps',
  'Working out the route…': 'Calculando la ruta…',
  Share: 'Compartir',
  Copy: 'Copiar',
  'Link to this route': 'Enlace a esta ruta',
  'Google Maps link': 'Enlace de Google Maps',
  'To navigate with the phone': 'Para navegar con el móvil',
  '{what} copied': '{what} copiado',
  'Coordinates copied': 'Coordenadas copiadas',
  'Copy coordinates': 'Copiar coordenadas',
  'Add a stop here': 'Añadir una parada aquí',
  Link: 'Enlace',
  'Copy the link': 'Copia el enlace',
  'Apex route': 'Ruta de Apex',
  'Saved routes': 'Rutas guardadas',
  Delete: 'Borrar',
  'Delete "{name}"?': '¿Borrar «{name}»?',
  'Discard the unsaved changes to this route?': '¿Descartar los cambios sin guardar de esta ruta?',
  'Not saved: {err}': 'No se ha guardado: {err}',
  'Saved routes unavailable: {err}': 'Las rutas guardadas no están disponibles: {err}',
  'Saving routes needs an account, and this server has no sign-in set up.':
    'Para guardar rutas hace falta una cuenta, y este servidor no tiene el acceso configurado.',
  'Route opened from a link. Save it to keep your own copy.': 'Ruta abierta desde un enlace. Guárdala para tener tu copia.',
  'Route {date}': 'Ruta {date}',
  'Could not draw the route: {err}': 'No se ha podido calcular la ruta: {err}',
  'Imported {n} stop': 'Importada {n} parada',
  'Imported {n} stops': 'Importadas {n} paradas',
  'Track imported as {n} stops along it; check the line follows it': 'Trazado importado como {n} paradas; comprueba que la línea lo sigue',
  'Could not import {file}: {err}': 'No se ha podido importar {file}: {err}',

  // Account
  'Sign in': 'Iniciar sesión',
  'Sign out': 'Cerrar sesión',
  'Signed out': 'Sesión cerrada',
  Account: 'Cuenta',
  'Signing in did not work. Try again.': 'No se ha podido iniciar sesión. Inténtalo de nuevo.',
  'Test riders (development only)': 'Usuarios de prueba (solo desarrollo)',
  'Rider A': 'Usuario A',
  'Rider B': 'Usuario B',

  // Map and places
  Map: 'Mapa',
  Satellite: 'Satélite',
  'Map type': 'Tipo de mapa',
  'Switch to {name}': 'Cambiar a {name}',
  'My places': 'Mis sitios',
  'Kept in this browser.': 'Guardadas en este navegador.',
  'to have them on every device.': 'para tenerlas en todos tus dispositivos.',
  'Show on the map': 'Mostrar en el mapa',
  'Hide from the map': 'Ocultar del mapa',
  Colour: 'Color',
  'Refresh from Google Maps': 'Actualizar desde Google Maps',
  Unlink: 'Desvincular',
  'Paste a Google Maps list link': 'Pega el enlace de una lista de Google Maps',
  'Google Maps list link': 'Enlace de una lista de Google Maps',
  Add: 'Añadir',
  'Adding…': 'Añadiendo…',
  'In Google Maps open a list, tap Share and paste the link. It stays linked and refreshes by itself.':
    'En Google Maps abre una lista, pulsa Compartir y pega aquí el enlace. Queda vinculada y se actualiza sola.',
  '{n} place': '{n} sitio',
  '{n} places': '{n} sitios',
  'Linked "{name}" with {n} places': 'Vinculada «{name}» con {n} sitios',
  '"{name}" is up to date': '«{name}» está al día',
  'Stop showing "{name}"? The list stays in Google Maps.': '¿Dejar de mostrar «{name}»? La lista sigue en Google Maps.',
  'Could not link the list: {err}': 'No se ha podido vincular la lista: {err}',
  'Could not refresh: {err}': 'No se ha podido actualizar: {err}',
  'Could not change the colour: {err}': 'No se ha podido cambiar el color: {err}',
  'Your Google lists are unavailable: {err}': 'Tus listas de Google no están disponibles: {err}',
  'Add to route': 'Añadir a la ruta',
  // Guide
  'How it works': 'Cómo funciona',
  'Draw your route': 'Dibuja tu ruta',
  'Find a place': 'Busca un sitio',
  'Search a town, a pass or one of your places. The + on a leg puts it between two stops.':
    'Busca un pueblo, un puerto o uno de tus sitios. El + de un tramo lo mete entre dos paradas.',
  'Ride each leg your way': 'Cada tramo a tu manera',
  'In Advanced, choose each leg: fast, curvy or twisty. Tap the dot next to each kind to pick its colour.':
    'En Avanzado eliges cada tramo: rápida, curvas o muchas curvas. Toca el punto de cada tipo para cambiar su color.',
  'Your places': 'Tus sitios',
  'Paste a Google Maps list and its places show on the map, ready to add to a route.':
    'Pega una lista de Google Maps y sus sitios salen en el mapa, listos para añadirlos a una ruta.',
  'Keep your routes': 'Guarda tus rutas',
  'Sign in with Google and save your routes: they follow you between the phone and the computer.':
    'Inicia sesión con Google y guarda tus rutas: te siguen del móvil al ordenador.',
  'Ride it': 'A rodar',
  'Open the route in Google Maps to navigate with the phone, or share it with the button beside it.':
    'Abre la ruta en Google Maps para navegar con el móvil, o compártela con el botón de al lado.',
  Next: 'Siguiente',
  Back: 'Atrás',
  Skip: 'Saltar',
  Close: 'Cerrar',
  'Start riding': 'Empezar',
  'Your stops': 'Tus paradas',
  'Each stop you add shows here with its number. Drag it on the map to move it, drag ⠿ to change the order, and ✕ takes it away.':
    'Cada parada que añadas sale aquí con su número. Arrástrala en el mapa para moverla, arrastra ⠿ para cambiar el orden, y la ✕ la quita.',
  'Click the map to add stops. Drag the line to take other roads; the route redraws along them as you go.':
    'Haz clic en el mapa para añadir paradas. Arrastra la línea para ir por otras carreteras; la ruta se redibuja por ellas mientras la mueves.',
  'Google Maps will list {n} stops; links have been tested up to {max}.': 'Google Maps mostrará {n} paradas; los enlaces se han probado hasta {max}.',
  'No way around it: the route still uses {what}.': 'No hay otra forma: la ruta sigue usando {what}.',
  ' and ': ' y ',
  motorways: 'autopistas',
  tolls: 'peajes',
  ferries: 'ferris',
};
