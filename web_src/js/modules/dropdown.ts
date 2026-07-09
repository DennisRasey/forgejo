// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// Details can be opened by clicking summary or by pressing Space or Enter while
// being focused on summary. But without JS options for closing it are limited.
// Event listeners in this file provide more convenient options for that:
// click iteration with anything on the page and pressing Escape.

export function initDropdowns() {
  // Close open dropdown by clicking elsewhere on the page
  document.addEventListener('click', (event: MouseEvent) => {
    const dropdown = document.querySelector<HTMLDetailsElement>('details.dropdown[open]');
    if (dropdown === null) {
      // No open dropdowns on page, nothing to do
      return;
    }

    const target = event.target as HTMLElement;
    if (dropdown.contains(target)) {
      // User clicked something in the open dropdown, don't interfere
      return;
    }

    // User clicked something elsewhere, close the open dropdown
    dropdown.removeAttribute('open');
  });

  // Close open dropdown when it is unfocused (e.g. when user pressed Tab or Shift+Tab),
  // but not when user lost focus completely (e.g. browser window became unfocused)
  document.addEventListener('focusout', (event: FocusEvent) => {
    const dropdown = document.querySelector<HTMLDetailsElement>('details.dropdown[open]');
    if (dropdown === null) {
      // No open dropdowns on page, nothing to do
      return;
    }

    const target = event.target as HTMLElement;
    const newTarget = event.relatedTarget as HTMLElement;

    if (newTarget !== null && dropdown.contains(target) && !dropdown.contains(newTarget)) {
      // The previously focused element was within the open dropdown, but something
      // else is now focused, so the dropdown should be closed
      dropdown.removeAttribute('open');
    }
  });

  // Keyboard interaction with dropdowns
  document.addEventListener('keydown', (event: KeyboardEvent) => {
    if (!['Escape', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
      // This eventListener is only concerned about a few keys
      return;
    }

    if (document.activeElement.localName === 'summary' && event.key === 'ArrowDown') {
      const parentDropdown = document.activeElement.parentElement as HTMLDetailsElement;
      if (parentDropdown.classList.contains('dropdown')) {
        // User pressed ArrowDown on a focused summary of a closed dropdown.
        // We'll open the dropdown and focus it's first item
        parentDropdown.setAttribute('open', 'true');
        const firstFocusable = parentDropdown.querySelector<HTMLElement>('.content > ul > li :is(a, button, input)');
        firstFocusable?.focus();
        event.preventDefault();
        return;
      }
    }

    const dropdown = document.querySelector<HTMLDetailsElement>('details.dropdown[open]');
    // This part of the code only knows how to work with open dropdown
    if (dropdown === null) {
      // No open dropdowns on page, nothing to do
      return;
    }

    if (event.key === 'Escape') {
      // User pressed Escape while having an open dropdown, we'll close it
      dropdown.removeAttribute('open');
      return;
    }

    // Knowing document.activeElement, find the <li> that contains it
    const dropdownItems = dropdown.querySelectorAll<HTMLLIElement>('.content > ul > li');
    let activeLi: HTMLLIElement, activeLiIndex: number;
    for (let i = 0; i < dropdownItems.length; i++) {
      const li = dropdownItems[i] as HTMLLIElement;
      if (!li.contains(document.activeElement)) continue;
      activeLi = li;
      activeLiIndex = i;
      break;
    }
    if (activeLi === undefined) {
      // The focused element is not a list item or it's contents, but something else in the dropdown
      return;
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      if (activeLiIndex === 0) {
        // Last child is already selected, but we can navigate back to the opener and close the dropdown
        dropdown.querySelector('summary').focus();
        dropdown.removeAttribute('open');
        return;
      }
      dropdownItems[activeLiIndex - 1].querySelector<HTMLElement>(':is(a, button, input)')?.focus();
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      if (activeLiIndex === dropdownItems.length - 1) {
        // First child is already selected
        return;
      }
      dropdownItems[activeLiIndex + 1].querySelector<HTMLElement>(':is(a, button, input)')?.focus();
    }
  });
}
