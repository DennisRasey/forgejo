// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/js/components/RepoBranchTagSelector.vue
// web_src/js/features/common-global.js
// web_src/css/repo.css
// web_src/css/modules/stats-bar.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.use({user: 'user2'});

test('Language stats bar', async ({browser}) => {
  // This test doesn't need JS and runs a little faster without it
  const context = await browser.newContext({javaScriptEnabled: false});
  const page = await context.newPage();

  const response = await page.goto('/user2/language-stats-test');
  expect(response?.status()).toBe(200);

  await expect(page.locator('#language-stats ul')).toBeHidden();

  await page.click('#language-stats summary');
  await expect(page.locator('#language-stats ul')).toBeVisible();
  await screenshot(page);

  await page.click('#language-stats summary');
  await expect(page.locator('#language-stats ul')).toBeHidden();
  await screenshot(page);
});

test('Repo title', async ({browser}) => {
  const context = await browser.newContext({javaScriptEnabled: false});
  const page = await context.newPage();

  const response = await page.goto('/user2/repo1');
  expect(response?.status()).toBe(200);

  const repoHeader = page.locator('.repo-header');
  expect(await repoHeader.locator('.flex-item-title').evaluate((el) => getComputedStyle(el).fontWeight)).toBe('400');
  expect(await repoHeader.locator('.flex-item-title a[href="/user2"]').evaluate((el) => getComputedStyle(el).fontWeight)).toBe('400');
  expect(await repoHeader.locator('.flex-item-title a[href="/user2/repo1"]').evaluate((el) => getComputedStyle(el).fontWeight)).toBe('600');
});

test('Branch selector commit icon', async ({page}) => {
  const response = await page.goto('/user2/repo1');
  expect(response?.status()).toBe(200);

  await expect(page.locator('.branch-dropdown-button svg.octicon-git-branch')).toBeVisible();
  await expect(page.locator('.branch-dropdown-button')).toHaveText('master');

  await page.goto('/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d');
  await expect(page.locator('.branch-dropdown-button svg.octicon-git-commit')).toBeVisible();
  await expect(page.locator('.branch-dropdown-button')).toHaveText('65f1bf27bc');
});

test('Star and unstar', async ({page}) => {
  const response = await page.goto('/user2/repo1');
  expect(response?.status()).toBe(200);

  const starButton = page.locator('button[aria-label="Star"], button[aria-label="Unstar"]');
  await starButton.click();

  await expect(
    page.locator('button[aria-label="Star"]:focus, button[aria-label="Unstar"]:focus'),
  ).toBeVisible();

  expect(page.url()).not.toContain('/star');
  expect(page.url()).not.toContain('/unstar');

  await starButton.click();
  expect(page.url()).not.toContain('/star');
  expect(page.url()).not.toContain('/unstar');
});

test('Watch/unwatch button URL retention', async ({page}) => {
  const response = await page.goto('/user2/repo1');
  expect(response?.status()).toBe(200);

  const watchDropdown = page.locator('details.dropdown#watch-button');
  const watchSummary = watchDropdown.locator('summary');
  const watchMenu = watchDropdown.locator('.content');

  // Open the dropdown
  await watchSummary.click();
  await expect(watchMenu).toBeVisible();

  // Select PRs
  const prsCheckbox = watchMenu.locator('input[name="watch_pull_requests"]');
  await prsCheckbox.click();

  // Save
  const saveButton = watchMenu.locator('button:has-text("Save")');
  await saveButton.isVisible();
  await saveButton.click();

  // Wait for menu to disappear
  await expect(watchMenu).toBeHidden();
  expect(page.url()).not.toContain('/watch');
  expect(page.url()).not.toContain('/unwatch');

  // Open the dropdown once more
  await watchSummary.click();
  await expect(watchMenu).toBeVisible();

  // Unwatch
  const unwatchButton = watchMenu.locator('button:has-text("Unwatch")');
  await unwatchButton.click();

  // Wait for menu to disappear
  await expect(watchMenu).toBeHidden();
  expect(page.url()).not.toContain('/watch');
  expect(page.url()).not.toContain('/unwatch');
});
