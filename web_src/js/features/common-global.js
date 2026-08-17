import $ from 'jquery';
import '../vendor/jquery.are-you-sure.js';
import {clippie} from 'clippie';
import {createDropzone} from './dropzone.js';
import {showGlobalErrorMessage} from '../bootstrap.js';
import {handleGlobalEnterQuickSubmit} from './comp/QuickSubmit.js';
import {svg} from '../svg.js';
import {hideElem, showElem, toggleElem, resetForms, initSubmitEventPolyfill, submitEventSubmitter} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';
import {showTemporaryTooltip} from '../modules/tippy.js';
import {showErrorToast} from '../modules/toast.js';
import {request, POST, GET} from '../modules/fetch.js';
import '../htmx.js';
import {initTab} from '../modules/tab.ts';
import {initGlobalShowModal} from './show-modal.ts';
import {showModal} from '../modules/modal.ts';

const {appUrl, appSubUrl, i18n} = window.config;

export function initGlobalFormDirtyLeaveConfirm() {
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if (!$('.user.signin').length) {
    $('form:not(.ignore-dirty)').areYouSure();
  }
}

export function initHeadNavbarContentToggle() {
  const navbar = document.getElementById('navbar');
  const btn = document.getElementById('navbar-expand-toggle');
  if (!navbar || !btn) return;

  btn.addEventListener('click', () => {
    const isExpanded = btn.classList.contains('active');
    navbar.classList.toggle('navbar-menu-open', !isExpanded);
    btn.classList.toggle('active', !isExpanded);
  });
}

export function initFootLanguageMenu() {
  async function linkLanguageAction() {
    const $this = $(this);
    await GET($this.data('url'));
    window.location.reload();
  }

  $('.language-menu a[lang]').on('click', linkLanguageAction);
}

export function initGlobalEnterQuickSubmit() {
  $(document).on('keydown', '.js-quick-submit', (e) => {
    if (((e.ctrlKey && !e.altKey) || e.metaKey) && (e.key === 'Enter')) {
      handleGlobalEnterQuickSubmit(e.target);
      return false;
    }
  });
}

export function initGlobalButtonClickOnEnter() {
  $(document).on('keypress', 'div.ui.button,span.ui.button', (e) => {
    if (e.code === ' ' || e.code === 'Enter') {
      e.target.click();
      e.preventDefault();
    }
  });
}

// fetchActionDoRedirect does real redirection to bypass the browser's limitations of "location"
// more details are in the backend's fetch-redirect handler
function fetchActionDoRedirect(redirect) {
  const form = document.createElement('form');
  const input = document.createElement('input');
  form.method = 'post';
  form.action = `${appSubUrl}/-/fetch-redirect`;
  input.type = 'hidden';
  input.name = 'redirect';
  input.value = redirect;
  form.append(input);
  document.body.append(form);
  form.submit();
}

async function fetchActionDoRequest(actionElem, url, opt) {
  try {
    const resp = await request(url, opt);
    if (resp.status === 200) {
      let {redirect} = await resp.json();
      redirect = redirect || actionElem.getAttribute('data-redirect');
      actionElem.classList.remove('dirty'); // remove the areYouSure check before reloading
      if (redirect) {
        fetchActionDoRedirect(redirect);
      } else {
        window.location.reload();
      }
      return;
    } else if (resp.status >= 400 && resp.status < 500) {
      const text = await resp.text();
      try {
        const data = JSON.parse(text);
        // the code was quite messy, sometimes the backend uses "err", sometimes it uses "error", and even "user_error"
        // but at the moment, as a new approach, we only use "errorMessage" here, backend can use JSONError() to respond.
        if (data.errorMessage) {
          showErrorToast(data.errorMessage, {useHtmlBody: data.renderFormat === 'html'});
        } else {
          showErrorToast(`server error: ${resp.status}`);
        }
      } catch {
        // if the response is not valid JSON (e.g. plain text "Quota exceeded."), show it directly
        showErrorToast(text.trim() || `server error: ${resp.status}`);
      }
    } else {
      showErrorToast(`server error: ${resp.status}`);
    }
  } catch (e) {
    if (e.name !== 'AbortError') {
      console.error('error when doRequest', e);
      showErrorToast(`${i18n.network_error} ${e}`);
    }
  }
  actionElem.classList.remove('is-loading', 'loading-icon-2px');
}

