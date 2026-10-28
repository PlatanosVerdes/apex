import { html, nothing } from '../vendor/lit-3.3.3.js';
import { t } from './i18n.js';

const SEEN_KEY = 'apex.tourSeen';

// Each step points at a part of the page and says what it does; the first shows the route or the map.
export const STEPS = [
  { target: 'route', title: 'Draw your route', text: 'Click the map to add stops. Drag the line to take other roads; the route redraws along them as you go.' },
  { target: '.stops', title: 'Your stops', text: 'Each stop you add shows here with its number. Drag it on the map to move it, drag ⠿ to change the order, and ✕ takes it away.' },
  { target: '.search', title: 'Find a place', text: 'Search a town, a pass or one of your places. The + on a leg puts it between two stops.' },
  { target: '.mode', title: 'Ride each leg your way', text: 'In Advanced, choose each leg: fast, curvy or twisty. Tap the dot next to each kind to pick its colour.', advanced: true },
  { target: '.places-toggle', title: 'Your places', text: 'Paste a Google Maps list and its places show on the map, ready to add to a route.' },
  { target: '.panel header .save, .panel header .saved-mark, .panel header .account', title: 'Keep your routes', text: 'Sign in with Google and save your routes: they follow you between the phone and the computer.' },
  { target: '.actions', title: 'Ride it', text: 'Open the route in Google Maps to navigate with the phone, or share it with the button beside it.' },
];

export function tourSeen() {
  try {
    return localStorage.getItem(SEEN_KEY) === '1';
  } catch {
    return true;
  }
}

export function markTourSeen() {
  try {
    localStorage.setItem(SEEN_KEY, '1');
  } catch {}
}

// renderTour draws the highlight around the step's target and the bubble beside it. rect is the
// target's box, measured by the caller after the page has settled.
export function renderTour(step, rect, { next, back, close }) {
  const s = STEPS[step];
  const pad = 6;
  const box = rect && { left: rect.left - pad, top: rect.top - pad, width: rect.width + pad * 2, height: rect.height + pad * 2 };
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const bubbleW = Math.min(320, vw - 32);
  let pos = { left: (vw - bubbleW) / 2, top: vh / 2 - 90 };
  if (box) {
    const below = box.top + box.height + 12;
    const roomBelow = vh - below;
    // Below the target when it fits, else above it, else inside its top edge (the map step).
    const above = box.top - 190 - 12;
    const top = roomBelow > 190 ? below : above >= 16 ? above : box.top + 12;
    const left = Math.min(Math.max(16, box.left + box.width / 2 - bubbleW / 2), vw - bubbleW - 16);
    pos = { left, top };
  }
  const last = step === STEPS.length - 1;
  return html`
    <div class="tour" role="dialog" aria-modal="true" aria-labelledby="tour-title">
      ${box
        ? html`<div class="tour-hole" style-box=${JSON.stringify(box)}></div>`
        : html`<div class="tour-shade"></div>`}
      <div class="tour-bubble" style-box=${JSON.stringify({ left: pos.left, top: pos.top, width: bubbleW })}>
        <div class="tour-count">${step + 1} / ${STEPS.length}</div>
        <h2 id="tour-title">${t(s.title)}</h2>
        <p>${t(s.text)}</p>
        ${last ? html`<a class="tour-privacy" href="/privacy" target="_blank" rel="noopener">${t('Privacy')}</a>` : nothing}
        <div class="tour-buttons">
          <button class="tour-skip" @click=${close}>${t(last ? 'Close' : 'Skip')}</button>
          ${step > 0 && !last ? html`<button class="btn" @click=${back}>${t('Back')}</button>` : nothing}
          <button class="btn primary" @click=${last ? close : next}>${t(last ? 'Start riding' : 'Next')}</button>
        </div>
      </div>
    </div>
  `;
}

// placeBoxes sets each element's position from its style-box attribute through the CSSOM: the page's
// CSP refuses style attributes written in the HTML.
export function placeBoxes(root) {
  for (const el of root.querySelectorAll('[style-box]')) {
    const b = JSON.parse(el.getAttribute('style-box'));
    for (const [k, v] of Object.entries(b)) el.style.setProperty(k, `${v}px`);
  }
}
