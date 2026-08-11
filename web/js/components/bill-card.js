import { escapeHTML, formatDate, formatDateLong, cleanTitle } from '../api.js';
import { capitol, info } from '../icons.js';

/**
 * The hero card markup shared by the homepage feature and the bill detail
 * page: chamber kicker, bill title, status line, summary, and one verdict
 * column per model.
 *
 * `heading` picks the element for the title, so the detail page can own the
 * page's h1. `facts` adds the sponsor / policy / source block the homepage
 * leaves out.
 */
export function billArticle(bill, { heading = 'h2', facts = false } = {}) {
  const title = cleanTitle(bill.title, bill.number);
  return `
    <article class="featured">
      <div class="featured__body">
        <span class="featured__spine" aria-hidden="true"></span>
        <p class="featured__kicker">
          <span class="featured__kicker-mark">${capitol(20)}</span>
          ${escapeHTML(`${bill.chamber} Bill`)}
        </p>
        <${heading} class="featured__title">
          ${escapeHTML(bill.number)} <span class="featured__dash">–</span> ${escapeHTML(title)}
        </${heading}>
        <div class="featured__meta">
          <span class="chip">Status</span>
          <span class="featured__status">${escapeHTML(bill.status || 'Introduced')}</span>
          <time class="featured__date" datetime="${bill.updatedAt}">${formatDateLong(bill.updatedAt)}</time>
        </div>
        <div class="featured__summary">
          <p>${escapeHTML(bill.summary)}</p>
        </div>
        ${facts ? factsBlock(bill) : ''}
      </div>
      <div class="verdicts">
        <p class="verdicts__heading"><span>AI Vote Verdicts</span></p>
        <div class="verdicts__grid" data-count="${bill.votes.length}"></div>
      </div>
    </article>`;
}

/** Fills the card's empty verdict grid with one column per model verdict. */
export function fillVerdicts(root, votes) {
  const grid = root.querySelector('.verdicts__grid');
  if (!grid) return;
  grid.replaceChildren();
  votes.forEach((vote) => {
    const card = document.createElement('model-vote-card');
    card.vote = vote;
    grid.appendChild(card);
  });
}

/** Explains an empty verdict panel rather than leaving five ellipses. */
export function pipelineNotice(pipeline) {
  if (!pipeline || pipeline.votingEnabled) return '';
  return `<p class="notice">
      <span class="notice__icon" aria-hidden="true">${info(18)}</span>
      <span>No verdicts yet: set <code>OPENROUTER_KEY</code> in <code>.env</code> and restart the server to collect votes.</span>
    </p>`;
}

function factsBlock(bill) {
  const rows = [
    ['Sponsor', bill.sponsor ? `${bill.sponsor}${bill.sponsorParty ? ` (${bill.sponsorParty})` : ''}` : ''],
    ['Policy Area', bill.policyArea],
    ['Stage', bill.statusCategory],
    ['Introduced', bill.introducedDate ? formatDate(bill.introducedDate) : ''],
  ].filter(([, value]) => value);

  if (!rows.length && !bill.sourceUrl) return '';
  return `
    <div class="bill-facts">
      <dl class="bill-facts__list">
        ${rows
          .map(
            ([label, value]) =>
              `<div class="bill-facts__row"><dt>${label}</dt><dd>${escapeHTML(value)}</dd></div>`
          )
          .join('')}
      </dl>
      ${
        bill.sourceUrl
          ? `<a class="link-arrow" href="${escapeHTML(bill.sourceUrl)}" rel="noopener" target="_blank">
              View on Congress.gov &rsaquo;
            </a>`
          : ''
      }
    </div>`;
}
