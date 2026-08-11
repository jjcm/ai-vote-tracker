/**
 * Original SVG marks for the site. Everything here is drawn from primitives —
 * nothing is traced or cropped out of the design reference renders.
 */

const stroke = (d, extra = '') =>
  `<path d="${d}" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"${extra ? ' ' + extra : ''}/>`;

/** Neoclassical capitol: finial, dome, pediment, colonnade, stylobate. */
export function capitol(size = 32, className = 'icon-capitol') {
  return `<svg class="${className}" viewBox="0 0 64 64" width="${size}" height="${size}" role="img" aria-hidden="true">
    ${stroke('M32 5v6')}
    ${stroke('M22.5 25a9.5 11 0 0 1 19 0')}
    ${stroke('M19 25h26')}
    ${stroke('M17 30h30')}
    ${stroke('M7 37 32 28l25 9')}
    ${stroke('M8.5 37h47')}
    ${stroke('M14 41v11M23 41v11M32 41v11M41 41v11M50 41v11', 'stroke-width="2"')}
    ${stroke('M10 41h44')}
    ${stroke('M9 52h46')}
    ${stroke('M5.5 57.5h53')}
  </svg>`;
}

/** Laurel wreath emblem, generated leaf by leaf around two arcs. */
export function laurel(size = 44, className = 'icon-laurel') {
  const cx = 32;
  const cy = 33;
  const radius = 21;
  const leaves = [];
  for (const side of [-1, 1]) {
    for (let i = 0; i < 8; i++) {
      const t = i / 7;
      const angle = (28 + t * 118) * (Math.PI / 180);
      const x = cx + side * radius * Math.sin(angle);
      const y = cy + radius * Math.cos(angle);
      const rot = side * (90 - (angle * 180) / Math.PI) + (side > 0 ? 28 : -28);
      const scale = 0.72 + 0.3 * Math.sin(t * Math.PI);
      leaves.push(
        `<ellipse cx="${x.toFixed(2)}" cy="${y.toFixed(2)}" rx="${(2.5 * scale).toFixed(2)}" ry="${(5.4 * scale).toFixed(
          2
        )}" transform="rotate(${rot.toFixed(1)} ${x.toFixed(2)} ${y.toFixed(2)})" fill="currentColor" opacity="0.92"/>`
      );
    }
  }
  return `<svg class="${className}" viewBox="0 0 64 64" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="32" cy="32" r="29" fill="none" stroke="currentColor" stroke-width="1.1" opacity="0.55"/>
    <path d="M32 55C21 50 15.5 42 15.5 32.5c0-6.6 2.3-12 6.5-16.5" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.8"/>
    <path d="M32 55c11-5 16.5-13 16.5-22.5 0-6.6-2.3-12-6.5-16.5" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.8"/>
    ${leaves.join('')}
    ${star(0, '')}
  </svg>`;
}

/** Five-pointed star. Pass a translate/scale transform via `attrs`. */
export function star(size = 0, className = 'icon-star', attrs = 'transform="translate(32 30) scale(0.42)"') {
  const points = [];
  for (let i = 0; i < 10; i++) {
    const r = i % 2 === 0 ? 12 : 5;
    const a = (i * 36 - 90) * (Math.PI / 180);
    points.push(`${(r * Math.cos(a)).toFixed(2)},${(r * Math.sin(a)).toFixed(2)}`);
  }
  const polygon = `<polygon points="${points.join(' ')}" fill="currentColor" ${attrs}/>`;
  if (!size) return polygon;
  return `<svg class="${className}" viewBox="-14 -14 28 28" width="${size}" height="${size}" role="img" aria-hidden="true">
    <polygon points="${points.join(' ')}" fill="currentColor"/>
  </svg>`;
}

/** Doric column capital used as the medallion between the masthead rules. */
export function columnMark(size = 26, className = 'icon-column') {
  return `<svg class="${className}" viewBox="0 0 64 64" width="${size}" height="${size}" role="img" aria-hidden="true">
    ${stroke('M14 18h36', 'stroke-width="3"')}
    ${stroke('M18 24h28', 'stroke-width="2.4"')}
    ${stroke('M24 24v18M32 24v18M40 24v18', 'stroke-width="2.4"')}
    ${stroke('M18 46h28', 'stroke-width="2.4"')}
    ${stroke('M14 52h36', 'stroke-width="3"')}
  </svg>`;
}

