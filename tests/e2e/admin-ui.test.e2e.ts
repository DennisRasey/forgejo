// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/js/features/admin/**
// templates/admin/**
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.use({user: 'user1'});

test('Admin notices modal', async ({page}) => {
  const response = await page.goto('/admin/notices');
  expect(response?.status()).toBe(200);

  await page.getByText('description1').click();
  await expect(page.locator('#detail-modal .content')).toHaveText('description1');
  await screenshot(page, page.locator('#detail-modal'));
  await page.getByText('Cancel').click();
  await expect(page.locator('#change-email-modal')).toBeHidden();

  await page.getByText('description2').click();
  await expect(page.locator('#detail-modal .content')).toHaveText('description2');
  await screenshot(page, page.locator('#detail-modal'));
  await page.getByText('Cancel').click();
  await expect(page.locator('#change-email-modal')).toBeHidden();

  await page.getByText('description3').click();
  await expect(page.locator('#detail-modal .content')).toHaveText('description3');
  await screenshot(page, page.locator('#detail-modal'));
  await page.getByText('Cancel').click();
  await expect(page.locator('#change-email-modal')).toBeHidden();
});

test('Admin email list', async ({page}) => {
  const response = await page.goto('/admin/emails');
  expect(response?.status()).toBe(200);

  await page.locator('[data-uid="21"]').click();
  await expect(page.locator('#change-email-modal .content')).toHaveText('Are you sure you want to update this email address?');
  await screenshot(page, page.locator('#change-email-modal .content'));
  await page.locator('#email-action-form').getByText('No').click();
  await expect(page.locator('#change-email-modal')).toBeHidden();

  const activated = await page.locator('[data-uid="9"] .svg').evaluate((node) => node.classList.contains('octicon-check'));
  await page.locator('[data-uid="9"]').click();
  await page.getByRole('button', {name: 'Yes'}).click();

  // Retry-proof
  if (activated) {
    await expect(page.locator('[data-uid="9"] .svg')).toHaveClass(/octicon-x/);
  } else {
    await expect(page.locator('[data-uid="9"] svg')).toHaveClass(/octicon-check/);
  }
});

test('Admin: delete a user', async ({page}) => {
  const response = await page.goto('/admin/users/1/edit');
  expect(response?.status()).toBe(200);

  const modal = page.locator('#delete-user-modal');
  const okButton = page.locator('#delete-user-modal .primary.button');

  // Check that modal appears after clicking
  await expect(modal).toBeHidden();
  await expect(okButton).toBeHidden();
  await page.locator('[data-modal="#delete-user-modal"]').click();
  await expect(modal).toBeVisible();
  await expect(okButton).toBeVisible();

  // Agree with deletion
  await okButton.click();

  // Should have been redirected to /admin/users/1
  await expect(page).toHaveURL(/\/admin\/users\/1$/);

  // This test doesn't actually delete a user as it attempts to delete the doer and
  // receives an error. This is enough to test that the request reaches the correct
  // endpoint without causing e2e retry headache
  await expect(page.locator('#flash-message')).toBeVisible();
});

test('Admin: delete a user from their public profile', async ({page, request}) => {
  // Reload the fixtures in case of an already deleting the user
  await request.patch('/_e2e/fixtures/reload');

  // Using the imported user to delete because it doesn't own any repositories
  const response = await page.goto('/imported');
  expect(await response?.status()).toBe(200);

  const modal = page.locator('#delete-user-modal');
  const okButton = page.locator('#delete-user-modal .primary.button');

  // Check that modal appears after clicking
  await expect(modal).toBeHidden();
  await expect(okButton).toBeHidden();
  await page.locator('#profile-avatar-card .dropdown').click();
  await page.locator('[data-modal="#delete-user-modal"]').click();
  await expect(modal).toBeVisible();
  await expect(okButton).toBeVisible();

  // Agree with deletion
  await okButton.click();

  // Should have been redirected to /admin/users
  await expect(page).toHaveURL(/\/admin\/users$/);
});
