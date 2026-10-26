import {svg} from '../svg.ts';

export function makeCodeCopyButton(): HTMLButtonElement {
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
    // suggestion and mermaid blocks are replaced by a rendered diff or diagram (see markup/suggestion.js
    // and markup/mermaid.js), so they must not get a copy button. Mermaid adds its own.
    if (/(?:^|\s)language-(?:suggestion|mermaid)(?:\s|$)/i.test(el.className)) continue;
    const parent = el.closest('pre');
    let btn = parent.nextElementSibling?.classList?.contains('code-copy') ? parent.nextElementSibling : null;
    if (!btn) {
      btn = makeCodeCopyButton();
      parent.after(btn);
    }
    // remove final trailing newline introduced during HTML rendering
    btn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));
  }
}