async function formFetchAction(e) {
  if (!e.target.classList.contains('form-fetch-action')) return;

  e.preventDefault();
  const formEl = e.target;
  if (formEl.classList.contains('is-loading')) return;

  formEl.classList.add('is-loading');
  if (formEl.clientHeight < 50) {
    formEl.classList.add('loading-icon-2px');
  }

  const formMethod = formEl.getAttribute('method') || 'get';
  const formActionUrl = formEl.getAttribute('action');
  const formData = new FormData(formEl);
  const formSubmitter = submitEventSubmitter(e);
  const [submitterName, submitterValue] = [formSubmitter?.getAttribute('name'), formSubmitter?.getAttribute('value')];
  if (submitterName) {
    formData.append(submitterName, submitterValue || '');
  }

  let reqUrl = formActionUrl;
  const reqOpt = {method: formMethod.toUpperCase()};
  if (formMethod.toLowerCase() === 'get') {
    const params = new URLSearchParams();
    for (const [key, value] of formData) {
      params.append(key, value.toString());
    }
    const pos = reqUrl.indexOf('?');
    if (pos !== -1) {
      reqUrl = reqUrl.slice(0, pos);
    }
    reqUrl += `?${params.toString()}`;
  } else {
    reqOpt.body = formData;
  }

  await fetchActionDoRequest(formEl, reqUrl, reqOpt);
}

export function initGlobalCommon() {
  // Semantic UI modules.
  const $uiDropdowns = $('.ui.dropdown');

  // do not init "custom" dropdowns, "custom" dropdowns are managed by their own code.
  $uiDropdowns.filter(':not(.custom)').dropdown();

  // The "jump" means this dropdown is mainly used for "menu" purpose,
  // clicking an item will jump to somewhere else or trigger an action/function.
  // When a dropdown is used for non-refresh actions with tippy,
  // it must have this "jump" class to hide the tippy when dropdown is closed.
  $uiDropdowns.filter('.jump').dropdown({
    action: 'hide',
    onShow() {
      // hide associated tooltip while dropdown is open
      this._tippy?.hide();
      this._tippy?.disable();
    },
    onHide() {
      this._tippy?.enable();

      // hide all tippy elements of items after a while. eg: use Enter to click "Copy Link" in the Issue Context Menu
      setTimeout(() => {
        const $dropdown = $(this);
        if ($dropdown.dropdown('is hidden')) {
          $(this).find('.menu > .item').each((_, item) => {
            item._tippy?.hide();
          });
        }
      }, 2000);
    },
  });

  // Special popup-directions, prevent Fomantic from guessing the popup direction.
  // With default "direction: auto", if the viewport height is small, Fomantic would show the popup upward,
  //   if the dropdown is at the beginning of the page, then the top part would be clipped by the window view.
  //   eg: Issue List "Sort" dropdown
  // But we can not set "direction: downward" for all dropdowns, because there is a bug in dropdown menu positioning when calculating the "left" position,
  //   which would make some dropdown popups slightly shift out of the right viewport edge in some cases.
  //   eg: the "Create New Repo" menu on the navbar.
  $uiDropdowns.filter('.upward').dropdown('setting', 'direction', 'upward');
  $uiDropdowns.filter('.downward').dropdown('setting', 'direction', 'downward');

  for (const el of document.querySelectorAll('.tabular.menu')) {
    initTab(el);
  }

  initSubmitEventPolyfill();
  document.addEventListener('submit', formFetchAction);
  document.addEventListener('click', linkAction);

  // Remove stale loader when navigating back using browser nav
  window.addEventListener('pageshow', (event) => {
    // Trigger if page is restored from cache
    if (event.persisted) {
      // Find all forms that use the fetch action
      const fetchForms = document.querySelectorAll('form.form-fetch-action');

      for (const form of fetchForms) {
        // Remove form-level loading states
        form.classList.remove('is-loading');
      }
    }
  });
}

// Sometimes unrelated inputs are stored in forms for convenience, for example,
// modal inputs. To prevent them from blocking the forms for noJS clients they
// are disabled by default. TypeScript: root is HTMLElement
export function initDisabledInputs(root) {
  for (const el of root.querySelectorAll('input.js-enable[disabled]')) {
    el.removeAttribute('disabled');
  }
}

export function initGlobalDropzone() {
  for (const el of document.querySelectorAll('.dropzone')) {
    initDropzone(el);
  }
}

