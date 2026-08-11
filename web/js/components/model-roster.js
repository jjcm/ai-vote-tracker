import { escapeHTML } from '../api.js';
import { modelMark } from '../icons.js';

/** The five models on the bench, read from the server's catalog. */
class ModelRoster extends HTMLElement {
  async connectedCallback() {
    this.className = 'model-roster';
    try {
      const res = await fetch('/api/models', { headers: { Accept: 'application/json' } });
      const { models } = await res.json();
      this.innerHTML = models
        .map(
          (m) => `<div class="model-roster__item">
            ${modelMark(m.key, 26)}
            <span class="model-roster__name">${escapeHTML(m.name)}</span>
            <span class="model-roster__id">${escapeHTML(m.openRouterId)}</span>
          </div>`
        )
        .join('');
    } catch {
      this.innerHTML = '<p class="table-empty">Model roster unavailable.</p>';
    }
  }
}

customElements.define('model-roster', ModelRoster);
