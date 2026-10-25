// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {expect, type Locator, type Page} from '@playwright/test';
import {test} from '../utils_e2e.ts';

/**
 * Constructs test cases that assure interactive modal closure behaviors for a given modal.
 *
 * These tests verify that the modal can be light-dismissed or closed via a button.
 * Callers must take care to have a `test.beforeEach` block navigate to the relevant webpage
 * before these tests run.
 *
 * @param modalLoc Returns the locator for the modal under test.
 * @param actuatorLoc Returns the locator for an element that activates the modal on click.
 * @param cancelButtonName The accessible name of the modal action button for dismissal without acceptance. Usually "Cancel" or "Close".
 */
export function testModalClosure(modalLoc: (p: Page) => Locator, actuatorLoc: (p: Page) => Locator, cancelButtonName: string) {
  test.describe('Modal closure', () => {
    test.beforeEach(async ({page}) => {
      const modal = modalLoc(page);
      await expect(modal).toBeHidden();

      const button = actuatorLoc(page);
      await button.click();
    });

    test('Esc key', async ({page}) => {
      const modal = modalLoc(page);
      await expect(modal).toBeVisible();
      await page.keyboard.press('Escape');
      await expect(modal).toBeHidden();
    });

    test('Click outside', async ({page}) => {
      const modal = modalLoc(page);
      await expect(modal).toBeVisible();
      // can't select ::backdrop directly, so manually click just outside of the bounding box for the same effect
      const box = await modal.boundingBox();
      await page.mouse.click(box.x + 2, box.y + 2); // clicking the modal itself does nothing
      await expect(modal).toBeVisible();
      await page.mouse.click(box.x - 1, box.y);
      await expect(modal).toBeHidden();
    });

    test(`${cancelButtonName} button`, async ({page, isMobile}) => {
      const modal = modalLoc(page);
      await expect(modal).toBeVisible();
      await modal.getByRole('button', {name: cancelButtonName}).click({force: isMobile}); // 😕 on some webpages, other elements intercept pointer events somehow, but only on mobile 🤷‍♀️
      await expect(modal).toBeHidden();
    });
  });
}