export async function initDropzone(dropzoneEl, zone = undefined) {
  if (!dropzoneEl) return;

  let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
  let fileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone

  const initFilePreview = (file, data, isReload = false) => {
    file.uuid = data.uuid;
    fileUuidDict[file.uuid] = {submitted: isReload};
    const input = document.createElement('input');
    input.id = data.uuid;
    input.name = 'files';
    input.type = 'hidden';
    input.value = data.uuid;
    const inputPath = document.createElement('input');
    inputPath.name = `files_fullpath[${data.uuid}]`;
    inputPath.type = 'hidden';
    inputPath.value = htmlEscape(file.fullPath || file.name);
    dropzoneEl.querySelector('.files').append(input, inputPath);

    // Create a "Copy Link" element, to conveniently copy the image
    // or file link as Markdown to the clipboard
    const copyLinkElement = document.createElement('div');
    copyLinkElement.className = 'tw-text-center';
    // The a element has a hardcoded cursor: pointer because the default is overridden by .dropzone
    copyLinkElement.innerHTML = `<a href="#" style="cursor: pointer;">${svg('octicon-copy', 14, 'copy link')} Copy link</a>`;
    copyLinkElement.addEventListener('click', async (e) => {
      e.preventDefault();
      const name = file.name.slice(0, file.name.lastIndexOf('.'));
      let fileMarkdown = `[${name}](/attachments/${file.uuid})`;
      if (file.type.startsWith('image/')) {
        fileMarkdown = `!${fileMarkdown}`;
      } else if (file.type.startsWith('video/')) {
        fileMarkdown = `<video src="/attachments/${file.uuid}" title="${htmlEscape(name)}" controls></video>`;
      }
      const success = await clippie(fileMarkdown);
      showTemporaryTooltip(e.target, success ? i18n.copy_success : i18n.copy_error);
    });
    file.previewTemplate.append(copyLinkElement);
  };
  const updateDropzoneState = () => {
    dropzoneEl.classList.toggle('dz-started', dropzoneEl.querySelector('.dz-preview'));
  };

  const dz = await createDropzone(dropzoneEl, {
    url: dropzoneEl.getAttribute('data-upload-url'),
    maxFiles: dropzoneEl.getAttribute('data-max-file'),
    maxFilesize: dropzoneEl.getAttribute('data-max-size'),
    acceptedFiles: (['*/*', ''].includes(dropzoneEl.getAttribute('data-accepts')) ? null : dropzoneEl.getAttribute('data-accepts')),
    addRemoveLinks: true,
    dictDefaultMessage: dropzoneEl.getAttribute('data-default-message'),
    dictInvalidFileType: dropzoneEl.getAttribute('data-invalid-input-type'),
    dictFileTooBig: dropzoneEl.getAttribute('data-file-too-big'),
    dictRemoveFile: dropzoneEl.getAttribute('data-remove-file'),
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
    init() {
      this.on('success', initFilePreview);
      this.on('removedfile', async (file) => {
        document.getElementById(file.uuid)?.remove();
        document.querySelector(`input[name="files_fullpath[${file.uuid}]"]`)?.remove();
        if (disableRemovedfileEvent) return;
        if (dropzoneEl.getAttribute('data-remove-url') && !fileUuidDict[file.uuid].submitted) {
          try {
            await POST(dropzoneEl.getAttribute('data-remove-url'), {data: new URLSearchParams({file: file.uuid})});
          } catch (error) {
            console.error(error);
          }
        }
        updateDropzoneState();
      });
      this.on('error', function (file, message) {
        showErrorToast(message);
        this.removeFile(file);
      });
      this.on('reload', async () => {
        if (!zone || !dz.removeAllFiles) return;
        try {
          const response = await GET(zone.getAttribute('data-attachment-url'));
          const data = await response.json();
          // do not trigger the "removedfile" event, otherwise the attachments would be deleted from server
          disableRemovedfileEvent = true;
          dz.removeAllFiles(true);
          dropzoneEl.querySelector('.files').innerHTML = '';
          for (const element of dropzoneEl.querySelectorAll('.dz-preview')) element.remove();
          fileUuidDict = {};
          disableRemovedfileEvent = false;

          for (const attachment of data) {
            attachment.type = attachment.mime_type;
            dz.emit('addedfile', attachment);
            dz.emit('complete', attachment);
            if (attachment.type.startsWith('image/')) {
              const imgSrc = `${dropzoneEl.getAttribute('data-link-url')}/${attachment.uuid}`;
              dz.emit('thumbnail', attachment, imgSrc);
            }
            initFilePreview(attachment, {uuid: attachment.uuid}, true);
            fileUuidDict[attachment.uuid] = {submitted: true};
          }
        } catch (error) {
          console.error(error);
        }
        updateDropzoneState();
      });
      this.on('create-thumbnail', (attachment, file) => {
        if (attachment.type && /image.*/.test(attachment.type)) {
          // When a new issue is created, a thumbnail cannot be fetch, so we need to create it locally.
          // The implementation is took from the dropzone library (`dropzone.js` > `_processThumbnailQueue()`)
          dz.createThumbnail(
            file,
            dz.options.thumbnailWidth,
            dz.options.thumbnailHeight,
            dz.options.thumbnailMethod,
            true,
            (dataUrl) => {
              dz.emit('thumbnail', attachment, dataUrl);
            },
          );
        }
      });
    },
  });
}

