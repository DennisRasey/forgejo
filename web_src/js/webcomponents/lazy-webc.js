// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {onDomReady} from '../utils/dom.ts';

/**
 * Lazy-load the promise (making it a singleton).
 * @param {()=>Promise} newPromise Promise factory.
 * @returns {()=>Promise} Singleton promise
 */
function lazyPromise(newPromise) {
  /** @type {Promise?} */
  let p;
  return () => {
    p ??= newPromise();
    return p;
  };
}

// the following web components will only be loaded if present in the page (to reduce the bundle size for infrequently used components)
const loadableComponents = {
  'model-viewer': lazyPromise(() => {
    return import(/* webpackChunkName: "model-viewer" */ '@google/model-viewer');
  }),
  'pdf-object': lazyPromise(() => {
    return import(/* webpackChunkName: "pdf-object" */ './pdf-object.js');
  }),
  'citation-information': lazyPromise(() => {
    return import(/* webpackChunkName: "citation-information" */ './citation-information.js');
  }),
};

/**
 * Replace elt with an element having the given tag.
 * @param {HTMLElement} elt The element to replace.
 * @param {string} name The tagName of the new element.
 */
function replaceTag(elt, name) {
  const successor = document.createElement(name);
  // Move the children to the successor
  while (elt.firstChild) {
    successor.append(elt.firstChild);
  }
  // Copy the attributes to the successor
  for (let index = elt.attributes.length - 1; index >= 0; --index) {
    successor.attributes.setNamedItem(elt.attributes[index].cloneNode());
  }
  // Replace elt with the successor
  elt.parentNode.replaceChild(successor, elt);
}

onDomReady(() => {
  // The lazy-webc component will replace itself with an element of the type given in the attribute tag.
  // This seems to be the best way without having to create a global mutationObserver.
  // See https://codeberg.org/forgejo/forgejo/pulls/8510 for discussion.
  window.customElements.define(
    'lazy-webc',
    class extends HTMLElement {
      connectedCallback() {
        const name = this.getAttribute('tag');
        if (loadableComponents[name]) {
          loadableComponents[name]().finally(() => {
            replaceTag(this, name);
          });
        } else {
          console.error('lazy-webc: unknown webcomponent:', name);
          replaceTag(this, name); // still replace it, maybe it was eagerly defined
        }
      }
    },
  );
});
