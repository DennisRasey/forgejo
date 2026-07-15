// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/repo/home.tmpl
// templates/repo/diff/box.tmpl
// templates/repo/issue/view_content/context_menu.tmpl
// web_src/js/features/clipboard.js
// @watch end

import {expect, type Page} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test('copy src file path to clipboard', async ({page}) => {
  const response = await page.goto('/user2/repo1/src/branch/master/README.md');
  expect(response?.status()).toBe(200);

  await page.click('[data-clipboard-text]');

  await expect(async () => {
    const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboardText).toContain('README.md');
  }).toPass();

  await expect(page.getByText('Copied')).toBeVisible();
  await screenshot(page, page.getByText('Copied'), 50);
});

async function evaluateCommentCopyMarkdown(page: Page, url: string, commentId: string, clipboardExpectations?: string[]): Promise<boolean> {
  const response = await page.goto(url);
  expect(response?.status()).toBe(200);

  const areaOfInterest = page.locator(`#${commentId} .comment-container details.dropdown`);

  // Open dropdown
  await areaOfInterest.locator('summary').click();
  await expect(areaOfInterest).toHaveAttribute('open');

  // Request copy and check if it succeeded
  await areaOfInterest.locator('.content ul li button').getByText('Copy Markdown').click();
  await expect(async () => {
    const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
    for (const expectation of clipboardExpectations) {
      expect(clipboardText).toContain(expectation);
    }
  }).toPass({timeout: 3000});

  // Dropdown should have been closed
  await expect(areaOfInterest).not.toHaveAttribute('open');

  return true;
}

test.describe('Copy comment as Markdown to clipboard', () => {
  test('Issue top comment', async ({page}) => {
    await expect(async () => {
      await evaluateCommentCopyMarkdown(page, '/user2/repo1/issues/1', 'issue-1', ['content for the first issue']);
    }).toPass({timeout: 3000});
  });

  test('Issue reply comment', async ({page}) => {
    await expect(async () => {
      await evaluateCommentCopyMarkdown(page, '/user2/repo1/issues/1', 'issuecomment-1001', [
        '## Lorem Ipsum',
        '**I am not appealed**',
        '`feature`',
      ]);
    }).toPass({timeout: 3000});
  });

  test('PR top comment', async ({page}) => {
    await expect(async () => {
      await evaluateCommentCopyMarkdown(page, '/user2/repo1/pulls/5', 'issue-11', ['content for the a pull request']);
    }).toPass({timeout: 3000});
  });
});

test('copy diff file path to clipboard', async ({page}) => {
  const response = await page.goto('/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d/README.md');
  expect(response?.status()).toBe(200);

  await page.click('[data-clipboard-text]');

  await expect(async () => {
    const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboardText).toContain('README.md');
  }).toPass();

  await expect(page.getByText('Copied')).toBeVisible();
  await screenshot(page, page.getByText('Copied'), 50);
});
