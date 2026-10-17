import {debounce} from 'throttle-debounce';

function elementsCall<A extends unknown[]>(el: string | Element | Element[] | NodeListOf<Element> | JQuery, func: (...args: [Element, ...A]) => void, ...args: A) {
  if (typeof el === 'string') {
    el = document.querySelectorAll(el);
  }
  if (el instanceof Node) {
    func(el, ...args);
  } else if (el.length !== undefined) {
    // this works for: NodeList, HTMLCollection, Array, jQuery
    for (const e of el) {
      func(e, ...args);
    }
  } else {
    throw new Error('invalid argument to be shown/hidden');
  }
}

/**
 * @param force `true` to show, `false` to hide, `undefined` or omit to toggle
 */
function toggleShown(el: Element, force?: boolean) {
  if (force === true) {
    el.classList.remove('tw-hidden');
  } else if (force === false) {
    el.classList.add('tw-hidden');
  } else if (force === undefined) {
    el.classList.toggle('tw-hidden');
  } else {
    throw new Error('invalid force argument');
  }
}

/** Unhide one or more elements. */
export function showElem(el: string | Element | Element[] | NodeListOf<Element> | JQuery) {
  elementsCall(el, toggleShown, true);
}

/** Hide one or more elements. */
export function hideElem(el: string | Element | Element[] | NodeListOf<Element> | JQuery) {
  elementsCall(el, toggleShown, false);
}

/**
 * Toggle visibility of one or more elements.
 * @param force `true` to show, `false` to hide, `undefined` or omit to toggle
 */
export function toggleElem(el: string | Element | Element[] | NodeListOf<Element> | JQuery, force?: boolean) {
  elementsCall(el, toggleShown, force);
}

/** Returns `true` if the element has been forcibly hidden. */
export function isElemHidden(el: Element): boolean {
  return el.classList.contains('tw-hidden');
}

export function resetForms(el: Element) {
  queryElemChildren(el, 'form', resetForm);
}

function resetForm(form: HTMLFormElement) {
  form.reset();
}

function applyElemsCallback<E extends Element, I extends Iterable<E>>(elems: I, fn?: (e: E) => void): I {
  if (fn) {
    for (const el of elems) {
      fn(el);
    }
  }
  return elems;
}

export function queryElemSiblings(el: Element, selector: string = '*', fn?: (e: Element) => void): Element[] {
  return applyElemsCallback(Array.from(el.parentNode.children).filter((child) => child !== el && child.matches(selector)), fn);
}

/** it works like jQuery.children: only the direct children are selected */
export function queryElemChildren<K extends keyof HTMLElementTagNameMap>(parent: Element, selector: K, fn: (e: HTMLElementTagNameMap[K]) => void): NodeListOf<HTMLElementTagNameMap[K]> {
  return applyElemsCallback(parent.querySelectorAll<HTMLElementTagNameMap[K]>(`:scope > ${selector}`), fn);
}

export function queryElems<K extends keyof HTMLElementTagNameMap>(selector: K, fn: (e: HTMLElementTagNameMap[K]) => void): NodeListOf<HTMLElementTagNameMap[K]> {
  return applyElemsCallback(document.querySelectorAll(selector), fn);
}

export function onDomReady(cb: () => void) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', cb);
  } else {
    cb();
  }
}

/**
 * Checks whether an element is owned by the current document, and whether it is a document fragment or element node.
 * If it is, it means it is a "normal" element managed by us, which can be modified safely.
 */
export function isDocumentFragmentOrElementNode(el: Node): el is Element | DocumentFragment {
  try {
    return el.ownerDocument === document && el.nodeType === Node.ELEMENT_NODE || el.nodeType === Node.DOCUMENT_FRAGMENT_NODE;
  } catch {
    // in case the el is not in the same origin, then the access to nodeType would fail
    return false;
  }
}

/**
 * Autosize a textarea to fit content. Based on
 * https://github.com/github/textarea-autosize
 *
 * ---------------------------------------------------------------------
 * Copyright (c) 2018 GitHub, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 *
 * ---------------------------------------------------------------------
 */
