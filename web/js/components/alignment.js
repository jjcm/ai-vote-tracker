import { api, escapeHTML, formatScore, cleanTitle } from '../api.js';
import { modelMark, chevron } from '../icons.js';

const TICKS = [
  { value: -1, label: 'Liberal' },
  { value: -0.5, label: 'Lean Liberal' },
  { value: 0, label: 'Center' },
  { value: 0.5, label: 'Lean Conservative' },
  { value: 1, label: 'Conservative' },
];

/**
 * Everything on /alignment: the -1..+1 spectrum, the model snapshot cards, the
 * methodology panel, and the recent-bill agreement grid. One fetch drives all
 * four so the numbers can never disagree with each other.
 */
class AlignmentReport extends HTMLElement {
  connectedCallback() {
    this.innerHTML = '<p class="table-empty">Computing alignment…</p>';
    this.load();
  }

  disconnectedCallback() {
    clearTimeout(this._poll);
  }

  async load() {
    try {
      const data = await api.alignment();
      this.render(data);
      const pending = data.recentBills.some((b) => b.votes.some((v) => v.vote === 'Pending'));
      clearTimeout(this._poll);
      if (pending && data.pipeline?.votingEnabled) this._poll = setTimeout(() => this.load(), 6000);
    } catch (err) {
      this.innerHTML = `<p class="table-empty">${escapeHTML(err.message)}</p>`;
    }
  }

  render(data) {
    this.innerHTML = `
      ${spectrumPanel(data)}
      <section-heading label="Model Snapshots"></section-heading>
      <div class="snapshot-grid">${data.models.map(snapshotCard).join('')}</div>
      ${methodologyPanel(data)}
      ${agreementPanel(data)}
      <div class="alignment-foot">
        <p class="alignment-foot__note">Alignment scores update as models vote on more legislation.</p>
        <a class="link-arrow" href="#methodology">View full methodology ${chevron(12)}</a>
      </div>`;
  }
}

function positionFor(score) {
  return ((Number(score) + 1) / 2) * 100;
}

function spectrumPanel(data) {
  // Stack overlapping markers so two close scores stay readable.
  const sorted = [...data.models].sort((a, b) => a.score - b.score);
  let lastPos = -Infinity;
  let level = 0;
  let deepest = 0;
  const markers = sorted.map((m) => {
    const pos = positionFor(m.score);
    level = pos - lastPos < 9 ? level + 1 : 0;
    deepest = Math.max(deepest, level);
    lastPos = pos;
    return `
      <div class="spectrum__marker spectrum__marker--${m.tone}" style="left:${pos.toFixed(2)}%;--level:${level}">
        <span class="spectrum__dot"></span>
        <span class="spectrum__value">${formatScore(m.score)}</span>
        <span class="spectrum__name">${escapeHTML(m.name)}</span>
      </div>`;
  });

  return `
    <section class="panel panel--spectrum">
      <h2 class="panel__title">Overall Political Alignment</h2>
      <p class="spectrum__legend">
        <span class="spectrum__legend-left">Left (Liberal)</span>
        <span class="spectrum__legend-arrow" aria-hidden="true">&#10229;&nbsp;&#10230;</span>
        <span class="spectrum__legend-right">Right (Conservative)</span>
      </p>
      <div class="spectrum">
        <div class="spectrum__ticks">
          ${TICKS.map(
            (t) => `<div class="spectrum__tick" style="left:${positionFor(t.value)}%">
              <span class="spectrum__tick-value">${t.value === 0 ? '0' : formatScore(t.value).replace('.00', '.0')}</span>
              <span class="spectrum__tick-label">${t.label}</span>
            </div>`
          ).join('')}
        </div>
        <div class="spectrum__axis" style="--levels:${deepest}">
          <span class="spectrum__axis-line"></span>
          <span class="spectrum__center" aria-hidden="true"></span>
          ${TICKS.map((t) => `<span class="spectrum__notch" style="left:${positionFor(t.value)}%"></span>`).join('')}
          ${markers.join('')}
        </div>
      </div>
    </section>`;
}

