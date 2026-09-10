import {svg} from '../svg.js';

export function makeCodeCopyButton() {
  const button = document.createElement('button');
  button.classList.add('code-copy', 'ui', 'button');
  button.innerHTML = svg('octicon-copy');
  return button;
}

export function renderCodeCopy() {
  const els = document.querySelectorAll('.markup .code-block code');
  if (!els.length) return;

  for (const el of els) {
    if (!el.textContent) continue;
    let btn = el.nextElementSibling?.classList?.contains('code-copy') ? el.nextElementSibling : null;
    if (!btn) {
      btn = makeCodeCopyButton();
      el.after(btn);
    }
    // remove final trailing newline introduced during HTML rendering
    btn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));
  }
}
