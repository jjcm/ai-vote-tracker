import { verdictBadge, verdictMark, pendingMark, modelMark } from '../icons.js';
import { escapeHTML } from '../api.js';

const DECIDED = new Set(['Yes', 'No']);

/**
 * <vote-badge vote="Yes" label variant="filled|outline" size="22">
 * Filled navy check / red cross for the bill tables, outlined green check /
 * red cross for the alignment agreement table.
 */
class VoteBadge extends HTMLElement {
  static get observedAttributes() {
    return ['vote', 'label', 'variant', 'size'];
  }

  attributeChangedCallback() {
    if (this.isConnected) this.render();
  }

  connectedCallback() {
    this.render();
  }

  render() {
    const vote = this.getAttribute('vote') || 'Pending';
    const variant = this.getAttribute('variant') || 'filled';
    const size = Number(this.getAttribute('size')) || (variant === 'outline' ? 18 : 22);
    const showLabel = this.hasAttribute('label');
    const decided = DECIDED.has(vote);
    const tone = decided ? (vote === 'Yes' ? 'yes' : 'no') : vote === 'Error' ? 'error' : 'pending';

    let glyph;
    if (!decided) glyph = pendingMark(size);
    else if (variant === 'outline') glyph = verdictMark(vote, size);
    else glyph = verdictBadge(vote, size);

    const caption = decided ? vote : vote === 'Error' ? 'Unavailable' : 'Pending';
    this.className = `vote-badge vote-badge--${tone} vote-badge--${variant}`;
    this.innerHTML = `<span class="vote-badge__glyph">${glyph}</span>${
      showLabel ? `<span class="vote-badge__label">${caption}</span>` : ''
    }`;
    this.setAttribute('title', `${this.getAttribute('model') || 'Model'}: ${caption}`);
  }
}

/** <model-mark key="opus" name="Opus"> — icon plus small-caps model name. */
class ModelMark extends HTMLElement {
  connectedCallback() {
    const key = this.getAttribute('key') || 'opus';
    const name = this.getAttribute('name') || '';
    const size = Number(this.getAttribute('size')) || 20;
    this.className = 'model-mark-row';
    this.innerHTML = `<span class="model-mark-row__icon">${modelMark(key, size)}</span>${
      name ? `<span class="model-mark-row__name">${escapeHTML(name)}</span>` : ''
    }`;
  }
}

/**
 * One column of the featured card's verdict panel: model mark, the large
 * Yes/No, a star divider, and the model's one-sentence rationale.
 */
class ModelVoteCard extends HTMLElement {
  connectedCallback() {
    this.render();
  }

  set vote(value) {
    this._vote = value;
    if (this.isConnected) this.render();
  }

  render() {
    const v = this._vote || {
      modelKey: this.getAttribute('model-key'),
      modelName: this.getAttribute('model-name'),
      vote: this.getAttribute('vote') || 'Pending',
      reason: this.getAttribute('reason') || '',
    };
    const decided = DECIDED.has(v.vote);
    const tone = decided ? (v.vote === 'Yes' ? 'yes' : 'no') : 'pending';
    const verdict = decided ? v.vote : v.vote === 'Error' ? '—' : '…';
    let reason = v.reason;
    if (!decided) {
      reason = v.vote === 'Error' ? 'This model could not be reached for this bill.' : 'Awaiting this model’s verdict.';
    }

    this.className = `verdict verdict--${tone}`;
    this.innerHTML = `
      <div class="verdict__model">
        <span class="verdict__icon">${modelMark(v.modelKey, 20)}</span>
        <span class="verdict__name">${escapeHTML(v.modelName || '')}</span>
      </div>
      <p class="verdict__value">${verdict}</p>
      <span class="verdict__star" aria-hidden="true"></span>
      <p class="verdict__reason">${escapeHTML(reason)}</p>
      ${memo(v)}`;
  }
}

/**
 * The pros and cons this model wrote for itself before voting, folded away
 * until asked for. The verdict sentence is the headline; this is the working.
 */
function memo(vote) {
  const pros = vote.pros || [];
  const cons = vote.cons || [];
  if (!pros.length && !cons.length) return '';
  return `
    <details class="verdict__memo">
      <summary>For &amp; Against</summary>
      ${list('For', pros)}
      ${list('Against', cons)}
    </details>`;
}

function list(heading, entries) {
  if (!entries.length) return '';
  return `
    <p class="verdict__memo-heading">${heading}</p>
    <ul class="verdict__memo-list">
      ${entries.map((e) => `<li>${escapeHTML(e)}</li>`).join('')}
    </ul>`;
}

customElements.define('vote-badge', VoteBadge);
customElements.define('model-mark', ModelMark);
customElements.define('model-vote-card', ModelVoteCard);
