// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {expect} from '@playwright/test';
import {test, test_context} from './utils_e2e.ts';

test('thanks.dev default template is valid', async ({browser}, workerInfo) => {
  // See also modules/setting/funding_live_test.go
  test.skip(
    !process.env['FUNDING_TEST_LIVE_PROVIDERS'] ||
    workerInfo.project.name !== 'chromium',
    'This is a really slow test against a live service, so limit to a subset of client.',
  );

  // matches our `thanks_dev` provider template; profile from https://github.com/sebastianbergmann/object-graph
  const context = await test_context(browser, {baseURL: 'https://thanks.dev'});
  const page = await context.newPage();
  const response = await page.goto('/u/gh/sebastianbergmann', {waitUntil: 'domcontentloaded'});
  expect(response.status()).toBe(403); // normal thanks.dev behavior 🙃

  // loading is successful when profile details (such as a link to the user's GitHub profile) are visible
  await expect(page.locator('a[href="https://github.com/sebastianbergmann"]').first()).toBeVisible();
});
