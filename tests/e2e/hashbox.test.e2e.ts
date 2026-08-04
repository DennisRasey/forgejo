// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/css/modules/hashbox.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test('Hashbox styles', async ({browser}) => {
  const context = await browser.newContext({javaScriptEnabled: false});
  const page = await context.newPage();

  await page.goto('/-/demo/hashbox');

  const heights = await page.locator('.sha.label .signature').evaluateAll((items) =>
    items.map((item) => item.getBoundingClientRect().height),
  );
  // Confirm that elements to verify were found
  expect(heights.length).toBeGreaterThan(1);

  // All signatures should be same height
  expect(heights.every((height) => height === heights[0])).toBeTruthy();
});
