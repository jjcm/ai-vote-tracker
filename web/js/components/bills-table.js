import { api, escapeHTML, formatDate, cleanTitle } from '../api.js';
import { modelMark } from '../icons.js';
import { billHref, wireRowLinks } from './row-link.js';

/**
 * Compact latest-bills table on the homepage: one vote glyph per model, no
 * summaries. <latest-bills limit="5">
 */
class LatestBills extends HTMLElement {
  connectedCallback() {
    this.innerHTML = '<div class="table-wrap"><p class="table-empty">Loading recent legislation…</p></div>';
    this.load();
  }

  disconnectedCallback() {
    clearTimeout(this._timer);
  }

  async load(refresh = false) {
    try {
      const data = await api.featured({ refresh });
      const limit = Number(this.getAttribute('limit')) || 5;
      this.render(data.latest.slice(0, limit), data.models);
      const pending = data.latest.slice(0, limit).some((b) => b.votes.some((v) => v.vote === 'Pending'));
      if (pending && data.pipeline?.votingEnabled) {
        clearTimeout(this._timer);
        this._timer = setTimeout(() => this.load(true), 5000);
      }
    } catch (err) {
      this.innerHTML = `<div class="table-wrap"><p class="table-empty">${escapeHTML(err.message)}</p></div>`;
    }
  }

  render(bills, models) {
    this.innerHTML = `
      <div class="table-wrap">
        <table class="ledger ledger--compact">
          <thead>
            <tr>
              <th scope="col" class="col-bill">Bill</th>
              <th scope="col" class="col-title">Title</th>
              <th scope="col" class="col-chamber">Chamber</th>
              ${models.map((m) => `<th scope="col" class="col-vote">${modelHead(m)}</th>`).join('')}
              <th scope="col" class="col-updated">Updated</th>
            </tr>
          </thead>
          <tbody>
            ${bills.map((bill) => row(bill, models)).join('')}
          </tbody>
        </table>
      </div>`;

    wireRowLinks(this);
  }
}

/** Brand glyph above the model name, so the vote columns read at a glance. */
function modelHead(model) {
  return `<span class="col-vote__model">${modelMark(model.key, 18)}<span>${escapeHTML(model.name)}</span></span>`;
}

function row(bill, models) {
  const byModel = new Map(bill.votes.map((v) => [v.modelKey, v]));
  const href = billHref(bill.id);
  return `
    <tr class="row-link" data-href="${escapeHTML(href)}">
      <td class="col-bill"><a class="bill-number" href="${escapeHTML(href)}">${escapeHTML(bill.number)}</a></td>
      <td class="col-title">${escapeHTML(cleanTitle(bill.title, bill.number))}</td>
      <td class="col-chamber"><span class="chamber chamber--${bill.chamber.toLowerCase()}">${escapeHTML(
        bill.chamber
      )}</span></td>
      ${models
        .map((m) => {
          const v = byModel.get(m.key);
          return `<td class="col-vote"><vote-badge vote="${v ? v.vote : 'Pending'}" model="${escapeHTML(
            m.name
          )}" size="24"></vote-badge></td>`;
        })
        .join('')}
      <td class="col-updated">${formatDate(bill.updatedAt)}</td>
    </tr>`;
}

customElements.define('latest-bills', LatestBills);
