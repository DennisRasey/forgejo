import htmx from 'htmx.org';
import {showErrorToast} from './modules/toast.ts';

// https://github.com/bigskysoftware/idiomorph#htmx
import 'idiomorph/dist/idiomorph-ext.js';

// https://htmx.org/reference/#config
htmx.config.requestClass = 'is-loading';
htmx.config.scrollIntoViewOnBoost = false;

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event: CustomEvent) => {
  // TODO: add translations
  showErrorToast(`Network error when calling ${event.detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event: CustomEvent) => {
  // hide any previous flash message to avoid confusions (in case the
  // error toast would have been shown over a success/info message)
  const flashMsgDiv = document.getElementById('flash-message');
  if (flashMsgDiv) {
    flashMsgDiv.textContent = '';
    flashMsgDiv.className = '';
  }
  // TODO: add translations
  showErrorToast(`Error ${event.detail.xhr.status} when calling ${event.detail.requestConfig.path}: ${event.detail.xhr.responseText}`);
});
