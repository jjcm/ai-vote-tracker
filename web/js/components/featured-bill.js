import { api, escapeHTML, formatDateLong, cleanTitle } from '../api.js';
import { capitol } from '../icons.js';

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
    const bill = data.featured;
    const chamberLabel = `${bill.chamber} Bill`;
    const title = cleanTitle(bill.title, bill.number);
    this.innerHTML = `
      <article class="featured">
        <div class="featured__body">
          <span class="featured__spine" aria-hidden="true"></span>
          <p class="featured__kicker">
            <span class="featured__kicker-mark">${capitol(20)}</span>
            ${escapeHTML(chamberLabel)}
          </p>
          <h2 class="featured__title">
            ${escapeHTML(bill.number)} <span class="featured__dash">–</span> ${escapeHTML(title)}
          </h2>
          <div class="featured__meta">
            <span class="chip">Status</span>
            <span class="featured__status">${escapeHTML(bill.status || 'Introduced')}</span>
            <time class="featured__date" datetime="${bill.updatedAt}">${formatDateLong(bill.updatedAt)}</time>
          </div>
          <div class="featured__summary">
            <p>${escapeHTML(bill.summary)}</p>
          </div>
        </div>
        <div class="verdicts">
          <p class="verdicts__heading"><span>AI Vote Verdicts</span></p>
          <div class="verdicts__grid" data-count="${bill.votes.length}"></div>
        </div>
      </article>`;

    const grid = this.querySelector('.verdicts__grid');
    bill.votes.forEach((vote) => {
      const card = document.createElement('model-vote-card');
      card.vote = vote;
      grid.appendChild(card);
    });
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
