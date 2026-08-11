import { api, escapeHTML, formatDate, cleanTitle } from '../api.js';
import { search as searchIcon, chevron, info, modelMark } from '../icons.js';
import { billHref, wireRowLinks } from './row-link.js';

const PER_PAGE = 8;

/**
 * The /bills listing: keyword search, chamber / model / status filters, a
 * per-model vote column set, and pagination.
 */
class BillsBrowser extends HTMLElement {
  connectedCallback() {
    const params = new URLSearchParams(location.search);
    this.state = {
      q: params.get('q') || '',
      chamber: params.get('chamber') || '',
      model: params.get('model') || '',
      status: params.get('status') || '',
      page: Number(params.get('page')) || 1,
    };
    this.models = [];
    this.renderShell();
    this.load();
  }

  disconnectedCallback() {
    clearTimeout(this._poll);
    clearTimeout(this._debounce);
  }

  renderShell() {
    this.innerHTML = `
      <form class="filters" role="search">
        <label class="field field--search">
          <span class="visually-hidden">Search bills</span>
          <input type="search" name="q" placeholder="Search bills or keywords…" autocomplete="off" />
          <span class="field__icon" aria-hidden="true">${searchIcon(16)}</span>
        </label>
        <label class="field field--select">
          <span class="visually-hidden">Chamber</span>
          <select name="chamber">
            <option value="">All Chambers</option>
            <option value="Senate">Senate</option>
            <option value="House">House</option>
          </select>
        </label>
        <label class="field field--select">
          <span class="visually-hidden">Model</span>
          <select name="model"><option value="">All Models</option></select>
        </label>
        <label class="field field--select">
          <span class="visually-hidden">Status</span>
          <select name="status"><option value="">All Statuses</option></select>
        </label>
        <button type="button" class="btn btn--ghost" data-reset>Reset Filters</button>
      </form>
      <div class="table-wrap" data-results>
        <p class="table-empty">Loading legislation…</p>
      </div>
      <div class="pagination" data-pagination hidden></div>
      <p class="notice">
        <span class="notice__icon" aria-hidden="true">${info(18)}</span>
        <span data-notice>Model votes are generated per bill and cached; they may change when a bill is re-run.</span>
      </p>`;

    const form = this.querySelector('.filters');
    form.addEventListener('submit', (e) => e.preventDefault());
    form.querySelector('[name=q]').addEventListener('input', (e) => {
      clearTimeout(this._debounce);
      const value = e.target.value;
      this._debounce = setTimeout(() => this.update({ q: value, page: 1 }), 220);
    });
    ['chamber', 'model', 'status'].forEach((name) => {
      form.querySelector(`[name=${name}]`).addEventListener('change', (e) => this.update({ [name]: e.target.value, page: 1 }));
    });
    form.querySelector('[data-reset]').addEventListener('click', () => {
      this.update({ q: '', chamber: '', model: '', status: '', page: 1 });
    });
  }

  update(patch) {
    Object.assign(this.state, patch);
    this.syncControls();
    this.syncURL();
    this.load();
  }

  syncControls() {
    const form = this.querySelector('.filters');
    form.querySelector('[name=q]').value = this.state.q;
    form.querySelector('[name=chamber]').value = this.state.chamber;
    form.querySelector('[name=model]').value = this.state.model;
    form.querySelector('[name=status]').value = this.state.status;
  }

  syncURL() {
    const params = new URLSearchParams();
    Object.entries(this.state).forEach(([k, v]) => {
      if (v && !(k === 'page' && v === 1)) params.set(k, v);
    });
    const qs = params.toString();
    history.replaceState(null, '', qs ? `?${qs}` : location.pathname);
  }

  async load() {
    try {
      const data = await api.bills({
        q: this.state.q,
        chamber: this.state.chamber,
        status: this.state.status,
        page: this.state.page,
        perPage: PER_PAGE,
      });
      this.models = data.models;
      this.populateSelects(data);
      this.renderResults(data);

      const pending = data.bills.some((b) => b.votes.some((v) => v.vote === 'Pending'));
      clearTimeout(this._poll);
      if (pending && data.pipeline?.votingEnabled) this._poll = setTimeout(() => this.load(), 6000);
    } catch (err) {
      this.querySelector('[data-results]').innerHTML = `<p class="table-empty">${escapeHTML(err.message)}</p>`;
    }
  }

