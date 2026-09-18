// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// routers/web/user/**
// templates/shared/user/**
// web_src/js/features/common-global.js
// web_src/js/modules/dropdown.ts
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.use({user: 'user2'});

test('Follow and block actions', async ({page}) => {
  await page.goto('/user1');

  const followButton = page.locator('.main-actions > button');
  const dropdownOpener = page.locator('.main-actions .dropdown summary');
  const dropdownContent = page.locator('.main-actions .dropdown .content');
  const blockButton = page.locator('#action-block');
  const blockModal = page.locator('#block-user');
  const flashMessage = page.locator('#flash-message');

  // Check if following and then unfollowing works.
  await expect(followButton).toContainText('Follow');
  await followButton.click();
  await expect(followButton).toContainText('Unfollow');
  await followButton.click();
  await expect(followButton).toContainText('Follow');

  // Dropdown is closed, block button is not visible
  await expect(dropdownContent).toBeHidden();
  await expect(blockButton).toBeHidden();

  // Open dropdown, block button should become visible
  await dropdownOpener.click();
  await expect(dropdownContent).toBeVisible();
  await expect(blockButton).toBeVisible();
  await expect(blockButton).toContainText('Block');

  // Use button to open confirmation modal
  await blockButton.click();
  await expect(blockModal).toBeVisible();
  await screenshot(page);

  // Opening modal closes dropdown
  await expect(dropdownContent).toBeHidden();
  await expect(blockButton).toBeHidden();

  // Confirm block action
  await blockModal.locator('.red.button').click();

  // Changes after blocking: modal is hidden, button is changed to "Unblock"
  await expect(blockButton).toContainText('Unblock');
  await expect(blockModal).toBeHidden();

  // Attempting to follow the user now yields in a error
  await followButton.click();
  await expect(flashMessage).toBeVisible();
  await expect(flashMessage).toContainText('You cannot follow this user because you have blocked this user or this user has blocked you.');
  await screenshot(page);

  // Unblocking the user changes the button back to "Block"
  await dropdownOpener.click();
  await blockButton.click();
  await expect(blockButton).toContainText('Block');
});
