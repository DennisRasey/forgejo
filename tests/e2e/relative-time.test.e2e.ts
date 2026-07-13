// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/admin/dashboard.tmpl
// web_src/js/webcomponents/relative-time.js
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test.use({user: 'user1'});

test('Relative time after htmx swap', async ({page}, workerInfo) => {
  test.skip(
    workerInfo.project.name !== 'firefox' && workerInfo.project.name !== 'Mobile Chrome',
    'This is a really slow test, so limit to a subset of client.',
  );
  await page.goto('/admin');

  const relativeTime = page.locator('.admin-dl-horizontal > dd:nth-child(2) > relative-time');
  // The admin dashboard uses <relative-time format="duration"> for server
  // uptime, which renders as a duration like "5 days, 3 hours" (no "ago").
  // Check that the component produced a formatted duration rather than the
  // raw datetime fallback.
  const durationPattern = /\d+ (year|month|week|day|hour|minute|second)/;
  await expect(relativeTime).toContainText(durationPattern);

  // The <relative-time> custom element renders its formatted text into an open
  // shadow root. Read that text directly: locator.textContent() does not
  // reflect the shadow-DOM content, and an htmx morph re-adds the raw-datetime
  // light-DOM fallback which would pollute toHaveText/toContainText.
  const textBefore = await relativeTime.evaluate((el) => el.shadowRoot?.textContent ?? '');

  const body = page.locator('body');
  await body.evaluate(
    (element) =>
      new Promise<void>((resolve) =>
        element.addEventListener('htmx:afterSwap', () => {
          resolve();
        }),
      ),
  );

  // The system-status panel refreshes itself via htmx every 5 seconds. A
  // previous regression (0e8d752d86) reset the rendered relative-time text on
  // swap; assert the formatted text survives the swap unchanged.
  await expect.poll(
    () => relativeTime.evaluate((el) => el.shadowRoot?.textContent ?? ''),
  ).toBe(textBefore);
});
