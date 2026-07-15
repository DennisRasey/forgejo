// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/js/features/common-global.js
// web_src/js/features/comp/Paste.js
// web_src/js/features/repo-issue.js
// web_src/js/features/repo-legacy.js
// @watch end

import {expect, type Locator, type Page} from '@playwright/test';
import {test, dynamic_id} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.use({user: 'user2'});

async function pasteImage(el: Locator) {
  await el.evaluate(async (el) => {
    const base64 = `data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAA4AAAAOCAYAAAAfSC3RAAAAHklEQVQoU2MUk1P7z0AGYBzViDvURgMHT4oaQoEDAFuJEu2fuGfhAAAAAElFTkSuQmCC`;
    // eslint-disable-next-line no-restricted-syntax
    const response = await fetch(base64);
    const blob = await response.blob();

    el.focus();

    let pasteEvent = new Event('paste', {bubbles: true, cancelable: true});
    pasteEvent = Object.assign(pasteEvent, {
      clipboardData: {
        items: [
          {
            kind: 'file',
            type: 'image/png',
            getAsFile() {
              return new File([blob], 'foo.png', {type: blob.type});
            },
          },
        ],
      },
    });

    el.dispatchEvent(pasteEvent);
  });
}

async function assertCopy(page: Page, startWith: string) {
  const dropzone = page.locator('.dropzone');
  const preview = dropzone.locator('.dz-preview');
  const copyLink = preview.locator('.octicon-copy').locator('..');
  await copyLink.click();

  await expect(async () => {
    const clipboardContent = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboardContent).toContain(startWith);
  }).toPass();
}

test('Paste image in new comment', async ({page}) => {
  await page.goto('/user2/repo1/issues/new');

  const waitForAttachmentUpload = page.waitForResponse((response) => {
    return response.request().method() === 'POST' && response.url().endsWith('/attachments');
  });
  await pasteImage(page.locator('.markdown-text-editor'));
  await waitForAttachmentUpload;

  const dropzone = page.locator('.dropzone');
  await expect(dropzone.locator('.files')).toHaveCount(1);
  const preview = dropzone.locator('.dz-preview');
  await expect(preview).toHaveCount(1);
  await expect(preview.locator('.dz-filename')).toHaveText('foo.png');
  await expect(preview.locator('.octicon-copy')).toBeVisible();
  await assertCopy(page, '![foo](');

  await screenshot(page, page.locator('.issue-content-left'));
});

test('Re-add images to dropzone on edit', async ({page}) => {
  await page.goto('/user2/repo1/issues/new');

  const issueTitle = dynamic_id();
  await page.locator('#issue_title').fill(issueTitle);
  const waitForAttachmentUpload = page.waitForResponse((response) => {
    return response.request().method() === 'POST' && response.url().endsWith('/attachments');
  });
  await pasteImage(page.locator('.markdown-text-editor'));
  await waitForAttachmentUpload;
  await page.getByRole('button', {name: 'Create issue'}).click();

  await expect(page).toHaveURL(/\/user2\/repo1\/issues\/\d+$/);
  await page.click('.comment-container details.dropdown summary');
  const waitForAttachmentsLoad = page.waitForResponse((response) => {
    return response.request().method() === 'GET' && response.url().endsWith('/attachments');
  });
  await page.click('.comment-container details.dropdown .content .edit-content');
  await waitForAttachmentsLoad;

  const dropzone = page.locator('.dropzone');
  await expect(dropzone.locator('.files').first()).toHaveCount(1);
  const preview = dropzone.locator('.dz-preview');
  await expect(preview).toHaveCount(1);
  await expect(preview.locator('.dz-filename')).toHaveText('foo.png');
  await expect(preview.locator('.octicon-copy')).toBeVisible();
  await assertCopy(page, '![foo](');

  await screenshot(page, page.locator('.issue-content-left'));
});
