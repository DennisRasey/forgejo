// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {toggleElem} from '../utils/dom.ts';
import {showModal} from '../modules/modal.ts';

export function initRepoBranchButton() {
  const createBranchModal = document.querySelector('#create-branch-modal');
  for (const el of document.querySelectorAll('.show-create-branch-modal')) {
    el.addEventListener('click', () => {
      const createBranchModalForm = createBranchModal.querySelector('form');
      const branchFromName = el.getAttribute('data-branch-from');

      createBranchModalForm.action = createBranchModalForm.getAttribute('data-base-action') + encodeURIComponent(branchFromName);
      createBranchModal.querySelector('#modal-create-branch-from-span').textContent = branchFromName;

      showModal('create-branch-modal', undefined);
    });
  }

  const renameBranchModel = document.querySelector('#rename-branch-modal');
  for (const el of document.querySelectorAll('.show-rename-branch-modal')) {
    el.addEventListener('click', () => {
      const oldBranchName = el.getAttribute('data-old-branch-name');
      (renameBranchModel.querySelector('input[name="from"]') as HTMLInputElement).value = oldBranchName;

      const branchToEl = renameBranchModel.querySelector('.label-branch-from');
      branchToEl.textContent = branchToEl.getAttribute('data-locale').replace('%s', oldBranchName);

      toggleElem(renameBranchModel.querySelector('.default-branch-warning'), el.getAttribute('data-is-default-branch') === 'true');

      showModal('rename-branch-modal', undefined);
    });
  }
}
