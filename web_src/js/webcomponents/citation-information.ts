// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import '@citation-js/plugin-software-formats';
import '@citation-js/plugin-bibtex';
import {Cite, plugins, type InputOptions} from '@citation-js/core';

import {getCurrentLocale} from '../utils.ts';
import {initTab} from '../modules/tab.ts';

window.customElements.define(
  'citation-information',
  class extends HTMLElement {
    connectedCallback() {
      const children = this.children; // eslint-disable-line wc/no-child-traversal-in-connectedcallback
      if (children.length !== 1 || children[0].tagName !== 'TABLE') {
        // developer error
        throw new Error(
          `<citation-information> expected one <table> child, got ${children.length}`,
        );
      }

      const lang = getCurrentLocale() || 'en-US';

      const raw = children[0] as HTMLTableElement;
      raw.dataset.tab = 'raw';
      raw.classList.add('tab', 'active');

      // like in copy-content
      const lineEls = raw.querySelectorAll('.lines-code');
      const code = Array.from(lineEls, (el) => el.textContent).join('');

      const inputType = plugins.input.type(code);
      let parsed: Cite;
      try {
        // FIXME: upstream failed to mark Cite constructor options as optional, despite them each having default values, so tsc complains here about an incomplete object
        parsed = new Cite(code, {forceType: inputType} as InputOptions);
      } catch (err) {
        const elContainer = document.createElement('div');
        elContainer.classList.add('ui', 'warning', 'message');

        const elHeader = document.createElement('div');
        elHeader.classList.add('header');
        elHeader.textContent = `Could not parse citation-information (format ${inputType})`; // ideally this message should be localized, however the error below will likely be in english
        elContainer.append(elHeader);

        const elParagraph = document.createElement('pre');
        elParagraph.textContent = err;
        elContainer.append(elParagraph);
        this.prepend(elContainer);
        return;
      }

      const toggleBar = document.createElement('div');
      toggleBar.classList.add('switch');

      const newButton = (txt: string, id: string, tooltip: string, active?: boolean) => {
        const el = document.createElement('button');
        el.textContent = txt;
        el.dataset.tab = id;
        if (tooltip) {
          el.dataset.tooltipContent = tooltip;
        }
        el.classList.add('item');
        if (active) {
          el.classList.add('active');
        }
        return el;
      };
      let originalText = 'Original';
      let originalTooltip = '';
      switch (inputType) {
        case '@biblatex/text':
          originalText = 'BibTeX';
          break;
        case '@else/yaml':
          originalText = 'CFF';
          originalTooltip = 'Citation File Format';
          break;
      }
      toggleBar.append(newButton(originalText, 'raw', originalTooltip, true));

      const appendTab = (id: string, btnLabel: string, btnTooltip: string, tabContent: string) => {
        const el = document.createElement('pre');
        el.textContent = tabContent;
        el.dataset.tab = id;
        el.classList.add('tab');
        el.style.padding = '1rem';
        el.style.margin = '0';
        this.append(el);
        toggleBar.append(newButton(btnLabel, id, btnTooltip));
      };
      if (inputType !== '@biblatex/text') {
        appendTab(
          'bibtex',
          'BibTeX',
          '',
          parsed.format('bibtex', {lang, format: 'text'}).trim(),
        );
      }
      if (inputType !== '@else/yaml') {
        appendTab(
          'cff',
          'CFF',
          'Citation File Format',
          parsed.format('cff').trim(),
        );
      }

      const toggleBarParent = document.querySelector('.file-header-left');
      toggleBarParent.prepend(toggleBar);
      initTab(toggleBarParent);
    }
  },
);
