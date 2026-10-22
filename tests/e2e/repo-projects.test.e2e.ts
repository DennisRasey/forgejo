// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/projects/view.tmpl
// web_src/js/features/repo-projects.ts
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test.use({user: 'user2'});

test.describe('project columns', () => {
  test.beforeEach(async ({page}, testInfo) => {
    const resp = await page.goto('/user2/repo2/projects');
    expect(resp.status()).toBe(200);

    // create a project for this test (project IDs are not always predictable, but test name works ok)
    const testPath = testInfo.titlePath.join(' › ');
    const testTitle = `${testPath} (${testInfo.project.name})`; // e.g. "repo-projects › project columns › create (Mobile Chrome)"
    const projectName = `Test project: ${testTitle}`;
    await page.getByRole('link', {name: 'New project'}).click();
    await page.getByRole('textbox', {name: 'Title'}).fill(projectName);
    await page.getByRole('button', {name: 'Create project'}).click();
    await page.getByRole('link', {name: projectName}).click();
  });

  test('create', async ({page}) => {
    const createModal = page.locator('dialog#new-project-column-item');
    await expect(createModal).toBeHidden();
    await expect(page.locator('.project-column')).toBeHidden();
    await page.getByRole('button', {name: 'New column'}).click();
    await expect(createModal).toBeVisible();
    await createModal.getByRole('textbox', {name: 'Name'}).fill('Test column');
    await createModal.getByRole('button', {name: 'Create column'}).click();
    await expect(createModal).toBeHidden();
    await expect(page.locator('.project-column', {hasText: 'Test column'})).toBeVisible();
  });

  test('edit', async ({page}) => {
    const createModal = page.locator('dialog#new-project-column-item');
    await expect(page.locator('.project-column')).toBeHidden();
    await page.getByRole('button', {name: 'New column'}).click();
    await createModal.getByRole('textbox', {name: 'Name'}).fill('Test column');
    await createModal.getByRole('button', {name: 'Create column'}).click();
    const testColumn = page.locator('.project-column').first();
    const testColumnTitle = testColumn.locator('span.project-column-title-label'); // TODO: heading level 3 or 4
    await expect(testColumn).toBeVisible();
    await expect(testColumnTitle).toHaveText('Test column');
    await expect(testColumnTitle).not.toHaveText('Different test column');

    const editModal = page.locator('dialog[id^="edit-project-column-modal-"]');
    await expect(editModal).toBeHidden();
    await testColumn.getByRole('combobox').click(); // TODO: label, and improved dropdown?
    await testColumn.getByRole('option', {name: 'Edit column'}).click();
    await expect(editModal).toBeVisible();
    await editModal.getByRole('textbox', {name: 'Name'}).fill('Different test column');
    await editModal.getByRole('button', {name: 'Edit column'}).click();
    await expect(editModal).toBeHidden();
    await expect(testColumnTitle).not.toHaveText('Test column');
    await expect(testColumnTitle).toHaveText('Different test column');
  });
});
