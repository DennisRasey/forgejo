// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/shared/user/actions_menu.tmpl
// templates/org/header.tmpl
// templates/explore/search.tmpl
// templates/demo/dropdown.tmpl
// web_src/js/modules/dropdown.ts
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

for (const run of [
  {title: 'JS off', useJs: false},
  {title: 'JS on', useJs: true},
]) {
  test.describe(run.title, () => {
    test.use({javaScriptEnabled: run.useJs});

    const selectorPrefix = '#profile-avatar-card .dialog-dropdown';

    test('Click open/close', async ({page}) => {
      await page.goto('/user1');

      // Open and close by clicking opener
      const opener = page.locator(`${selectorPrefix} > .opener`);
      const dropdownContent = page.locator(`${selectorPrefix} > dialog`);
      await expect(dropdownContent).toBeHidden();
      await opener.click();
      await expect(dropdownContent).toBeVisible();
      await opener.click();
      await expect(dropdownContent).toBeHidden();

      // Open by clicking opener, then close by clicking elsewhere
      const elsewhere = page.locator('.username');
      await expect(dropdownContent).toBeHidden();
      await opener.click();
      await expect(dropdownContent).toBeVisible();
      await elsewhere.click();
      await expect(dropdownContent).toBeHidden();
    });

    test('Enter/Space/Escape open/close', async ({page}) => {
      await page.goto('/user1');

      const opener = page.locator(`${selectorPrefix} > .opener`);
      const dropdownContent = page.locator(`${selectorPrefix} > dialog`);

      await opener.focus();
      // Open with Enter, close with Space
      await opener.press(`Enter`);
      await expect(dropdownContent).toBeVisible();
      await opener.press(`Space`);
      await expect(dropdownContent).toBeHidden();

      // Open with Space, close with Enter
      await opener.press(`Space`);
      await expect(dropdownContent).toBeVisible();
      await opener.press(`Enter`);
      await expect(dropdownContent).toBeHidden();

      // Open with Enter, close with Enter
      await opener.press(`Enter`);
      await expect(dropdownContent).toBeVisible();
      await opener.press(`Escape`);
      await expect(dropdownContent).toBeHidden();
    });

    test('Close by opening a different dropdown', async ({page}) => {
      await page.goto('/user1');

      const opener = page.locator(`${selectorPrefix} > .opener`);
      const dropdownContent = page.locator(`${selectorPrefix} > dialog`);

      // Open and then close by opening a different dropdown
      const languageMenu = page.locator('.language-menu');
      await opener.click();
      await expect(dropdownContent).toBeVisible();
      await expect(languageMenu).toBeHidden();
      await page.locator('.language.dropdown').click();
      await expect(dropdownContent).toBeHidden();
      if (run.useJs) {
        // languageMenu won't open w/o JS because it is a legacy dropdown
        await expect(languageMenu).toBeVisible();
      }
    });

    test('Tab navigation', async ({page}, workerInfo) => {
      test.skip(!run.useJs, 'Proper "close on Shift+Tab" relies on focusout JS event');

      await page.goto('/user1');

      const opener = page.locator(`${selectorPrefix} > .opener`);
      const dropdown = page.locator(selectorPrefix);
      const dropdownContent = page.locator(`${selectorPrefix} > dialog`);

      // Navigate and close with Shift+Tab
      await opener.focus();
      await dropdown.press(`Enter`);
      await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
      await dropdown.press('Shift+Tab');
      if (workerInfo.project.name === 'firefox') {
        // Firefox focuses <dialog> first before going to opener. This is unwanted but adding
        // role="..." doesn't help, so navigation just requires an extra combination for now
        await dropdown.press('Shift+Tab');
      }
      await expect(opener).toBeFocused();
      await dropdown.press('Shift+Tab');
      if (workerInfo.project.name === 'firefox') {
        // Ditto
        await dropdown.press('Shift+Tab');
      }
      await expect(dropdownContent).toBeHidden();
    });

    test('Arrow key interaction', async ({page}) => {
      test.skip(!run.useJs, 'Arrow key interaction is JS only functionality');

      await page.goto('/user1');

      const opener = page.locator(`${selectorPrefix} > .opener`);
      const dropdown = page.locator(selectorPrefix);
      const dropdownContent = page.locator(`${selectorPrefix} > dialog`);

      // Open and navigate with ArrowDown, close with ArrowUp
      await opener.focus();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".atom"]`)).toBeFocused();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".keys"]`)).toBeFocused();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".gpg"]`)).toBeFocused();
      // ArrowDown won't move us farther than the last dropdown item
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".gpg"]`)).toBeFocused();
      // Pressing Tab on last item will move us away from the dropdown and close the dropdown
      await dropdown.press(`Tab`);
      await expect(dropdownContent).toBeHidden();

      // Open with ArrowDown, close with with ArrowUp
      await opener.focus();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
      await dropdown.press(`ArrowDown`);
      await expect(page.locator(`a[href$=".atom"]`)).toBeFocused();
      await dropdown.press(`ArrowUp`);
      await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
      // Pressing ArrowUp on first item will move us to opener, but no farther from here
      await dropdown.press(`ArrowUp`);
      await expect(opener).toBeFocused();
      await dropdown.press(`Escape`);
      await expect(dropdownContent).toBeHidden();
    });
  });
}