export function autosize(textarea: HTMLTextAreaElement, {viewportMarginBottom = 0} = {}) {
  const textareaParent = textarea.parentElement;
  function createJumpPreventer() {
    const el = document.createElement('div');
    textareaParent.prepend(el, textareaParent.firstChild);
    return el;
  }
  const jumpPreventer = textareaParent.querySelector('div') || createJumpPreventer();
  let isUserResized = false;
  // lastStyleHeight and initialStyleHeight are CSS values like '100px'
  let lastMouseX: number;
  let lastMouseY: number;
  let lastStyleHeight: string;
  let initialStyleHeight: string;
  let lastLines: number;

  function onUserResize(event: MouseEvent) {
    if (isUserResized) return;
    if (lastMouseX !== event.clientX || lastMouseY !== event.clientY) {
      const newStyleHeight = textarea.style.height;
      if (lastStyleHeight && lastStyleHeight !== newStyleHeight) {
        isUserResized = true;
      }
      lastStyleHeight = newStyleHeight;
    }

    lastMouseX = event.clientX;
    lastMouseY = event.clientY;
  }

  function overflowOffset(): {top: number; bottom: number} {
    let offsetTop = 0;
    let el: HTMLElement = textarea;

    while (el !== document.body && el !== null) {
      offsetTop += el.offsetTop || 0;
      el = el.offsetParent as HTMLElement;
    }

    const top = offsetTop - document.defaultView.scrollY;
    const bottom = document.documentElement.clientHeight - (top + textarea.offsetHeight);
    return {top, bottom};
  }

  function resizeToFit() {
    if (isUserResized) return;
    if (textarea.offsetWidth <= 0 && textarea.offsetHeight <= 0) return;

    try {
      const {top, bottom} = overflowOffset();
      const isOutOfViewport = top < 0 || bottom < 0;

      const currLines = textarea.value.split('\n').length;
      const shouldResize = !isOutOfViewport || currLines < lastLines;
      if (currLines < lastLines) jumpPreventer.style.height = '0';
      lastLines = currLines;
      if (!shouldResize) return;

      const computedStyle = getComputedStyle(textarea);
      const topBorderWidth = parseFloat(computedStyle.borderTopWidth);
      const bottomBorderWidth = parseFloat(computedStyle.borderBottomWidth);
      const isBorderBox = computedStyle.boxSizing === 'border-box';
      const borderAddOn = isBorderBox ? topBorderWidth + bottomBorderWidth : 0;

      const adjustedViewportMarginBottom = Math.min(bottom, viewportMarginBottom);
      const curHeight = parseFloat(computedStyle.height);
      const maxHeight = curHeight + bottom - adjustedViewportMarginBottom;

      textarea.style.height = 'auto';
      const newHeight = Math.min(maxHeight, textarea.scrollHeight + borderAddOn);

      textarea.style.height = `${newHeight}px`;
      jumpPreventer.style.height = textarea.style.height;
      lastStyleHeight = textarea.style.height;
    } finally {
      // ensure that the textarea is fully scrolled to the end, when the cursor
      // is at the end during an input event
      if (textarea.selectionStart === textarea.selectionEnd &&
          textarea.selectionStart === textarea.value.length) {
        textarea.scrollTop = textarea.scrollHeight;
      }
    }
  }

  function onFormReset() {
    isUserResized = false;
    if (initialStyleHeight !== undefined) {
      textarea.style.height = initialStyleHeight;
    } else {
      textarea.style.removeProperty('height');
    }
  }

  textarea.addEventListener('mousemove', onUserResize);
  textarea.addEventListener('input', resizeToFit);
  textarea.form?.addEventListener('reset', onFormReset);
  initialStyleHeight = textarea.style.height ?? undefined;
  lastLines = textarea.value.split('\n').length;
  if (textarea.value) resizeToFit();

  return {
    resizeToFit,
    destroy() {
      textarea.removeEventListener('mousemove', onUserResize);
      textarea.removeEventListener('input', resizeToFit);
      textarea.form?.removeEventListener('reset', onFormReset);
    },
  };
}

