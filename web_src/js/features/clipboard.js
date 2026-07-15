import {showTemporaryTooltip} from '../modules/tippy.js';
import {toAbsoluteUrl} from '../utils.js';
import {clippie} from 'clippie';

const {copy_success, copy_error} = window.config.i18n;

// Enable clipboard copy from HTML attributes. These properties are supported:
// - data-clipboard-text: Direct text to copy
// - data-clipboard-target: Holds a selector for an element whose content is copied. For
//   <input> and <textarea> the value is copied, for any other element its text content.
// - data-clipboard-text-type: When set to 'url' will convert relative to absolute urls
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', async (e) => {
    const target = e.target.closest('[data-clipboard-text], [data-clipboard-target]');
    if (!target) return;

    e.preventDefault();

    let text = target.getAttribute('data-clipboard-text');
    if (!text) {
      const source = document.querySelector(target.getAttribute('data-clipboard-target'));
      text = source?.value ?? source?.textContent;
    }

    if (text && target.getAttribute('data-clipboard-text-type') === 'url') {
      text = toAbsoluteUrl(text);
    }

    if (text) {
      const success = await clippie(text);
      let tippyTarget = target;

      // If successful, auto-close parent dropdown, change tippy target to opener
      const dropdownParent = target.closest('details.dropdown');
      if (success && dropdownParent) {
        dropdownParent.removeAttribute('open');
        tippyTarget = dropdownParent.querySelector('summary');
      }

      showTemporaryTooltip(tippyTarget, success ? copy_success : copy_error);
    }
  });
}
