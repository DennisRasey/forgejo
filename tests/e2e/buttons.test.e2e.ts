// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/css/modules/switch.css
// web_src/css/modules/button.css
// web_src/css/modules/dropdown.css
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test.use({user: 'user2'});

test('Buttons and other controls have consistent height', async ({page}) => {
  await page.goto('/user1');

  // The height of dropdown opener and the button should be matching, even in mobile browsers with coarse pointer
  let buttonHeight = (await page.locator('#profile-avatar-card .main-actions > button').boundingBox()).height;
  const openerHeight = (await page.locator('#profile-avatar-card .main-actions .dropdown').boundingBox()).height;
  expect(openerHeight).toBe(buttonHeight);

  await page.goto('/notifications');

  // The height should also be consistent with the button on the previous page
  const switchHeight = (await page.locator('.switch').boundingBox()).height;
  expect(buttonHeight).toBe(switchHeight);

  buttonHeight = (await page.locator('.button-row .button[href="/notifications/subscriptions"]').boundingBox()).height;
  expect(buttonHeight).toBe(switchHeight);

  const purgeButtonHeight = (await page.locator('form[action="/notifications/purge"]').boundingBox()).height;
  expect(buttonHeight).toBe(purgeButtonHeight);
});

test('Button visuals', async ({browser}) => {
  async function getButtonProperties(page, selector) {
    return await page.locator(selector).evaluate((el) => {
      // In Firefox getComputedStyle is undefined if returned from evaluate
      const s = getComputedStyle(el);
      return {
        backgroundColor: s.backgroundColor,
        fontWeight: s.fontWeight,
        opacity: s.opacity,
        pointerEvents: s.pointerEvents,
        borderWidth: s.borderWidth,
      };
    });
  }

  const context = await browser.newContext({javaScriptEnabled: false});
  const page = await context.newPage();
  const response = await page.goto('/-/demo/buttons');
  expect(response?.status()).toBe(200);

  const transparent = 'rgba(0, 0, 0, 0)';

  const primary = await getButtonProperties(page, 'button.primary:not(.disabled)');
  const secondary = await getButtonProperties(page, 'button.secondary:not(.disabled)');
  const danger = await getButtonProperties(page, 'button.danger:not(.disabled)');

  for (const item of [primary, secondary, danger]) {
    // Evaluate that all buttons have background-color specified
    expect(item.backgroundColor).not.toBe(transparent);
    // Evaluate font weights
    expect(item.fontWeight).toBe('500');
    // Evaluate opacity
    expect(item.opacity).toBe('1');
    // Evaluate border width
    expect(item.borderWidth).toBe('1px');
  }

  // Evaluate that background-colors are different
  expect(primary.backgroundColor).not.toBe(secondary.backgroundColor);
  expect(primary.backgroundColor).not.toBe(danger.backgroundColor);

  const primaryDisabled = await getButtonProperties(page, '.button.primary.disabled');
  const secondaryDisabled = await getButtonProperties(page, '.button.secondary.disabled');
  const dangerDisabled = await getButtonProperties(page, '.button.danger.disabled');

  // Evaluate properties specific to disabled buttons
  for (const item of [primaryDisabled, secondaryDisabled, dangerDisabled]) {
    // Evaluate opacity
    expect(item.opacity).toBe('0.55');
    // Evaluate pointer-events
    expect(item.pointerEvents).toBe('none');

    // Evaluate other properties of non-disabled buttons
    expect(item.backgroundColor).not.toBe(transparent);
    expect(item.fontWeight).toBe('500');
  }

  // Evaluate outline from `:focus-visible` appearing
  for (const button of await page.locator('.page-content .button-sequence .button').all()) {
    // First, get to the element with .focus(), then activate :focus-visible by keyboard
    await button.focus();
    await page.keyboard.press('Tab');
    await page.keyboard.press('Shift+Tab');

    expect(await button.evaluate((el) => getComputedStyle(el).outlineWidth)).toBe('3px');
  }
});
