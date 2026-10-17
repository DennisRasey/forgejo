// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {onDomReady} from '../utils/dom.ts';

/**
 * Lazy-load the promise (making it a singleton).
 * @param newPromise Promise factory.
 * @returns Singleton promise
 */
function lazyPromise<T>(newPromise: () => Promise<T>): () => Promise<T> {
  let p: Promise<T>;
  return () => {
    p ??= newPromise();
    return p;
  };
}

// the following web components will only be loaded if present in the page (to reduce the bundle size for infrequently used components)
// NOTE: tsconfig "module" must be "esnext" or equivalent
const loadableComponents = {
  'model-viewer': lazyPromise(() => {
    // FIXME: this causes an error when WebGL context isn't available. We may need to do some detecting on our own for this case.
    return import(/* webpackChunkName: "model-viewer" */ '@google/model-viewer');
  }),
  'pdf-object': lazyPromise(() => {
    return import(/* webpackChunkName: "pdf-object" */ './pdf-object.ts');
  }),
  'citation-information': lazyPromise(() => {
    return import(/* webpackChunkName: "citation-information" */ './citation-information.ts');
  }),
} as const;

/**
 * Replace elt with an element having the given tag.
 * @param elt The element to replace.
 * @param name The tagName of the new element.
 */
function replaceTag(elt: HTMLElement, name: string) {
  const successor = document.createElement(name);
  // Move the children to the successor
  while (elt.firstChild) {
    successor.append(elt.firstChild);
  }
  // Copy the attributes to the successor
  for (let index = elt.attributes.length - 1; index >= 0; --index) {
    successor.attributes.setNamedItem(elt.attributes[index].cloneNode() as Attr);
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
