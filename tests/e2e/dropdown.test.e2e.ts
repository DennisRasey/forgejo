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

test('JS enhanced interaction', async ({page}) => {
  await page.goto('/user1');

  await expect(page.locator('body')).not.toContainClass('no-js');
  const nojsNotice = page.locator('body .full noscript');
  await expect(nojsNotice).toBeHidden();

  // Open and close by clicking summary
  const selectorPrefix = '#profile-avatar-card details.dropdown';
  const dropdown = page.locator(selectorPrefix);
  const dropdownSummary = page.locator(`${selectorPrefix} > summary`);
  const dropdownContent = page.locator(`${selectorPrefix} > .content`);
  await expect(dropdownContent).toBeHidden();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeHidden();

  // Close by clicking elsewhere
  const elsewhere = page.locator('.username');
  await expect(dropdownContent).toBeHidden();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeVisible();
  await elsewhere.click();
  await expect(dropdownContent).toBeHidden();

  // Open and close with keypressing
  await dropdownSummary.focus();
  // Open with Enter, close with Space
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Space`);
  await expect(dropdownContent).toBeHidden();
  // Open with Space, close with Enter
  await dropdownSummary.press(`Space`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeHidden();
  // Open with Enter, close with Enter
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Escape`);
  await expect(dropdownContent).toBeHidden();

  // Open and navigate with ArrowDown, close with Tab
  await dropdownSummary.focus();
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

  // Navigate and close with Shift+Tab
  await dropdownSummary.focus();
  await dropdown.press(`Enter`);
  await expect(dropdownSummary).toBeFocused();
  await dropdown.press(`ArrowDown`);
  await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
  await dropdown.press('Shift+Tab');
  await expect(dropdownSummary).toBeFocused();
  await dropdown.press('Shift+Tab');
  await expect(dropdownContent).toBeHidden();

  // Navigate with ArrowUp
  await dropdownSummary.focus();
  await dropdown.press(`ArrowDown`);
  await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
  await dropdown.press(`ArrowDown`);
  await expect(page.locator(`a[href$=".atom"]`)).toBeFocused();
  await dropdown.press(`ArrowUp`);
  await expect(page.locator(`a[href$=".rss"]`)).toBeFocused();
  // Pressing ArrowUp on first item will move us to summary, but no farther from here
  await dropdown.press(`ArrowUp`);
  await expect(dropdownSummary).toBeFocused();
  await dropdown.press(`Escape`);
  await expect(dropdownContent).toBeHidden();

  // Open and then close by opening a different dropdown
  const languageMenu = page.locator('.language-menu');
  await dropdownSummary.click();
  await expect(dropdownContent).toBeVisible();
  await expect(languageMenu).toBeHidden();
  await page.locator('.language.dropdown').click();
  await expect(dropdownContent).toBeHidden();
  await expect(languageMenu).toBeVisible();
});

