// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/shared/search/issue/syntax.tmpl
// web_src/css/modules/dialog.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {accessibilityCheck} from './shared/accessibility.ts';

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
  test.describe(`Issue search (${run.title})`, () => {
    test.use({javaScriptEnabled: run.useJs});

    test('info modal appears on click', async ({page}) => {
      const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const search = page.locator('form.issue-list-search');
      await expect(search).toBeVisible();

      const syntaxModal = page.locator('#search-syntax-modal');
      await expect(syntaxModal).toBeHidden();

      await search.locator('button[command="show-modal"]').click();
      await expect(syntaxModal).toBeVisible();
    });

    test('info modal disappears on Esc', async ({page}) => {
      const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const search = page.locator('form.issue-list-search');
      await expect(search).toBeVisible();

      const syntaxModal = page.locator('#search-syntax-modal');
      await expect(syntaxModal).toBeHidden();
      await search.locator('button[command="show-modal"]').click();
      await expect(syntaxModal).toBeVisible();

      await page.keyboard.press('Escape');
      await expect(syntaxModal).toBeHidden();
    });

    test('info modal disappears on Close button', async ({page, isMobile}) => {
      const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const search = page.locator('form.issue-list-search');
      await expect(search).toBeVisible();

      const syntaxModal = page.locator('#search-syntax-modal');
      await expect(syntaxModal).toBeHidden();
      await search.locator('button[command="show-modal"]').click();
      await expect(syntaxModal).toBeVisible();

      await syntaxModal.locator('button[command="close"]').click({force: isMobile}); // dl intercepts pointer events on mobile somehow
      await expect(syntaxModal).toBeHidden();
    });

    test('info modal disappears on click outside', async ({page}) => {
      const response = await page.goto('/user2/repo1/issues', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const search = page.locator('form.issue-list-search');
      await expect(search).toBeVisible();

      const syntaxModal = page.locator('#search-syntax-modal');
      await expect(syntaxModal).toBeHidden();
      await search.locator('button[command="show-modal"]').click();
      await expect(syntaxModal).toBeVisible();

      const box = await syntaxModal.boundingBox();
      await page.mouse.click(box.x + 2, box.y + 2); // clicking the modal itself does nothing
      await expect(syntaxModal).toBeVisible();
      await page.mouse.click(box.x - 1, box.y);
      await expect(syntaxModal).toBeHidden();
    });
  });
}
