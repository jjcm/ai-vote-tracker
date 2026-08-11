/**
 * Makes a whole ledger row behave like a link to the bill's detail page. Each
 * row still carries a real anchor on the bill number, so keyboard and
 * assistive-technology users get there without the row handler.
 */

/** The in-app detail page for a bill. */
export function billHref(id) {
  return `/bills/${encodeURIComponent(id)}`;
}

/**
 * Wires every `tr[data-href]` inside `root`. Clicks that land on a link, a
 * button, or a text selection are left alone.
 */
export function wireRowLinks(root) {
  root.querySelectorAll('tr[data-href]').forEach((row) => {
    row.addEventListener('click', (event) => {
      if (event.target.closest('a, button')) return;
      if (window.getSelection()?.toString()) return;
      const href = row.dataset.href;
      if (event.metaKey || event.ctrlKey) window.open(href, '_blank', 'noopener');
      else location.href = href;
    });
  });
}