test('No JS interaction', async ({browser}) => {
  const context = await browser.newContext({javaScriptEnabled: false});
  const nojsPage = await context.newPage();
  await nojsPage.goto('/user1');

  const nojsNotice = nojsPage.locator('body .full noscript');
  await expect(nojsNotice).toBeVisible();
  await expect(nojsPage.locator('body')).toContainClass('no-js');

  // Open and close by clicking summary
  const selectorPrefix = '#profile-avatar-card details.dropdown';
  const dropdownSummary = nojsPage.locator(`${selectorPrefix} > summary`);
  const dropdownContent = nojsPage.locator(`${selectorPrefix} > .content`);
  await expect(dropdownContent).toBeHidden();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeHidden();

  // Close by clicking elsewhere (by hitting ::before with increased z-index)
  const elsewhere = nojsPage.locator('#navbar');
  await expect(dropdownContent).toBeHidden();
  await dropdownSummary.click();
  await expect(dropdownContent).toBeVisible();
  // eslint-disable-next-line playwright/no-force-option
  await elsewhere.click({force: true});
  await expect(dropdownContent).toBeHidden();

  // Open and close with keypressing
  // Open with Enter, close with Space
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Space`);
  await expect(dropdownContent).toBeHidden();
  // Open with Space, close with Enter
  await dropdownSummary.press(`Space`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeHidden();
  // Closing by Escape is not possible w/o JS enhancements
  await dropdownSummary.press(`Enter`);
  await expect(dropdownContent).toBeVisible();
  await dropdownSummary.press(`Escape`);
  await expect(dropdownContent).toBeVisible();
});

test.describe(`Visual properties`, () => {
  async function evaluateDropdownItems(page, selector, direction, height) {
    const computedStyles = await page.locator(selector).evaluateAll((items) =>
      items.map((item) => {
        const s = getComputedStyle(item);
        return {
          direction: s.direction,
          height: s.height,
        };
      }),
    );
    for (const cs of computedStyles) {
      expect(cs.direction).toBe(direction);
      expect(cs.height).toBe(height);
    }
  }

  test('User profile', async ({browser, isMobile}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // User profile has dropdown used as an ellipsis menu
    await page.goto('/user1');
    const selectorPrefix = '#profile-avatar-card details.dropdown';
    const summary = page.locator(`${selectorPrefix} > summary`);

    // Has `.border` and pretty small default `inline-padding:`
    // Note: `getComputedStyle` can return `border` as 1±0.1px when `Show browser` is enabled
    expect(await summary.evaluate((el) => getComputedStyle(el).border)).toBe('1px solid rgba(0, 0, 0, 0.114)');
    expect(await summary.evaluate((el) => getComputedStyle(el).paddingInline)).toBe('7px');

    // Has a tooltip. Only translates into an aria-label w/ JS
    await expect(summary).toHaveAttribute('data-tooltip-content', 'More actions');

    // Background
    expect(await summary.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
    await summary.click();
    expect(await summary.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(226, 226, 229)');

    // Direction and item height
    if (isMobile) {
      // `<ul>`'s direction is reversed
      expect(await page.locator(`${selectorPrefix} > .content`).evaluate((el) => getComputedStyle(el).direction)).toBe('rtl');
      // `@media (pointer: coarse)` makes items taller
      await evaluateDropdownItems(page, `${selectorPrefix} > .content > ul > li`, 'ltr', '40px');
    } else {
      // Both use default direction
      expect(await page.locator(`${selectorPrefix} > .content`).evaluate((el) => getComputedStyle(el).direction)).toBe('ltr');
      // Regular item height
      await evaluateDropdownItems(page, `${selectorPrefix} > .content > ul > li`, 'ltr', '34px');
    }
  });

  test('Explore sort', async ({browser, isMobile}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // `/explore/users` has dropdown used as a sort options menu with text in the opener
    await page.goto('/explore/users');
    const selectorPrefix = '.list-header details.dropdown';
    const summary = page.locator(`${selectorPrefix} > summary`);
    await summary.click();

    // No `.border` and increased `inline-padding:` from `.options`
    expect(await summary.evaluate((el) => getComputedStyle(el).borderWidth)).toBe('0px');
    expect(await summary.evaluate((el) => getComputedStyle(el).paddingInline)).toBe('10.5px');

    // `<ul>`'s direction is reversed
    expect(await page.locator(`${selectorPrefix} > .content`).evaluate((el) => getComputedStyle(el).direction)).toBe('rtl');
    await evaluateDropdownItems(page, `${selectorPrefix} > .content > ul > li`, 'ltr', isMobile ? '40px' : '34px');

    // Background of inactive and `.active` items
    const activeItem = page.locator(`${selectorPrefix}> .content > ul > li:first-child > a`);
    const inactiveItem = page.locator(`${selectorPrefix}> .content > ul > li:last-child > a`);
    expect(await activeItem.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(226, 226, 229)');
    expect(await inactiveItem.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
  });

  test('Demo page', async ({browser}) => {
    const context = await browser.newContext({javaScriptEnabled: false});
    const page = await context.newPage();

    // `/-/demo` has dropdowns with various combinations of items
    await page.goto('/-/demo/dropdown');

    // Dropdown with just 3 items and nothing special
    await page.locator(`#dropdown-1 > summary`).click();
    expect(await page.locator(`#dd1_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px 4px 0px 0px');
    expect(await page.locator(`#dd1_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd1_g1_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with two groups of items separated with an <hr>
    await page.locator(`#dropdown-2 > summary`).click();
    expect(await page.locator(`#dd2_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px 4px 0px 0px');
    expect(await page.locator(`#dd2_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g1_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd2_g2_i3`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with only one item, which should be completely round
    await page.locator(`#dropdown-3 > summary`).click();
    expect(await page.locator(`#dd3_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('4px');
    await page.keyboard.press('Enter'); // Exit dropdown - page is in noJS mode

    // Dropdown with additional content and a HR - which the very first item should take into consideration
    await page.locator(`#dropdown-5 > summary`).click();
    expect(await page.locator(`#dd5_g1_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g1_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g2_i1`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px');
    expect(await page.locator(`#dd5_g2_i2`).evaluate((el) => getComputedStyle(el).borderRadius)).toBe('0px 0px 4px 4px');
  });
});
