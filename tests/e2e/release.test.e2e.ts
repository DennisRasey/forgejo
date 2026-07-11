// Copyright 2024-2026 The Forgejo Authors
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// models/repo/attachment.go
// modules/structs/attachment.go
// routers/web/repo/**
// services/attachment/**
// services/release/**
// templates/repo/release/**
// web_src/js/features/repo-release.js
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';
import {validate_form} from './shared/forms.ts';

test.use({user: 'user2'});

test.afterEach(async ({page}) => {
  // Delete release
  const response = await page.goto('/user2/repo2/releases/edit/2.0');
  test.skip(response.status() === 404, 'No release to delete');

  await page.locator('.delete-button').dispatchEvent('click');
  await page.locator('.button.ok').click();
  await expect(page).toHaveURL('/user2/repo2/releases');
});

test.describe('Releases', () => {
  test('External release attachments', async ({page, isMobile}) => {
    test.skip(isMobile);

    // Click "New release"
    await page.goto('/user2/repo2/releases');
    await page.getByRole('link', {name: 'New release'}).click();

    // Fill out form and create new release
    await expect(page).toHaveURL('/user2/repo2/releases/new');
    await validate_form({page}, 'fieldset');
    const textarea = page.locator('input[name=tag_name]');
    await textarea.pressSequentially('2.0');
    await expect(page.locator('input[name=title]')).toHaveValue('2.0');
    await page.click('#add-external-link');
    await page.click('#add-external-link');
    await page.fill('input[name=attachment-new-name-2]', 'Test');
    await page.fill('input[name=attachment-new-exturl-2]', 'https://forgejo.org/');
    await page.click('.remove-rel-attach');
    await screenshot(page);
    await page.getByRole('button', {name: 'Publish release'}).click();

    // Validate release page and click edit
    await expect(page).toHaveURL('/user2/repo2/releases');
    await expect(page.locator('.download[open] li')).toHaveCount(3);

    await expect(page.locator('.download[open] li:nth-of-type(1)')).toContainText('Test');
    await expect(page.locator('.download[open] li:nth-of-type(1) a')).toHaveAttribute('href', 'https://forgejo.org/');

    await expect(page.locator('.download[open] li:nth-of-type(2)')).toContainText('Source code (ZIP)');
    await expect(page.locator('.download[open] li:nth-of-type(2) span[data-tooltip-content]')).toHaveAttribute('data-tooltip-content', 'This attachment is automatically generated.');
    await expect(page.locator('.download[open] li:nth-of-type(2) a')).toHaveAttribute('href', '/user2/repo2/archive/2.0.zip');
    await expect(page.locator('.download[open] li:nth-of-type(2) a')).toHaveAttribute('type', 'application/zip');

    await expect(page.locator('.download[open] li:nth-of-type(3)')).toContainText('Source code (TAR.GZ)');
    await expect(page.locator('.download[open] li:nth-of-type(3) span[data-tooltip-content]')).toHaveAttribute('data-tooltip-content', 'This attachment is automatically generated.');
    await expect(page.locator('.download[open] li:nth-of-type(3) a')).toHaveAttribute('href', '/user2/repo2/archive/2.0.tar.gz');
    await expect(page.locator('.download[open] li:nth-of-type(3) a')).toHaveAttribute('type', 'application/gzip');
    await screenshot(page);
    await page.locator('.octicon-pencil').first().click();

    // Validate edit page and edit the release
    await expect(page).toHaveURL('/user2/repo2/releases/edit/2.0');
    await validate_form({page}, 'fieldset');
    await expect(page.locator('.attachment_edit:visible')).toHaveCount(2);
    await expect(page.locator('.attachment_edit:visible').nth(0)).toHaveValue('Test');
    await expect(page.locator('.attachment_edit:visible').nth(1)).toHaveValue('https://forgejo.org/');
    await page.locator('.attachment_edit:visible').nth(0).fill('Test2');
    await page.locator('.attachment_edit:visible').nth(1).fill('https://gitea.io/');
    await page.click('#add-external-link');
    await expect(page.locator('.attachment_edit:visible')).toHaveCount(4);
    await page.locator('.attachment_edit:visible').nth(2).fill('Test3');
    await page.locator('.attachment_edit:visible').nth(3).fill('https://gitea.com/');
    await screenshot(page);
    await page.getByRole('button', {name: 'Update release'}).click();

    // Validate release page and click edit
    await expect(page).toHaveURL('/user2/repo2/releases');
    await expect(page.locator('.download[open] li')).toHaveCount(4);
    await expect(page.locator('.download[open] li:nth-of-type(1)')).toContainText('Test2');
    await expect(page.locator('.download[open] li:nth-of-type(1) a')).toHaveAttribute('href', 'https://gitea.io/');
    await expect(page.locator('.download[open] li:nth-of-type(2)')).toContainText('Test3');
    await expect(page.locator('.download[open] li:nth-of-type(2) a')).toHaveAttribute('href', 'https://gitea.com/');
    await screenshot(page);
    await page.locator('.octicon-pencil').first().click();
  });

  test('Release name equals tag name if created from tag', async ({page}) => {
    await page.goto('/user2/repo2/releases/new?tag=v1.1');

    await expect(page.locator('input[name=title]')).toHaveValue('v1.1');
  });

  test('Release name equals release name if edit', async ({page, isMobile}) => {
    test.skip(isMobile);

    await page.goto('/user2/repo2/releases/new');

    await page.locator('input[name=title]').pressSequentially('v2.0');
    await page.locator('input[name=tag_name]').pressSequentially('2.0');
    await page.getByRole('button', {name: 'Publish release'}).click();

    await page.goto('/user2/repo2/releases/edit/2.0');

    await expect(page.locator('input[name=title]')).toHaveValue('v2.0');
  });

  test('UI reaction to lengthy UGC', async ({page, viewport, isMobile}) => {
    await page.goto('/user2/repo2/releases/new');

    await page.locator('input[name=tag_name]').pressSequentially('2.0');
    await page.locator('input[name=title]').pressSequentially('v'.repeat(200));
    await page.locator('textarea[name=content]').pressSequentially('v'.repeat(200)); // Description

    // Submit form. Mobile Chrome can't press the button in Playwright (not a Forgejo
    // bug). Work around this by pressing Enter on submit button
    await page.getByRole('button', {name: 'Publish release'}).press('Enter');

    // Check widths of UI elements
    await page.goto('/user2/repo2/releases');
    const release = page.locator('#release-list > li:has(a[href$="/tag/2.0"])');
    // Release entry should be less than viewport
    expect((await release.boundingBox()).width).toBeLessThan(viewport.width);
    if (isMobile) {
      const metaWidth = (await release.locator('.meta').boundingBox()).width;
      const titleWidth = (await release.locator('.release-title-wrap').boundingBox()).width;
      const detailsWidth = (await release.locator('.detail').boundingBox()).width;
      // In row layout they all should be similar to the viewport length, accounting
      // for 8px margins on each side
      expect(metaWidth).toBeCloseTo(viewport.width - 16, 0);
      expect(titleWidth).toBeCloseTo(viewport.width - 16, 0);
      expect(detailsWidth).toBeCloseTo(viewport.width - 16, 0);
      // They also should all be all same width
      expect(metaWidth).toBe(titleWidth);
      expect(titleWidth).toBe(detailsWidth);
    } else {
      // Left and right columns should be less than 25% and 75% of viewport width
      // But on wide screens there's a lot of additional emptiness, so we can't
      // match columns' width against the viewport, only make sure they fit
      expect((await release.locator('.meta').boundingBox()).width).toBeLessThan(viewport.width * 0.75);
      expect((await release.locator('.release-title-wrap').boundingBox()).width).toBeLessThan(viewport.width * 0.75);
      expect((await release.locator('.detail').boundingBox()).width).toBeLessThan(viewport.width * 0.75);
    }
  });
});