export function onInputDebounce<T extends (...args: unknown[]) => unknown>(fn: T): debounce<T> {
  return debounce(300, fn);
}

/**
 * Set the `src` attribute on an element and returns a promise that resolves once the element
 * has loaded or errored. Suitable for all elements mention in:
 * https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/load_event
 */
export function loadElem(el: HTMLImageElement, src: string): Promise<boolean> {
  return new Promise((resolve) => {
    el.addEventListener('load', () => resolve(true), {once: true});
    el.addEventListener('error', () => resolve(false), {once: true});
    el.src = src;
  });
}

// some browsers like PaleMoon don't have "SubmitEvent" support, so polyfill it by a tricky method: use the last clicked button as submitter
// it can't use other transparent polyfill patches because PaleMoon also doesn't support "addEventListener(capture)"
const needSubmitEventPolyfill = typeof SubmitEvent === 'undefined';

declare global {
  interface HTMLFormElement {
    _submitter?: HTMLElement;
  }
}

export function submitEventSubmitter(e: SubmitEvent): HTMLElement {
  return needSubmitEventPolyfill ? ((e.target as HTMLFormElement)._submitter || null) : e.submitter;
}

function submitEventPolyfillListener(e: PointerEvent | FocusEvent) {
  const form = (e.target as HTMLElement).closest('form');
  if (!form) return;
  form._submitter = (e.target as HTMLElement).closest('button:not([type]), button[type="submit"], input[type="submit"]');
}

export function initSubmitEventPolyfill() {
  if (!needSubmitEventPolyfill) return;
  console.warn(`This browser doesn't have "SubmitEvent" support, use a tricky method to polyfill`);
  document.body.addEventListener('click', submitEventPolyfillListener);
  document.body.addEventListener('focus', submitEventPolyfillListener);
}

/**
 * Check if an element is visible, equivalent to jQuery's `:visible` pseudo.
 * Note: This function doesn't account for all possible visibility scenarios.
 * @param element The element to check.
 * @returns True if the element is visible.
 */
export function isElemVisible(element: HTMLElement): boolean {
  if (!element) return false;

  return Boolean(element.offsetWidth || element.offsetHeight || element.getClientRects().length);
}

export interface PastedContent {
  text: string;
  images: File[];
}

/** Extract text and images from `paste` event. */
export function getPastedContent(e: ClipboardEvent): PastedContent {
  const images: File[] = [];
  for (const item of e.clipboardData?.items ?? []) {
    if (item.type?.startsWith('image/')) {
      images.push(item.getAsFile());
    }
  }
  const text = e.clipboardData?.getData?.('text') ?? '';
  return {text, images};
}

/** Replace selected text in a textarea while preserving editor history, e.g. CTRL-Z works after this. */
export function replaceTextareaSelection(textarea: HTMLTextAreaElement, text: string) {
  const before = textarea.value.slice(0, textarea.selectionStart ?? undefined);
  const after = textarea.value.slice(textarea.selectionEnd ?? undefined);
  let success = true;

  textarea.contentEditable = 'true';
  try {
    success = document.execCommand('insertText', false, text);
  } catch {
    success = false;
  }
  textarea.contentEditable = 'false';

  if (success && !textarea.value.slice(0, textarea.selectionStart ?? undefined).endsWith(text)) {
    success = false;
  }

  if (!success) {
    textarea.value = `${before}${text}${after}`;
    textarea.dispatchEvent(new CustomEvent('change', {bubbles: true, cancelable: true}));
  }
}

/** **Warning**: Do not enter any unsanitized variables here. */
export function createElementFromHTML(htmlString: string): Element {
  const div = document.createElement('div');
  div.innerHTML = htmlString.trim();
  return div.firstElementChild;
}