function snapshotCard(m) {
  return `
    <article class="snapshot snapshot--${m.tone}">
      <header class="snapshot__head">
        <span class="snapshot__icon">${modelMark(m.key, 26)}</span>
        <h3 class="snapshot__name">${escapeHTML(m.name)}</h3>
      </header>
      <p class="snapshot__label">Position Score</p>
      <p class="snapshot__score">${formatScore(m.score)}</p>
      <p class="snapshot__label">Alignment</p>
      <p class="snapshot__alignment">${escapeHTML(m.label)}</p>
      <p class="snapshot__summary">${escapeHTML(m.summary)}</p>
    </article>`;
}

function methodologyPanel(data) {
  return `
    <section class="panel panel--method" id="methodology">
      <div class="method">
        <h2 class="panel__title panel__title--left">How We Calculate Alignment</h2>
        <p class="method__copy">
          Each model votes “Yes” or “No” on every bill we track. Each bill carries an ideology score from
          −1.0 (progressive) to +1.0 (conservative). A Yes vote moves a model toward the bill’s score and a No vote
          moves it away; the weighted average of those movements is the model’s overall alignment. Scores close to 0
          are centrist, negative scores lean liberal, positive scores lean conservative.
          ${data.billsScored} of ${data.billsTotal} tracked bills currently carry an ideology score.
        </p>
        <div class="formula">
          <span class="formula__box"><b>Model Votes</b><i>Yes / No</i></span>
          <span class="formula__op">&times;</span>
          <span class="formula__box"><b>Bill Ideology Score</b><i>−1.0 to +1.0</i></span>
          <span class="formula__op">=</span>
          <span class="formula__box formula__box--result"><b>Overall Alignment Score</b><i>−1.0 to +1.0</i></span>
        </div>
      </div>
      <div class="guide">
        <h3 class="guide__title">Score Guide</h3>
        <ul class="guide__list">
          ${data.bands
            .map(
              (b) => `<li class="guide__item">
                <span class="guide__dot guide__dot--${b.tone}"></span>
                <span class="guide__range">${formatScore(b.min).replace('.00', '.0')} to ${formatScore(b.max).replace(
                '.00',
                '.0'
              )}</span>
                <span class="guide__label">${escapeHTML(b.label)}</span>
              </li>`
            )
            .join('')}
        </ul>
      </div>
    </section>`;
}

function agreementPanel(data) {
  const models = data.catalog;
  return `
    <section class="panel panel--agreement">
      <h2 class="panel__title panel__title--left">Recent Bill Agreement Overview</h2>
      <div class="table-wrap">
        <table class="ledger ledger--agreement">
          <thead>
            <tr>
              <th scope="col">Bill</th>
              ${models
                .map(
                  (m) =>
                    `<th scope="col" class="col-vote"><span class="agreement__model">${modelMark(
                      m.key,
                      17
                    )}${escapeHTML(m.name)}</span></th>`
                )
                .join('')}
            </tr>
          </thead>
          <tbody>
            ${data.recentBills
              .map((bill) => {
                const byModel = new Map(bill.votes.map((v) => [v.modelKey, v]));
                return `<tr>
                  <td class="col-title">
                    <span class="bill-number">${escapeHTML(bill.number)}</span>
                    <span class="agreement__dash">–</span>
                    ${escapeHTML(cleanTitle(bill.title, bill.number))}
                  </td>
                  ${models
                    .map((m) => {
                      const v = byModel.get(m.key);
                      return `<td class="col-vote"><vote-badge variant="outline" vote="${
                        v ? v.vote : 'Pending'
                      }" model="${escapeHTML(m.name)}" label></vote-badge></td>`;
                    })
                    .join('')}
                </tr>`;
              })
              .join('')}
          </tbody>
        </table>
      </div>
    </section>`;
}

customElements.define('alignment-report', AlignmentReport);
