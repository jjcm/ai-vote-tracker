import { api, escapeHTML, cleanTitle } from '../api.js';
import { billArticle, fillVerdicts, pipelineNotice } from './bill-card.js';
import { chevron } from '../icons.js';

/**
 * The /bills/{id} page: the homepage hero card opened out to full width, with
 * the bill's sponsor and source alongside the five model verdicts.
 */
class BillDetail extends HTMLElement {
  connectedCallback() {
    this.billId = billIdFromLocation();
    this.innerHTML = `${crumbs()}<div class="panel panel--message"><p>Loading bill…</p></div>`;
    this.load();
  }

  disconnectedCallback() {
    clearTimeout(this._timer);
  }

  async load() {
    if (!this.billId) {
      this.fail('No bill was named in the address.');
      return;
    }
    try {
      const data = await api.bill(this.billId);
      this.render(data);
      this.schedulePoll(data);
    } catch (err) {
      this.fail(err.message);
    }
  }

  schedulePoll(data) {
    const pending = data.bill.votes.some((v) => v.vote === 'Pending');
    if (!pending || !data.pipeline?.votingEnabled) return;
    clearTimeout(this._timer);
    this._timer = setTimeout(() => this.load(), 5000);
  }

  render(data) {
    const bill = data.bill;
    document.title = `${bill.number} — ${cleanTitle(bill.title, bill.number)} — AI Vote Tracker`;
    this.innerHTML = `
      ${crumbs()}
      ${billArticle(bill, { heading: 'h1', facts: true })}
      ${pipelineNotice(data.pipeline)}`;
    fillVerdicts(this, bill.votes);
  }

  fail(message) {
    this.innerHTML = `
      ${crumbs()}
      <div class="panel panel--message">
        <p>Could not load that bill: ${escapeHTML(message)}</p>
      </div>`;
  }
}

function crumbs() {
  return `<p class="crumbs"><a href="/bills"><span class="flip">${chevron(12)}</span> All Bills</a></p>`;
}

/** `/bills/s-1264` → `s-1264`. */
function billIdFromLocation() {
  const segments = location.pathname.split('/').filter(Boolean);
  return segments.length > 1 ? decodeURIComponent(segments[segments.length - 1]) : '';
}

customElements.define('bill-detail', BillDetail);
