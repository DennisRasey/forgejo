// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// <dialog>-based dropdowns can be opened and closed with a wide variety of ways.
// Event listeners in this file provide more a few more options for that:
// - close on focusout, by clicking elsewhere or navigating away with Tab/Shift+Tab
// - open with ArrowDown, navigate with ArrowUp/Down, close by ArrowUp on first item

export function initDropdowns() {
  // Close open dropdown when it is unfocused (e.g. when user pressed Tab or Shift+Tab),
  // but not when user lost focus completely (e.g. browser window became unfocused)
  document.addEventListener('focusout', (event: FocusEvent) => {
    const dropdown = document.querySelector<HTMLDialogElement>('.dialog-dropdown dialog:popover-open');
    if (dropdown !== null) {
      const parent = dropdown.parentElement as HTMLDivElement;
      const target = event.target as HTMLElement;
      const newTarget = event.relatedTarget as HTMLElement;

      if (newTarget !== null && parent.contains(target) && !parent.contains(newTarget)) {
        // The previously focused element was within the open dropdown, but something
        // else is now focused, so the dropdown should be closed
        dropdown.hidePopover();
      }
    }
  });

  // Keyboard interaction with dropdowns
  document.addEventListener('keydown', (event: KeyboardEvent) => {
    if (!['ArrowUp', 'ArrowDown'].includes(event.key)) {
      // This eventListener is only concerned about a few keys
      return;
    }

    if (document.activeElement.localName === 'button' && event.key === 'ArrowDown') {
      const parent = document.activeElement.parentElement as HTMLDivElement;
      if (parent.classList.contains('dialog-dropdown')) {
        // Pressing ArrowDown on a focused opener of a closed dropdown will open
        // the dropdown and focus it's first item
        parent.querySelector<HTMLDialogElement>('dialog').showPopover();
        const firstFocusable = parent.querySelector<HTMLElement>('.content > ul > li :is(a, button, input)');
        firstFocusable?.focus();
        event.preventDefault();
        return;
      }
    }

    const dropdown = document.querySelector<HTMLDialogElement>('.dialog-dropdown > dialog:popover-open');
    if (dropdown !== null) {
      // Knowing document.activeElement, find the <li> that contains it
      const dropdownItems = dropdown.querySelectorAll<HTMLLIElement>('dialog > ul > li');
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
          (dropdown.parentElement as HTMLDivElement).querySelector<HTMLButtonElement>('.opener').focus();
          dropdown.hidePopover();
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
    }
  });
}
