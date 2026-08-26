// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// routers/web/repo/view.go
// templates/org/header.tmpl
// templates/repo/header.tmpl
// templates/repo/view_file.tmpl
// templates/shared/donation_button.tmpl
// templates/shared/funding.tmpl
// templates/shared/user/profile_big_avatar.tmpl
// web_src/css/modules/dialog.css
// web_src/css/modules/message.css
// web_src/css/repo/header.css
// web_src/css/user.css
// @watch end

import {expect, type Locator} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';
import {accessibilityCheck} from './shared/accessibility.ts';

async function expectFundingEntry(entry: Locator, expectedProvider: string, expectedTitle: string, expectedValue: string) {
  await expect(entry.locator('a')).toHaveAttribute('href', expectedValue);
  await expect(entry.locator('a')).toHaveText(expectedTitle);
  await expect(entry.locator('.icon')).toHaveAccessibleName(expectedProvider);
  if (expectedProvider === 'custom') {
    await expect(entry.locator('.icon > svg')).toContainClass('octicon-link');
  } else {
    await expect(entry.locator('.icon > svg')).toContainClass(`brand-${expectedProvider}`);
  }
}

const widthCases = [208, 310, 400, 600] as const;
for (const width of widthCases) {
  test(`Repo funding config: errors readable at ${width}px wide`, async ({browser}) => {
    const context = await browser.newContext({screen: {width, height: 600}});
    const page = await context.newPage();

    const response = await page.goto('/user2/funding_some_valid/src/branch/main/.forgejo/FUNDING.yml', {waitUntil: 'domcontentloaded'});
    expect(response?.status()).toBe(200);

    const errors = page.locator('.ui.error.message').filter({hasText: 'Errors parsing funding config:'});
    await expect(errors).toBeVisible();
    await expect(errors).toBeInViewport({ratio: 1});

    await accessibilityCheck({page}, ['.ui.error.message'], [], []);
  });
}

const accessibilityCases = [
  {kind: 'repo', url: '/user2/funding_basic_complete', heading: 'Donate to user2/funding_basic_complete'},
  {kind: 'user', url: '/user39', heading: 'Donate to User39'},
  {kind: 'org', url: '/org6', heading: 'Donate to Org Six'},
] as const;
for (const testCase of accessibilityCases) {
  test(`Donation button (${testCase.kind}): accessibility`, async ({page}) => {
    const response = await page.goto(testCase.url, {waitUntil: 'domcontentloaded'});
    expect(response?.status()).toBe(200);

    const donationButton = page.getByRole('button').filter({hasText: 'Donate'});
    await expect(donationButton).toBeVisible();
    await expect(donationButton).toHaveAccessibleName(testCase.heading);
    await expect(page.locator('#funding-modal')).toBeHidden();

    await accessibilityCheck({page}, ['button.donation'], [], []);
  });
}

test('Funding modal: accessibility (valid config)', async ({page}) => {
  const response = await page.goto('/user2/funding_basic_complete', {waitUntil: 'domcontentloaded'});
  expect(response?.status()).toBe(200);

  const fundingModal = page.locator('#funding-modal');
  await expect(fundingModal).toBeHidden();
  await page.getByRole('button').filter({hasText: 'Donate'}).click();
  await expect(fundingModal).toBeVisible();
  await expect(fundingModal.locator('.ui.error.message')).toBeHidden();

  await accessibilityCheck({page}, ['dialog#funding-modal'], [], []);
});

test('Funding modal: accessibility (config errors)', async ({page}) => {
  const response = await page.goto('/user2/funding_some_valid', {waitUntil: 'domcontentloaded'});
  expect(response?.status()).toBe(200);

  const fundingModal = page.locator('#funding-modal');
  await expect(fundingModal).toBeHidden();
  await page.getByRole('button').filter({hasText: 'Donate'}).click();
  await expect(fundingModal).toBeVisible();
  await expect(fundingModal.getByRole('heading')).toHaveText('Donate to user2/funding_some_valid');
  await expect(fundingModal.locator('.ui.error.message', {hasText: 'The funding config contains errors'})).toBeVisible();

  const items = fundingModal.getByRole('listitem');
  await expect(items).toHaveCount(1);
  await expectFundingEntry(items.nth(0), 'custom', 'https://example.com', 'https://example.com');

  await accessibilityCheck({page}, ['dialog#funding-modal'], [], []);
});

