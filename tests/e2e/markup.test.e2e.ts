// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/css/markup/**
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test('Markup with #xyz-mode-only', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues/1');
  expect(response?.status()).toBe(200);

  const comment = page.locator('.comment-body>.markup', {hasText: 'test markup light/dark-mode-only'});
  await expect(comment).toBeVisible();
  await expect(comment.locator('[src$="#gh-light-mode-only"]')).toBeVisible();
  await expect(comment.locator('[src$="#gh-dark-mode-only"]')).toBeHidden();
  await screenshot(page);
});

test('Attention formatting', async ({page}) => {
  const response = await page.goto('/user2/markup-attention/src/branch/main/github-modern.md');
  expect(response?.status()).toBe(200);

  const attentionTypes = [
    'note',
    'tip',
    'important',
    'warning',
    'caution',
  ];

  await expect(async () => {
    await Promise.all(attentionTypes.map(async (attentionType) => {
      const selector = `.markup blockquote.attention-header.attention-${attentionType}`;
      expect(await page.locator(selector).evaluate((el) => getComputedStyle(el).borderInlineStartWidth)).toBe('4px');

      // Get all interesting colors
      const borderColor = await page.locator(selector).evaluate((el) => getComputedStyle(el).borderInlineStartColor);
      const titleColor = await page.locator(`${selector} > p.attention-title > strong`).evaluate((el) => getComputedStyle(el).color);
      const iconColor = await page.locator(`${selector} > p.attention-title > svg`).evaluate((el) => getComputedStyle(el).color);
      const ugcColor = await page.locator(`${selector} > p:not(.attention-title)`).evaluate((el) => getComputedStyle(el).color);

      // It's difficult to reliably evaluate the actual colors, but checking that the
      // colors are same for border/title/icon, and UGC is different is good enough
      expect(borderColor).toBe(titleColor);
      expect(titleColor).toBe(iconColor);
      expect(ugcColor).not.toBe(titleColor);
    }));
  }).toPass({timeout: 3000});

  await screenshot(page);
});
