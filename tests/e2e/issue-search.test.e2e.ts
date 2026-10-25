// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/shared/search/issue/syntax.tmpl
// web_src/css/modules/dialog.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {accessibilityCheck} from './shared/accessibility.ts';
import {testModalClosure} from './shared/modals.ts';

test('Issue search: accessibility', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
  expect(response?.status()).toBe(200);

  const search = page.locator('form.issue-list-search');
  await expect(search).toBeVisible();

  await accessibilityCheck({page}, ['form.issue-list-search'], [], []);
});

for (const run of [
  {title: 'JS off', useJs: false},
  {title: 'JS on', useJs: true},
]) {
  test.describe(`Issue search syntax modal (${run.title})`, () => {
    test.use({javaScriptEnabled: run.useJs});

    test.beforeEach(async ({page}) => {
      const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);
    });

    test('Appears on click', async ({page}) => {
      const search = page.locator('form.issue-list-search');
      await expect(search).toBeVisible();

      const syntaxModal = page.locator('#search-syntax-modal');
      await expect(syntaxModal).toBeHidden();

      await search.locator('button[command="show-modal"]').click();
      await expect(syntaxModal).toBeVisible();
    });

    testModalClosure(
      (page) => page.locator('#search-syntax-modal'),
      (page) => page.locator('form.issue-list-search button[command="show-modal"]'),
      'Close',
    );
  });
}
