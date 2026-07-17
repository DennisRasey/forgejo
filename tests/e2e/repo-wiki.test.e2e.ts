// @watch start
// templates/repo/wiki/**
// web_src/css/repo**
// web_src/js/features/repo-wiki.js
// @watch end

import {expect} from '@playwright/test';
import {dynamic_id, test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

for (const searchTerm of ['space', 'consectetur']) {
  for (const width of [null, 2560, 4000]) {
    test(`Search for '${searchTerm}' and test for no overflow ${width && `on ${width}-wide viewport` || ''}`, async ({page, viewport}) => {
      await page.setViewportSize({
        width: width ?? viewport.width,
        height: 1440, // We're testing that we fit horizontally - vertical scrolling is fine.
      });
      await page.goto('/user2/repo1/wiki');
      await page.getByPlaceholder('Search wiki').fill(searchTerm);
      await page.getByPlaceholder('Search wiki').click();
      // workaround: HTMX listens on keyup events, playwright's fill only triggers the input event
      // so we manually "type" the last letter
      await page.getByPlaceholder('Search wiki').dispatchEvent('keyup');

      await expect(page.locator('#wiki-search a[href]')).toBeInViewport({
        ratio: 1,
      });
      await screenshot(page);
    });
  }
}

test(`Search results show titles (and not file names)`, async ({page}) => {
  await page.goto('/user2/repo1/wiki');
  await page.getByPlaceholder('Search wiki').fill('spaces');
  await page.getByPlaceholder('Search wiki').click();
  // workaround: HTMX listens on keyup events, playwright's fill only triggers the input event
  // so we manually "type" the last letter
  await page.getByPlaceholder('Search wiki').dispatchEvent('keyup');
  await expect(page.locator('#wiki-search a[href] b')).toHaveText('Page With Spaced Name');
  await screenshot(page);
});

test('Wiki unicode-escape', async ({page}) => {
  await page.goto('/user2/unicode-escaping/wiki');
  await screenshot(page);

  await expect(page.locator('.ui.message.unicode-escape-prompt')).toHaveCount(3);

  const unescapedElements = page.locator('.ambiguous-code-point');
  for (let i = 0; i < await unescapedElements.count(); i++) {
    expect(await unescapedElements.nth(i).evaluate((el) => getComputedStyle(el).border)).toEqual('0px solid rgb(24, 24, 27)');
  }

  await page.locator('a.escape-button').click();

  const escapedElements = page.locator('.ambiguous-code-point');
  for (let i = 0; i < await escapedElements.count(); i++) {
    expect(await escapedElements.nth(i).evaluate((el) => getComputedStyle(el).border)).toEqual('1px solid rgb(202, 138, 4)');
  }
});

test.describe('Wiki editing', () => {
  test.use({user: 'user2'});
  const wikiTitle = dynamic_id();

  test('New page', async ({page}) => {
    await page.goto('/user2/repo1/wiki?action=_new');

    await page.getByPlaceholder('Page title').fill(wikiTitle);
    await page.getByPlaceholder('Page content').fill('Welcome to the new wiki!');
    await page.getByText('Save page').click();

    await expect(page.locator('.file-content')).toContainText(wikiTitle);
    await expect(page.locator('.wiki-content-main')).toHaveText('Welcome to the new wiki!');
  });

  test('Edit page', async ({page}) => {
    await page.goto(`/user2/repo1/wiki/${wikiTitle}.-`);
    await page.getByRole('link', {name: 'Edit'}).click();

    await page.getByPlaceholder('Page content').fill('Wiki has been deprecated');
    await page.getByText('Save page').click();

    await expect(page.locator('.wiki-content-main')).toHaveText('Wiki has been deprecated');
  });
});