for (const testCase of [
  {
    kind: 'tall',
    name: 'funding_basic_complete',
  },
  {
    kind: 'wide',
    name: 'funding_with_a_really_ridiculously_long_title_that_doesnt_really_happen_all_that_often_normally_but_could_really_mess_with_things_if_not_handled_properly',
  },
] as const) {
  for (const width of widthCases) {
    test(`Funding modal (${testCase.kind}): usable at ${width}px wide`, async ({browser}) => {
      const context = await browser.newContext({screen: {width, height: 600}});
      const page = await context.newPage();

      const response = await page.goto(`/user2/${testCase.name}`, {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const fundingModal = page.locator('#funding-modal');
      await expect(fundingModal).toBeHidden();
      await page.getByRole('button').filter({hasText: 'Donate'}).click();
      await expect(fundingModal).toBeVisible();
      await expect(fundingModal.getByRole('heading')).toBeInViewport({ratio: 1}); // shouldn't have to scroll to access a scrolling modal!

      await expect(fundingModal.getByRole('heading')).toHaveText(`Donate to user2/${testCase.name}`);
      await expect(fundingModal.getByRole('heading')).toBeInViewport({ratio: 1}); // title should remain at least partly visible (perhaps shortened with ellipsis) unless we scroll

      await expect(fundingModal.locator('.ui.error.message')).toBeHidden();

      const item = fundingModal.getByRole('listitem').first();
      await expect(item).toBeInViewport({ratio: 1});

      const close = fundingModal.getByLabel('Close');
      await expect(close).toBeInViewport({ratio: 1});

      await accessibilityCheck({page}, ['dialog#funding-modal'], [], []);
    });
  }
}

for (const run of [
  {title: 'JS off', useJs: false},
  {title: 'JS on', useJs: true},
]) {
  test.describe(run.title, () => {
    test.use({javaScriptEnabled: run.useJs});

    test('Funding modal (repo)', async ({page}) => {
      // hidden on repo without funding config
      let response = await page.goto('/user2/long-diff-test', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);
      await expect(page.locator('#funding-modal')).toBeHidden();
      await expect(page.getByRole('button').filter({hasText: 'Donate'})).toBeHidden();

      // shown on repo with funding config
      response = await page.goto('/user2/funding_basic_complete', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const fundingModal = page.locator('#funding-modal');
      await expect(fundingModal).toBeHidden();
      await page.getByRole('button').filter({hasText: 'Donate'}).click();
      await expect(fundingModal).toBeVisible();
      await expect(fundingModal).toHaveAccessibleName('Donate to user2/funding_basic_complete');
      await expect(fundingModal.locator('.ui.error.message')).toBeHidden();

      const items = fundingModal.getByRole('listitem');
      await expect(items).toHaveCount(14); // the most we can have!
      await expectFundingEntry(items.nth(0), 'community_bridge', 'crowdfunding.linuxfoundation.org/initiatives/example', 'https://crowdfunding.linuxfoundation.org/initiatives/example');
      await expectFundingEntry(items.nth(1), 'github', 'github.com/sponsors/example', 'https://github.com/sponsors/example');
      await expectFundingEntry(items.nth(2), 'github', 'github.com/sponsors/example2', 'https://github.com/sponsors/example2');
      await expectFundingEntry(items.nth(3), 'issuehunt', 'issuehunt.io/r/example', 'https://issuehunt.io/r/example');
      await expectFundingEntry(items.nth(4), 'ko_fi', 'ko-fi.com/example', 'https://ko-fi.com/example');
      await expectFundingEntry(items.nth(5), 'ko_fi', 'ko-fi.com/example_2_electric_boogaloo', 'https://ko-fi.com/example_2_electric_boogaloo');
      await expectFundingEntry(items.nth(6), 'liberapay', 'liberapay.com/example', 'https://liberapay.com/example');
      await expectFundingEntry(items.nth(7), 'patreon', 'patreon.com/example', 'https://patreon.com/example');
      await expectFundingEntry(items.nth(8), 'open_collective', 'opencollective.com/example', 'https://opencollective.com/example');
      await expectFundingEntry(items.nth(9), 'buy_me_a_coffee', 'buymeacoffee.com/example', 'https://buymeacoffee.com/example');
      await expectFundingEntry(items.nth(10), 'thanks_dev', 'thanks.dev/u/gh/example', 'https://thanks.dev/u/gh/example');
      await expectFundingEntry(items.nth(11), 'tidelift', 'tidelift.com/funding/github/npm/example', 'https://tidelift.com/funding/github/npm/example');
      await expectFundingEntry(items.nth(12), 'custom', 'https://example.com', 'https://example.com');
      await expectFundingEntry(items.nth(13), 'custom', 'https://xn--e28h.com', 'https://xn--e28h.com');

      await screenshot(page);
    });

    const appearanceCases = [
      // unlike normal repo funding configs, user/org ones always say sposor their user/org, not the repo itself:
      {kind: 'user', badUrl: '/user2', goodUrl: '/user39', heading: 'Donate to User39'},
      {kind: 'user/.profile', badUrl: '/user2', goodUrl: '/user39/.profile', heading: 'Donate to User39'},
      {kind: 'org', badUrl: '/org25', goodUrl: '/org6', heading: 'Donate to Org Six'},
      {kind: 'org/.profile', badUrl: '/org25', goodUrl: '/org6/.profile', heading: 'Donate to Org Six'},
    ] as const;
    for (const testCase of appearanceCases) {
      test(`Funding modal (${testCase.kind}): appears when a profile has a valid funding config`, async ({page}) => {
        // user/org without a funding config has no special button
        let response = await page.goto(testCase.badUrl, {waitUntil: 'domcontentloaded'});
        expect(response?.status()).toBe(200);
        await expect(page.locator('#funding-modal')).toBeHidden();
        await expect(page.getByRole('button').filter({hasText: 'Donate'})).toBeHidden();

        // user/org with a funding config shows the button
        response = await page.goto(testCase.goodUrl, {waitUntil: 'domcontentloaded'});
        expect(response?.status()).toBe(200);

        const fundingModal = page.locator('#funding-modal');
        await expect(fundingModal).toBeHidden();
        const donationButton = page.getByRole('button').filter({hasText: 'Donate'});
        await expect(donationButton).toBeVisible();
        await donationButton.click();

        await expect(fundingModal).toBeVisible();
        await expect(fundingModal.getByRole('heading')).toHaveText(testCase.heading);

        const items = fundingModal.getByRole('listitem');
        await expect(items).toHaveCount(3);
        await expectFundingEntry(items.nth(0), 'ko_fi', 'ko-fi.com/example', 'https://ko-fi.com/example');
        await expectFundingEntry(items.nth(1), 'liberapay', 'liberapay.com/example', 'https://liberapay.com/example');
        await expectFundingEntry(items.nth(2), 'custom', 'http://localhost:3003/', 'http://localhost:3003/');
      });
    }

    test('Funding modal: closes on Esc', async ({page}) => {
      const response = await page.goto('/user2/funding_basic_complete', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const fundingModal = page.locator('#funding-modal');
      await expect(fundingModal).toBeHidden();
      await page.getByRole('button').filter({hasText: 'Donate'}).click();
      await expect(fundingModal).toBeVisible();

      await page.keyboard.press('Escape');
      await expect(fundingModal).toBeHidden();
    });

    test('Funding modal: closes on outside click', async ({page}) => {
      const response = await page.goto('/user2/funding_basic_complete', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const fundingModal = page.locator('#funding-modal');
      await expect(fundingModal).toBeHidden();
      await page.getByRole('button').filter({hasText: 'Donate'}).click();
      await expect(fundingModal).toBeVisible();

      // not sure if it's possible to select ::backdrop here, so we manually click just outside of the bounding box for the same effect
      const box = await fundingModal.boundingBox();
      await page.mouse.click(box.x + 2, box.y + 2); // clicking the modal itself does nothing
      await expect(fundingModal).toBeVisible();
      await page.mouse.click(box.x - 1, box.y);
      await expect(fundingModal).toBeHidden();
    });

    test('Funding modal: closes on Close button', async ({page}) => {
      const response = await page.goto('/user2/funding_basic_complete', {waitUntil: 'domcontentloaded'});
      expect(response?.status()).toBe(200);

      const fundingModal = page.locator('#funding-modal');
      await expect(fundingModal).toBeHidden();
      await page.getByRole('button').filter({hasText: 'Donate'}).click();
      await expect(fundingModal).toBeVisible();

      await page.getByLabel('Close').click();
      await expect(fundingModal).toBeHidden();
    });
  });
}
