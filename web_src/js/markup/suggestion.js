// This module moves each rendered diff into the place of its
// matching code block, so no diff/escaping/highlighting logic lives on the client.
//
// A comment carries at most one suggestion, so each comment's single `.suggestion-diff`
// replaces the (first) suggestion code block of that comment's body. A suggestion block
// with no matching diff (previous side, outdated, plain comment) is left untouched and
// renders as ordinary code.

const suggestionClassRe = /(?:^|\s)language-suggestion(?:\s|$)/i;

function suggestionCodeBlocks(contentEl) {
  const blocks = [];
  for (const code of contentEl.querySelectorAll('pre.code-block > code')) {
    if (suggestionClassRe.test(code.className)) {
      blocks.push(code.parentElement);
    }
  }
  return blocks;
}

export function renderSuggestions(container = document) {
  for (const holder of container.querySelectorAll('.suggestion-diff[data-comment-id]:not([data-render-done])')) {
    const commentId = holder.getAttribute('data-comment-id');
    // getElementById takes a literal id (no CSS escaping), correct for "issuecomment-<n>-content".
    const content = document.getElementById(`issuecomment-${commentId}-content`);
    if (!content) continue;

    // A comment carries at most one suggestion, so swap in its first suggestion code block.
    const [pre] = suggestionCodeBlocks(content);
    if (!pre) continue;

    pre.replaceWith(holder);
    holder.classList.remove('tw-hidden');
    // Mark done only after a successful swap, so an early call (before the code block
    // is in the DOM) can still render it on a later pass.
    holder.setAttribute('data-render-done', 'true');
  }
}
