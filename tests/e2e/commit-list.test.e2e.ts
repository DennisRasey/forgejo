// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/repo/pulls/commits_list.tmpl
// web_src/css/repo.css
// web_src/css/repo/commit-list.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.describe(`PR commits`, () => {
  test.use({user: 'user2'});

  test('Any layout', async ({page}) => {
    const response = await page.goto('/user2/repo1/pulls/3/commits');
    expect(response?.status()).toBe(200);

    const commitGroup = page.locator('.commit-group:first-of-type');

    // Date group visibility test
    await expect(commitGroup).toBeVisible();
    await expect(commitGroup.locator('h4')).toBeVisible();

    const commit = commitGroup.locator('.commit:first-child');

    await expect(commit).toHaveCSS('display', 'grid');
  });

  test('Mobile responsive layout checks', async ({page, isMobile}) => {
    test.skip(!isMobile);

    const response = await page.goto('/user2/repo1/pulls/3/commits');
    expect(response?.status()).toBe(200);

    // Mobile-specific visibility test
    const commit = page.locator('.commit-group:first-of-type .commit:first-child');
    await expect(commit.locator('.commit-buttons')).toBeHidden();
    await expect(commit.locator('.button-sequence button[data-clipboard-text]')).toBeVisible();

    // Mobile-specific grid positioning
    // toHaveCSS returns absolute values in px with decimals. This matcher only
    // checks if the string has two \S+px separated by one \s+
    await expect(commit).toHaveCSS('grid-template-columns', /^\S+px\s+\S+px$/);
    await expect(commit.locator('.author')).toHaveCSS('grid-column-start', '1');
    await expect(commit.locator('.date')).toHaveCSS('grid-column-start', '2');
    await expect(commit.locator('.message')).toHaveCSS('grid-column-end', 'span 2');

    // Horizontal scrolling to check for overflow
    await expect(page.locator('.commits').first()).not.toHaveCSS('overflow-x', 'scroll');

    await screenshot(page);
  });

  test('Dropdown check in mobile viewport', async ({page, isMobile}) => {
    test.skip(!isMobile);

    const response = await page.goto('/user2/repo1/pulls/3/commits');
    expect(response?.status()).toBe(200);

    const commit = page.locator('.commit-group:first-of-type .commit:first-child');

    // Click dropdown btn
    const dropdown = commit.locator('.dialog-dropdown');
    await expect(dropdown).toBeVisible();
    await dropdown.locator('.opener').click();

    // List menu items of dropdown
    const menuItem = commit.locator('.dialog-dropdown ul li a'); // repo_path; always visible
    await expect(menuItem).toHaveAttribute('href', '/user2/repo1/src/commit/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd');
    await menuItem.click();
    await page.waitForURL(/.*\/user2\/repo1\/src\/commit\/5f22f7d0d95d614d25a5b68592adb345a4b5c7f/);
    await screenshot(page);
  });

  test('Desktop responsive layout checks', async ({page, isMobile}) => {
    test.skip(isMobile);

    const response = await page.goto('/user2/repo1/pulls/3/commits');
    expect(response?.status()).toBe(200);

    // Desktop-specific visibility test
    const commit = page.locator('.commit-group:first-of-type .commit:first-child');
    await expect(commit.locator('.commit-buttons')).toBeVisible();
    await expect(commit.locator('.button-sequence button[data-clipboard-text]')).toBeHidden();
    await expect(commit.locator('.dialog-dropdown')).toBeHidden();

    // Desktop layout is has specific grid-template-columns
    // toHaveCSS returns absolute values in px with decimals. This matcher only
    // checks if the string has five \S+px separated by four \s+
    await expect(commit).toHaveCSS('grid-template-columns', /^\S+px\s+\S+px\s+\S+px\s+\S+px\s+\S+px$/);

    await screenshot(page);
  });
});
