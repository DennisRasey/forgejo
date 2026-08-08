// @watch start
// templates/org/team/new.tmpl
// templates/org/settings/repos.tmpl
// web_src/css/form.css
// web_src/js/features/org-team.js
// web_src/js/features/tablesort.js
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';
import {validate_form} from './shared/forms.ts';

test.use({user: 'user2'});

test('org team settings', async ({page}) => {
  const response = await page.goto('/org/org3/teams/team1/edit');
  expect(response?.status()).toBe(200);

  await page.locator('input[name="permission"][value="admin"]').click();
  await expect(page.locator('.hide-unless-checked')).toBeHidden();
  await screenshot(page);

  await page.locator('input[name="permission"][value="read"]').click();
  await expect(page.locator('.hide-unless-checked')).toBeVisible();
  await screenshot(page);

  // we are validating the form here to include the part that could be hidden
  await validate_form({page});
});

test('org add and remove team repositories', async ({page}) => {
  const response = await page.goto('/org/org3/teams/team1/repositories');
  expect(response?.status()).toBe(200);

  // As this is a shared state between multiple runs, always add repo3 to have some initial state.
  await page.getByPlaceholder('Search repos…').fill('repo3');
  await page.getByRole('button', {name: 'Add', exact: true}).click();
  await expect(page.getByText('org3/repo3')).toBeVisible();

  // Open remove all dialog.
  await page.getByRole('button', {name: 'Remove all'}).click();
  await expect(page.locator('#removeall-repos-modal')).toBeVisible();
  await screenshot(page, page.locator('#removeall-repos-modal'));
  // Remove all repositories.
  await page.getByRole('button', {name: 'Yes'}).click();

  // Check that all repositories are removed.
  await expect(page.getByText('No repositories could be accessed by this team.')).toBeVisible();

  // Open add all dialog.
  await page.getByRole('button', {name: 'Add all'}).click();
  await expect(page.locator('#addall-repos-modal')).toBeVisible();
  await screenshot(page, page.locator('#addall-repos-modal'));
  // Add all repositories.
  await page.getByRole('button', {name: 'Yes'}).click();

  // Check that there are three repositories.
  await expect(page.getByText('No repositories could be accessed by this team.')).toBeHidden();
  await expect(page.getByText('org3/repo3')).toBeVisible();
  await expect(page.getByText('org3/repo21')).toBeVisible();
  await expect(page.getByText('org3/repo5')).toBeVisible();
});

test('org settings repositories list sorting', async ({page}) => {
  const response = await page.goto('/org/org3/settings/repos');
  expect(response?.status()).toBe(200);

  const names = page.locator('table tbody tr td:first-child a');
  await expect(names).toHaveCount(3);

  // Clicking the name header sorts ascending, a second click reverses the order.
  await page.locator('th[data-sortt-asc="alphabetically"]').click();
  await page.waitForURL(/sort=alphabetically/);
  await expect(names).toHaveText(['repo21', 'repo3', 'repo5']);
  await screenshot(page);

  await page.locator('th[data-sortt-asc="alphabetically"]').click();
  await page.waitForURL(/sort=reversealphabetically/);
  await expect(names).toHaveText(['repo5', 'repo3', 'repo21']);

  // The size columns cannot change the order of the all-zero-sized fixture
  // repositories, but clicking them must still update the sort parameter.
  await page.locator('th[data-sortt-asc="gitsize"]').click();
  await page.waitForURL(/sort=gitsize/);
  await page.locator('th[data-sortt-asc="gitsize"]').click();
  await page.waitForURL(/sort=reversegitsize/);

  await page.locator('th[data-sortt-asc="lfssize"]').click();
  await page.waitForURL(/sort=lfssize/);
  await page.locator('th[data-sortt-asc="lfssize"]').click();
  await page.waitForURL(/sort=reverselfssize/);
  await expect(names).toHaveCount(3);
});
