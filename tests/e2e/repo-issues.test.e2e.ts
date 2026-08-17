// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/shared/label_filter.tmpl
// web_src/js/features/repo-issue-sidebar-list.ts
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test.use({user: 'user2'});

test('Label filter exclusion', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues');
  expect(response?.status()).toBe(200);

  await expect(
    page.getByRole('link', {name: 'issue1'}),
  ).toBeVisible();

  // open label filter dropdown
  await page.getByRole('combobox').filter({has: page.getByText('Label')}).click();

  // exclude the label1 label attached to issue1
  const labelOption = page
    .getByRole('option').filter({has: page.getByText('label1')});
  await expect(labelOption).toBeVisible();
  await labelOption.hover();
  await labelOption.getByRole('button', {name: 'Exclude label'}).click();

  await expect(
    page.getByRole('link', {name: 'issue1'}),
  ).toBeHidden();

  // open label filter dropdown
  await page.getByRole('combobox').filter({has: page.getByText('Label')}).click();

  // clear exclusion
  await expect(labelOption).toBeVisible();
  await labelOption.hover();
  await labelOption.getByRole('button', {name: 'Clear exclusion'}).click();

  await expect(
    page.getByRole('link', {name: 'issue1'}),
  ).toBeVisible();
});

test('Delete multiple issues', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues');
  expect(response?.status()).toBe(200);

  const deleteButton = page.getByRole('button', {name: 'Delete'});
  await expect(deleteButton).toBeHidden();
  await page.getByLabel('Check/Uncheck "issue1"').click();
  await expect(deleteButton).toBeVisible();

  await deleteButton.click();
  await expect(page.locator('#delete-issues')).toBeVisible();
});
