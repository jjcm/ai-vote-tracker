import { api, escapeHTML } from '../api.js';
import { billArticle, fillVerdicts, pipelineNotice } from './bill-card.js';

/**
 * The homepage hero card: chamber kicker, bill title, status line, summary,
 * and one verdict column per model.
 */
class FeaturedBill extends HTMLElement {
  connectedCallback() {
    this.innerHTML = skeleton();
    this.load();
  }

  disconnectedCallback() {
    clearTimeout(this._timer);
  }

  async load(refresh = false) {
    try {
      const data = await api.featured({ refresh });
      this.render(data);
      this.schedulePoll(data);
    } catch (err) {
      this.innerHTML = `<div class="panel panel--message"><p>Could not load the featured bill: ${escapeHTML(
        err.message
      )}</p></div>`;
    }
  }

  // Verdicts arrive asynchronously on a cold start; keep refreshing until the
  // panel is complete.
  schedulePoll(data) {
    const bill = data.featured;
    const pending = bill.votes.some((v) => v.vote === 'Pending');
    if (!pending || !data.pipeline?.votingEnabled) return;
    clearTimeout(this._timer);
    this._timer = setTimeout(() => this.load(true), 5000);
  }

  render(data) {
    this.innerHTML = `${billArticle(data.featured)}${pipelineNotice(data.pipeline)}`;
    fillVerdicts(this, data.featured.votes);
  }
}

function skeleton() {
  return `<article class="featured featured--loading">
      <div class="featured__body">
        <span class="featured__spine" aria-hidden="true"></span>
        <p class="featured__kicker">Loading legislation…</p>
        <h2 class="featured__title">&nbsp;</h2>
      </div>
    </article>`;
}

customElements.define('featured-bill', FeaturedBill);