async function linkAction(e) {
  // A "link-action" can post AJAX request to its "data-url"
  // Then the browser is redirected to: the "redirect" in response, or "data-redirect" attribute, or current URL by reloading.
  const el = e.target.closest('.link-action');
  if (!el) return;

  e.preventDefault();
  const url = el.getAttribute('data-url');
  const doRequest = async () => {
    el.disabled = true;
    await fetchActionDoRequest(el, url, {method: 'POST'});
    el.disabled = false;
  };

  // If the "link-action" has "data-modal-id" attribute, a confirm modal dialog will be shown before taking action.
  const modalID = el.getAttribute('data-modal-id');
  if (!modalID) {
    await doRequest();
    return;
  }

  showModal(modalID, doRequest);
}

export function initGlobalLinkActions() {
  function showDeletePopup(e) {
    e.preventDefault();
    const $this = $(this || e.target);
    const dataArray = $this.data();

    const modalID = $this[0].getAttribute('data-modal-id');
    if (!modalID) {
      throw new Error('This button does not specify which modal it wants to open.');
    }

    const $dialog = $(`#${modalID}`);
    $dialog.find('.name').text($this.data('name'));
    for (const [key, value] of Object.entries(dataArray)) {
      if (key && key.startsWith('data')) {
        $dialog.find(`.${key}`).text(value);
      }
    }

    $dialog.modal({
      closable: false,
      onApprove: async () => {
        if ($this.data('type') === 'form') {
          document.querySelector($this.data('form')).requestSubmit();
          return;
        }
        if ($this[0].getAttribute('hx-confirm')) {
          e.detail.issueRequest(true);
          return;
        }
        const postData = new FormData();
        for (const [key, value] of Object.entries(dataArray)) {
          if (key && key.startsWith('data')) {
            postData.append(key.slice(4), value);
          }
          if (key === 'id') {
            postData.append('id', value);
          }
        }

        const response = await POST($this.data('url'), {data: postData});
        if (response.ok) {
          const data = await response.json();
          window.location.href = data.redirect;
        }
      },
    }).modal('show');
  }

  // Helpers.
  $('.delete-button').on('click', showDeletePopup);

  document.addEventListener('htmx:confirm', (e) => {
    e.preventDefault();
    // htmx:confirm is triggered for every HTMX request, even those that don't
    // have the `hx-confirm` attribute specified. To avoid opening modals for
    // those elements, check if 'e.detail.question' is empty, which contains the
    // value of the `hx-confirm` attribute.
    if (!e.detail.question) {
      e.detail.issueRequest(true);
    } else {
      showDeletePopup(e);
    }
  });
}

export function initGlobalButtons() {
  // There are many "cancel button" elements in modal dialogs, Fomantic UI expects they are button-like elements but never submit a form.
  // However, Gitea misuses the modal dialog and put the cancel buttons inside forms, so we must prevent the form submission.
  // There are a few cancel buttons in non-modal forms, and there are some dynamically created forms (eg: the "Edit Issue Content")
  document.addEventListener('click', (e) => {
    if (e.target.matches('form button.ui.cancel.button')) {
      e.preventDefault();
    }
  });

  for (const showPanelButton of document.querySelectorAll('.show-panel')) {
    showPanelButton.addEventListener('click', (e) => {
      // a '.show-panel' element can show a panel, by `data-panel="selector"`
      // if it has "toggle" class, it toggles the panel
      e.preventDefault();
      const sel = e.currentTarget.getAttribute('data-panel');
      if (e.currentTarget.classList.contains('toggle')) {
        toggleElem(sel);
      } else {
        showElem(sel);
      }
    });
  }

  for (const hidePanelButton of document.querySelectorAll('.hide-panel')) {
    hidePanelButton.addEventListener('click', (e) => {
      // a `.hide-panel` element can hide a panel, by `data-panel="selector"` or `data-panel-closest="selector"`
      e.preventDefault();
      let sel = e.currentTarget.getAttribute('data-panel');
      if (sel) {
        const element = document.querySelector(sel);
        hideElem(element);
        resetForms(element);
        return;
      }
      sel = e.currentTarget.getAttribute('data-panel-closest');
      if (sel) {
        const element = e.currentTarget.closest(sel);
        hideElem(element);
        resetForms(element);
        return;
      }
      // should never happen, otherwise there is a bug in code
      showErrorToast('Nothing to hide');
    });
  }

  initGlobalShowModal();
}

/**
 * Too many users set their ROOT_URL to wrong value, and it causes a lot of problems:
 *   * Cross-origin API request without correct cookie
 *   * Incorrect href in <a>
 *   * ...
 * So we check whether current URL starts with AppUrl(ROOT_URL).
 * If they don't match, show a warning to users.
 */
export function checkAppUrl() {
  const curUrl = window.location.href;
  // some users visit "https://domain/gitea" while appUrl is "https://domain/gitea/", there should be no warning
  if (curUrl.startsWith(appUrl) || `${curUrl}/` === appUrl) {
    return;
  }

  showGlobalErrorMessage(i18n.incorrect_root_url);
}