  populateSelects(data) {
    const modelSelect = this.querySelector('[name=model]');
    if (modelSelect.options.length <= 1) {
      data.models.forEach((m) => modelSelect.add(new Option(m.name, m.key)));
    }
    const statusSelect = this.querySelector('[name=status]');
    if (statusSelect.options.length <= 1 && data.statuses) {
      data.statuses.forEach((s) => statusSelect.add(new Option(s, s)));
    }
    this.syncControls();

    if (data.pipeline && !data.pipeline.votingEnabled) {
      this.querySelector('[data-notice]').textContent =
        'OPENROUTER_KEY is not set, so no model verdicts have been collected yet.';
    } else if (data.pipeline?.source === 'seed') {
      this.querySelector('[data-notice]').textContent =
        'Bills come from the built-in sample corpus; set CONGRESS_API_KEY for live Congress.gov legislation.';
    }
  }

  renderResults(data) {
    // "All Models" shows every column; picking one narrows the table to it.
    const columns = this.state.model ? this.models.filter((m) => m.key === this.state.model) : this.models;
    const results = this.querySelector('[data-results]');

    if (!data.bills.length) {
      results.innerHTML = '<p class="table-empty">No bills match those filters.</p>';
      this.renderPagination(data);
      return;
    }

    results.innerHTML = `
      <table class="ledger ledger--detail">
        <thead>
          <tr>
            <th scope="col" class="col-bill">Bill</th>
            <th scope="col" class="col-title">Title</th>
            <th scope="col" class="col-chamber">Chamber</th>
            ${columns
              .map(
                (m) =>
                  `<th scope="col" class="col-vote"><span class="col-vote__model">${modelMark(m.key, 18)}<span>${escapeHTML(
                    m.name
                  )}</span></span></th>`
              )
              .join('')}
            <th scope="col" class="col-updated">Updated</th>
            <th scope="col" class="col-go"><span class="visually-hidden">Open</span></th>
          </tr>
        </thead>
        <tbody>
          ${data.bills.map((bill) => detailRow(bill, columns)).join('')}
        </tbody>
      </table>`;

    wireRowLinks(results);
    this.renderPagination(data);
  }

  renderPagination(data) {
    const el = this.querySelector('[data-pagination]');
    const from = data.total === 0 ? 0 : (data.page - 1) * data.perPage + 1;
    const to = Math.min(data.page * data.perPage, data.total);
    const pages = pageList(data.page, data.totalPages);

    el.hidden = false;
    el.innerHTML = `
      <p class="pagination__count">Showing ${from} to ${to} of ${data.total} bill${data.total === 1 ? '' : 's'}</p>
      <div class="pagination__pages">
        <button class="page-btn" data-page="${data.page - 1}" ${data.page <= 1 ? 'disabled' : ''} aria-label="Previous page">
          <span class="flip">${chevron(15)}</span>
        </button>
        ${pages
          .map((p) =>
            p === '…'
              ? '<span class="page-gap">…</span>'
              : `<button class="page-btn${p === data.page ? ' is-current' : ''}" data-page="${p}">${p}</button>`
          )
          .join('')}
        <button class="page-btn" data-page="${data.page + 1}" ${
          data.page >= data.totalPages ? 'disabled' : ''
        } aria-label="Next page">${chevron(15)}</button>
      </div>`;

    el.querySelectorAll('[data-page]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const page = Number(btn.dataset.page);
        if (page >= 1 && page <= data.totalPages && page !== data.page) {
          this.update({ page });
          this.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
      });
    });
  }
}

function detailRow(bill, columns) {
  const byModel = new Map(bill.votes.map((v) => [v.modelKey, v]));
  const href = billHref(bill.id);
  return `
    <tr class="row-link" data-href="${escapeHTML(href)}">
      <td class="col-bill"><a class="bill-number" href="${escapeHTML(href)}">${escapeHTML(bill.number)}</a></td>
      <td class="col-title">
        <span class="ledger__title">${escapeHTML(cleanTitle(bill.title, bill.number))}</span>
        <span class="ledger__summary">${escapeHTML(bill.summary)}</span>
      </td>
      <td class="col-chamber"><span class="chamber chamber--${bill.chamber.toLowerCase()}">${escapeHTML(
        bill.chamber
      )}</span></td>
      ${columns
        .map((m) => {
          const v = byModel.get(m.key);
          return `<td class="col-vote"><vote-badge vote="${v ? v.vote : 'Pending'}" model="${escapeHTML(
            m.name
          )}" size="24" label></vote-badge></td>`;
        })
        .join('')}
      <td class="col-updated">${formatDate(bill.updatedAt)}</td>
      <td class="col-go"><a class="row-go" href="${escapeHTML(href)}" aria-label="Open ${escapeHTML(
        bill.number
      )}">${chevron(15)}</a></td>
    </tr>`;
}

function pageList(current, total) {
  if (total <= 6) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set([1, 2, total, current, current - 1, current + 1]);
  const sorted = [...pages].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);
  const out = [];
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push('…');
    out.push(p);
  });
  return out;
}

customElements.define('bills-browser', BillsBrowser);
