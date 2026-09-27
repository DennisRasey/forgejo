import {showModal} from '../modules/modal.ts';
import {POST} from '../modules/fetch.js';
import {showErrorToast} from '../modules/toast.js';

// Suggestions queued for a batch apply, keyed by comment id; the value is the (shared) apply endpoint.
const batch = new Map();

// Re-applies the batch state to the DOM. Exported so callers that re-render comment markup can resync
// otherwise re-rendered buttons revert to their default state.
export function syncSuggestionBatchUI() {
  // Starting a batch is exclusive: single applies are hidden, and the review box is replaced by the
  // batch commit/discard controls until the batch is emptied.
  const active = batch.size > 0;
  for (const btn of document.querySelectorAll('.apply-suggestion-single')) {
    btn.classList.toggle('tw-hidden', active);
  }
  for (const btn of document.querySelectorAll('.add-suggestion-batch')) {
    const inBatch = batch.has(btn.getAttribute('data-comment-id'));
    btn.textContent = btn.getAttribute(inBatch ? 'data-remove-label' : 'data-add-label');
    // solid primary while offering "Add", outlined once queued ("Remove")
    btn.classList.toggle('basic', inBatch);
  }
  document.querySelector('#review-box')?.classList.toggle('tw-hidden', active);
  const bar = document.querySelector('#suggestion-batch-bar');
  if (!bar) return;
  bar.classList.toggle('tw-hidden', !active);
  const count = bar.querySelector('.batch-count');
  if (count) count.textContent = String(batch.size);
}

// Posts the modal's comment ids + commit message as JSON; reloads on success, toasts on failure.
async function submitApply(form) {
  const action = form.getAttribute('action');
  const ids = (form.getAttribute('data-comment-ids') || '').split(',').filter(Boolean).map(Number);
  if (!action || !ids.length) return;

  // Add loading class to avoid multi click
  const okButton = form.querySelector('.ok.button');
  if (okButton?.classList.contains('is-loading')) return;
  okButton?.classList.add('is-loading');
  try {
    const resp = await POST(action, {data: {
      comment_ids: ids,
      commit_summary: form.querySelector('[name=commit_summary]').value,
      commit_message: form.querySelector('[name=commit_message]').value,
    }});
    if (resp.status === 200) {
      window.location.reload();
      return;
    }
    if (resp.status >= 400 && resp.status < 500) {
      // expected, user-facing failures carry a JSON errorMessage (or a short plain-text body for 403/413)
      const text = await resp.text();
      try {
        const data = JSON.parse(text);
        showErrorToast(data.errorMessage || `server error: ${resp.status}`, {useHtmlBody: data.renderFormat === 'html'});
      } catch {
        showErrorToast(text.trim() || `server error: ${resp.status}`);
      }
    } else {
      // 5xx returns an HTML error page; show a generic message rather than dumping it into a toast
      showErrorToast(`server error: ${resp.status}`);
    }
  } catch (e) {
    showErrorToast(`${e}`);
  }
  okButton?.classList.remove('is-loading'); // re-enable for a retry after a failure (on success we returned)
}

export function initRepoSuggestionApply() {
  document.addEventListener('click', (e) => {
    if (!(e.target instanceof Element)) return;

    const addBtn = e.target.closest('.add-suggestion-batch');
    if (addBtn) {
      const id = addBtn.getAttribute('data-comment-id');
      if (batch.has(id)) {
        batch.delete(id);
      } else {
        batch.set(id, addBtn.getAttribute('data-apply-url'));
      }
      syncSuggestionBatchUI();
      return;
    }

    const commitBtn = e.target.closest('.commit-suggestion-batch');
    if (commitBtn) {
      const modal = document.querySelector('#apply-suggestion-modal');
      const ids = Array.from(batch.keys());
      if (!modal || !ids.length) return;
      const form = modal.querySelector('form');
      form.setAttribute('action', batch.values().next().value);
      form.setAttribute('data-comment-ids', ids.join(','));
      form.querySelector('[name=commit_summary]').value = commitBtn.closest('#suggestion-batch-bar')?.getAttribute('data-default-summary') || '';
      form.querySelector('[name=commit_message]').value = '';
      showModal(modal, undefined);
      return;
    }

    if (e.target.closest('.clear-suggestion-batch')) {
      batch.clear();
      syncSuggestionBatchUI();
    }
  });

  // Submit the form
  document.querySelector('#apply-suggestion-modal form')?.addEventListener('submit', (e) => {
    e.preventDefault();
    submitApply(e.target);
  });
}
