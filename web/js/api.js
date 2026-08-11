/** Thin wrapper around the Go JSON API. */

async function get(path) {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    let detail = res.statusText;
    try {
      detail = (await res.json()).error || detail;
    } catch {
      /* non-JSON error body */
    }
    throw new Error(detail);
  }
  return res.json();
}

// The homepage renders two components from one payload; memoise so they share
// a single request. `refresh` bypasses the cache when polling for late votes.
let featuredPromise = null;

export const api = {
  featured: ({ refresh = false } = {}) => {
    if (refresh || !featuredPromise) featuredPromise = get('/api/featured');
    return featuredPromise;
  },
  bill: (id) => get(`/api/bills/${encodeURIComponent(id)}`),
  alignment: () => get('/api/alignment'),
  bills: (params = {}) => {
    const query = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== '' && v !== null && v !== undefined) query.set(k, v);
    });
    const qs = query.toString();
    return get(`/api/bills${qs ? `?${qs}` : ''}`);
  },
  revote: async (id, force = false) => {
    const res = await fetch(`/api/bills/${encodeURIComponent(id)}/vote${force ? '?force=true' : ''}`, {
      method: 'POST',
    });
    if (!res.ok && res.status !== 202) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
    return res.json();
  },
};

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const MONTHS_LONG = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];

function parseDate(value) {
  if (!value) return null;
  const d = new Date(value);
  if (Number.isNaN(d.getTime()) || d.getUTCFullYear() < 1900) return null;
  return d;
}

/** "May 8, 2025" */
export function formatDate(value) {
  const d = parseDate(value);
  if (!d) return '—';
  return `${MONTHS[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}`;
}

/** "MAY 8, 2025" for the featured card. */
export function formatDateLong(value) {
  const d = parseDate(value);
  if (!d) return '';
  return `${MONTHS_LONG[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}`;
}

/** Signed one-decimal score, e.g. "+0.41" or "−0.72". */
export function formatScore(value) {
  const n = Number(value) || 0;
  const sign = n < 0 ? '\u2212' : '+';
  return `${sign}${Math.abs(n).toFixed(2)}`;
}

export function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[c]);
}

/** Removes the "S. 1264 — " style prefix some bill titles repeat. */
export function cleanTitle(title, number) {
  if (!title || !number) return title || '';
  const prefix = new RegExp(`^\\s*${number.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}\\s*[-–—:]\\s*`, 'i');
  return title.replace(prefix, '');
}
