import pdfobject from 'pdfobject';

window.customElements.define(
  'pdf-object',
  class extends HTMLElement {
    connectedCallback() {
      // since the web-component is defined after the DOM is ready, it is safe to look at the children.
      const fallbackLink = this.innerHTML; // eslint-disable-line wc/no-child-traversal-in-connectedcallback
      pdfobject.embed(this.getAttribute('src'), this, {
        fallbackLink,
      });
    }
  },
);