/** Small laurel flourish that brackets section headings. */
export function flourish(direction = 'left', size = 34, className = 'icon-flourish') {
  const flip = direction === 'right' ? ' transform="scale(-1 1) translate(-64 0)"' : '';
  return `<svg class="${className}" viewBox="0 0 64 24" width="${size}" height="${(size * 24) / 64}" role="img" aria-hidden="true">
    <g${flip}>
      <path d="M2 12h30" fill="none" stroke="currentColor" stroke-width="1.2" opacity="0.7"/>
      <path d="M34 12c6-6 12-8 18-7-2 6-8 10-14 9" fill="none" stroke="currentColor" stroke-width="1.4"/>
      <path d="M38 12c5 5 11 7 17 6" fill="none" stroke="currentColor" stroke-width="1.4"/>
      <circle cx="60" cy="12" r="2" fill="currentColor"/>
    </g>
  </svg>`;
}

/** Filled verdict badge: navy check for Yes, red cross for No. */
export function verdictBadge(vote, size = 22) {
  const yes = vote === 'Yes';
  const glyph = yes
    ? '<path d="M9.2 15.4 6.4 12.6a1 1 0 0 1 1.4-1.4l1.4 1.4 4.9-4.9a1 1 0 1 1 1.4 1.4z" fill="#fff"/>'
    : '<path d="M15.6 9.1 13.5 11l2.1 2.1a1 1 0 0 1-1.4 1.4L12 12.4l-2.2 2.1a1 1 0 0 1-1.4-1.4L10.5 11 8.4 8.9a1 1 0 0 1 1.4-1.4L12 9.6l2.2-2.1a1 1 0 0 1 1.4 1.6z" fill="#fff"/>';
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="12" cy="11" r="9.4" fill="currentColor"/>
    ${glyph}
  </svg>`;
}

/** Outlined verdict mark used in the alignment agreement table. */
export function verdictMark(vote, size = 18) {
  const yes = vote === 'Yes';
  const glyph = yes
    ? '<path d="M8.2 12.2 10.7 14.7 15.9 9.3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>'
    : '<path d="M9 9l6 6M15 9l-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>';
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="12" cy="12" r="9.2" fill="none" stroke="currentColor" stroke-width="1.6" opacity="0.55"/>
    ${glyph}
  </svg>`;
}

export function pendingMark(size = 22) {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="12" cy="11" r="9.4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-dasharray="3 3" opacity="0.6"/>
  </svg>`;
}

export function chevron(size = 14) {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <path d="M9 5l7 7-7 7" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>`;
}

export function search(size = 16) {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" stroke-width="1.8"/>
    <path d="M16 16l4.5 4.5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
  </svg>`;
}

export function info(size = 18) {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
    <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.6"/>
    <path d="M12 11v6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
    <circle cx="12" cy="7.6" r="1.15" fill="currentColor"/>
  </svg>`;
}

/**
 * Model marks. The brand glyphs live as SVG files under /assets/models and are
 * painted through a CSS mask, so each one picks up the surrounding ink colour
 * and sits in the same engraved register as the rest of the page.
 *
 * Sources: Anthropic, OpenAI, DeepSeek and Google Gemini from Simple Icons
 * (CC0); Grok from @lobehub/icons-static-svg (MIT). Trademarks belong to their
 * respective owners.
 */
const MODEL_ICONS = {
  opus: '/assets/models/opus.svg',
  grok: '/assets/models/grok.svg',
  'gpt-sol': '/assets/models/gpt-sol.svg',
  deepseek: '/assets/models/deepseek.svg',
  gemini: '/assets/models/gemini.svg',
};

export function modelMark(key, size = 20) {
  const src = MODEL_ICONS[key];
  if (!src) {
    return `<svg class="model-mark" viewBox="0 0 24 24" width="${size}" height="${size}" role="img" aria-hidden="true">
      <circle cx="12" cy="12" r="8.4" fill="none" stroke="currentColor" stroke-width="1.4"/>
    </svg>`;
  }
  return `<span class="model-mark" style="--model-icon:url('${src}');--model-size:${size}px" aria-hidden="true"></span>`;
}

/** Horizontal rule with a centred star, used under page headlines. */
export function starRule(width = 120) {
  return `<svg class="star-rule" viewBox="0 0 ${width} 12" width="${width}" height="12" role="img" aria-hidden="true">
    <path d="M0 6h${width / 2 - 12}M${width / 2 + 12} 6H${width}" stroke="currentColor" stroke-width="1" opacity="0.45" stroke-dasharray="1 3"/>
    <g transform="translate(${width / 2} 6) scale(0.42)">${star(0, '', '')}</g>
  </svg>`;
}