test.describe(`Visual properties`, () => {
  test('User profile', async ({browser, isMobile}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // User profile has dropdown used as an ellipsis menu
    await page.goto('/user1');

    const selectorPrefix = '#profile-avatar-card .dialog-dropdown';
    const opener = page.locator(`${selectorPrefix} > .opener`);
    const item = page.locator(`${selectorPrefix} > dialog > ul > li:first-child`);

    // Has `.border` and pretty small default `inline-padding:`
    // Note: `getComputedStyle` can return `border` as 1±0.1px when `Show browser` is enabled
    expect(await opener.evaluate((el) => getComputedStyle(el).border)).toBe('1px solid rgba(0, 0, 0, 0.114)');
    expect(await opener.evaluate((el) => getComputedStyle(el).paddingInline)).toBe('7px');

    // Has a tooltip. Only translates into an aria-label w/ JS
    await expect(opener).toHaveAttribute('data-tooltip-content', 'More actions');

    // Background
    expect(await opener.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
    await opener.click();
    expect(await opener.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(226, 226, 229)');

    // Direction and item height
    if (isMobile) {
      // `@media (pointer: coarse)` makes items taller
      expect(await item.evaluate((el) => getComputedStyle(el).height)).toBe('40px');
    } else {
      // Regular item height
      expect(await item.evaluate((el) => getComputedStyle(el).height)).toBe('34px');
    }
  });

  test('Explore sort', async ({browser}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // `/explore/users` has dropdown used as a sort options menu with text in the opener
    await page.goto('/explore/users');
    const selectorPrefix = '.list-header .dialog-dropdown';
    const opener = page.locator(`${selectorPrefix} > .opener`);
    await opener.click();

    // No `.border` and increased `inline-padding:` from `.options`
    expect(await opener.evaluate((el) => getComputedStyle(el).borderWidth)).toBe('0px');
    expect(await opener.evaluate((el) => getComputedStyle(el).paddingInline)).toBe('10.5px');

    // Background of inactive and `.active` items
    const activeItem = page.locator(`${selectorPrefix} > dialog > ul > li:first-child > a`);
    const inactiveItem = page.locator(`${selectorPrefix} > dialog > ul > li:last-child > a`);
    expect(await activeItem.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(226, 226, 229)');
    expect(await inactiveItem.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
  });

  test('Demo page', async ({browser}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // `/-/demo` has dropdowns with various combinations of items
    await page.goto('/-/demo/dropdown');

    // Dropdown with just 3 items and nothing special
    await page.locator(`.opener[commandfor="dropdown-1"]`).click();
    expect(await page.locator(`#dd1_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px 4px 0px 0px');
    expect(await page.locator(`#dd1_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd1_g1_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with two groups of items separated with an <hr>
    await page.locator(`.opener[commandfor="dropdown-2"]`).click();
    expect(await page.locator(`#dd2_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px 4px 0px 0px');
    expect(await page.locator(`#dd2_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g1_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with only one item, which should be completely round
    await page.locator(`.opener[commandfor="dropdown-3"]`).click();
    expect(await page.locator(`#dd3_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with additional content and a HR - which the very first item should take into consideration
    await page.locator(`.opener[commandfor="dropdown-5"]`).click();
    expect(await page.locator(`#dd5_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g2_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g2_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
  });
});
