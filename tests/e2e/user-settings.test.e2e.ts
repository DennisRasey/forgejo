// @watch start
// templates/user/settings/**.tmpl
// web_src/css/{form,user}.css
// @watch end

import {expect, type Page} from '@playwright/test';
import {test, login_user, login} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';
import {validate_form} from './shared/forms.ts';

test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});

test('User: Profile settings', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings');

  await page.getByLabel('Full name').fill('SecondUser');

  const pronounsInput = page.locator('input[list="pronouns"]');
  await expect(pronounsInput).toHaveAttribute('placeholder', 'Unspecified');
  await pronounsInput.click();
  const pronounsList = page.locator('datalist#pronouns');
  const pronounsOptions = pronounsList.locator('option');
  const pronounsValues = await pronounsOptions.evaluateAll((opts) => opts.map((opt: HTMLOptionElement) => opt.value));
  expect(pronounsValues).toEqual(['he/him', 'she/her', 'they/them', 'it/its', 'any pronouns']);
  await pronounsInput.fill('she/her');

  await page.getByPlaceholder('Tell others a little bit').fill('I am a playwright test running for several seconds.');
  await page.getByPlaceholder('Tell others a little bit').press('Tab');
  await page.getByLabel('Website').fill('https://forgejo.org');
  await page.getByPlaceholder('Share your approximate').fill('on a computer chip');
  await page.getByLabel('Profile visibility').click();
  await page.getByLabel('Visible only to signed-in').click();
  await page.getByLabel('Hide email address Email address will').uncheck();
  await page.getByLabel('Hide activity from profile').check();

  await validate_form({page}, 'fieldset');
  await screenshot(page);
  await page.getByRole('button', {name: 'Update profile'}).click();
  await expect(page.getByText('Your profile has been updated.')).toBeVisible();
  await page.getByRole('link', {name: 'public activity'}).click();
  await expect(page.getByText('Your activity is only visible')).toBeVisible();
  await screenshot(page);

  await page.goto('/user2');
  await expect(page.getByText('SecondUser')).toBeVisible();
  await expect(page.getByText('on a computer chip')).toBeVisible();
  await expect(page.locator('li').filter({hasText: 'user2@example.com'})).toBeVisible();
  await expect(page.locator('li').filter({hasText: 'https://forgejo.org'})).toBeVisible();
  await expect(page.getByText('I am a playwright test')).toBeVisible();
  await screenshot(page);

  await page.goto('/user/settings');
  await page.locator('input[list="pronouns"]').fill('rob/ot');
  await page.getByLabel('Profile visibility').click();
  await page.getByLabel('Visible to everyone').click();
  await page.getByLabel('Hide email address Email address will').check();
  await page.getByLabel('Hide activity from profile').uncheck();
  await expect(page.getByText('Your profile has been updated.')).toBeHidden();
  await validate_form({page}, 'fieldset');
  await screenshot(page);
  await page.getByRole('button', {name: 'Update profile'}).click();
  await expect(page.getByText('Your profile has been updated.')).toBeVisible();

  await page.goto('/user2');
  await expect(page.getByText('SecondUser')).toBeVisible();
  await expect(page.locator('li').filter({hasText: 'user2@example.com'})).toBeHidden();
  await page.goto('/user2?tab=activity');
  await expect(page.getByText('Your activity is visible to everyone')).toBeVisible();
});

test('User: Storage overview', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/storage_overview');
  await page.waitForLoadState();
  await page.getByLabel('Git LFS – 8 KiB').nth(1).hover({position: {x: 250, y: 2}});
  await expect(page.getByText('Git LFS – 8 KiB')).toBeVisible();

  // Show/hide legend by clicking on the bar
  await expect(page.locator('.stats ul').nth(1)).toBeHidden();
  await expect(page.getByText('Git LFS 8 KiB').nth(1)).toBeHidden();

  await page.locator('.stats summary').nth(1).click();
  await expect(page.locator('.stats ul').nth(1)).toBeVisible();
  await expect(page.getByText('Git LFS 8 KiB').nth(1)).toBeVisible();
  await screenshot(page);

  await page.locator('.stats summary').nth(1).click();
  await expect(page.locator('.stats ul').nth(1)).toBeHidden();
  await expect(page.getByText('Git LFS 8 KiB').nth(1)).toBeHidden();

  await screenshot(page);
});

test('User: Canceling adding SSH key clears inputs', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/keys');
  await page.locator('#add-ssh-button').click();

  await page.getByLabel('Key name').fill('MyAwesomeKey');
  await page.locator('#ssh-key-content').fill('Wront key material');

  await page.getByRole('button', {name: 'Cancel'}).click();
  await page.locator('#add-ssh-button').click();

  const keyName = page.getByLabel('Key name');
  await expect(keyName).toHaveValue('');

  const content = page.locator('#ssh-key-content');
  await expect(content).toHaveValue('');
});

test('User: Canceling adding GPG key clears input', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/keys');
  await page.locator('.show-panel[data-panel="#add-gpg-key-panel"]').click();

  const gpgKeyContent = page.locator('#gpg-key-content');
  await gpgKeyContent.fill('Wront key material');

  await page.locator('.hide-panel[data-panel="#add-gpg-key-panel"]').click();

  await expect(gpgKeyContent).toHaveValue('');
});

test('User: Add access token', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/applications');
  await page.getByRole('link', {name: 'New access token'}).click();

  await page.locator('#scoped-access-submit').click();
  await page.locator('#name:invalid').isVisible();

  await page.selectOption('#access-token-scope-activitypub', 'read:activitypub');
  await page.locator('#scoped-access-submit').click();

  await page.locator('#name:invalid').isVisible();
  await expect(page.locator('#access-token-scope-activitypub')).toHaveValue('read:activitypub');

  const tokenName = globalThis.crypto.randomUUID();
  await page.locator('#name').fill(tokenName);
  await page.getByRole('radio', {name: /^All /}).click();
  await page.locator('#scoped-access-submit').click();

  await expect(page.locator('.ui.info.message.flash-info')).toBeVisible();
  const flashText = await page.locator('.ui.info.message.flash-info').textContent();
  expect(flashText?.trim()).toMatch(/^[0-9a-f]{40}$/);
  await page.getByText(tokenName).isVisible();
});

test('User: Add access token validation error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/applications');
  await page.getByRole('link', {name: 'New access token'}).click();

  await page.getByRole('button', {name: 'Generate token'}).click();
  await page.locator('#name:invalid').isVisible();

  await page.getByRole('textbox', {name: 'Token name *'}).fill('Token A');
  await page.getByRole('combobox', {name: 'activitypub'}).selectOption('read:activitypub');
  await page.getByRole('radio', {name: 'Public only'}).click();

  await page.getByRole('button', {name: 'Generate token'}).click();

  await page.getByText('has been used as an application name already.').isVisible();
  // validate that selected options (public-only, activitypub) are still selected after the validation error.
  await expect(page.getByRole('radio', {name: 'Public only'})).toBeChecked();
  await expect(page.getByRole('combobox', {name: 'activitypub'})).toHaveValue('read:activitypub');
});

test('User: Add specific repo access token', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/applications');
  await page.getByRole('link', {name: 'New access token'}).click();

  const tokenName = globalThis.crypto.randomUUID();
  await page.getByRole('textbox', {name: /^Token name/}).fill(tokenName);
  await page.getByRole('combobox', {name: 'repository'}).selectOption('read:repository');

  // clicking specific repositories will display currently available repositories:
  await expect(page.getByText('org17/big_test_private_4')).toBeHidden();
  await page.getByRole('radio', {name: 'Specific repositories'}).click();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await expect(page.getByText('user2/commits_search_test')).toBeVisible(); // another repo, will be used to verify search worked

  await page.getByPlaceholder('Search repos…').fill('big_test_private_4');
  await page.getByRole('button', {name: 'Search…'}).click();

  // verify search results visible:
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await expect(page.getByText('user2/commits_search_test')).toBeHidden();

  // after performing a search, verify that the token name, 'selected repositories', and selected permissions are maintained
  await expect(page.getByRole('textbox', {name: /^Token name/})).toHaveValue(tokenName);
  await expect(page.getByRole('radio', {name: 'Specific repositories'})).toBeChecked();
  await expect(page.getByRole('combobox', {name: 'repository'})).toHaveValue('read:repository');
  await expect(page.getByPlaceholder('Search repos…')).toHaveValue('big_test_private_4'); // search box still populated after reload

  // Add the big_test_private_4 repo.
  await page.getByRole('button', {name: 'Add org17/big_test_private_4'}).click();
  await expect(page.getByText('Selected repository (1)')).toBeVisible();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();

  // Remove it to test remove, and then re-add
  await page.getByRole('button', {name: 'Remove org17/big_test_private_4'}).click();
  await expect(page.getByText('Selected repositories (0)')).toBeVisible();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await page.getByRole('button', {name: 'Add org17/big_test_private_4'}).click();

  // Create the token and check for success.
  await page.getByRole('button', {name: 'Generate token'}).click();
  await expect(page.locator('.ui.info.message.flash-info')).toBeVisible();
  const flashText = await page.locator('.ui.info.message.flash-info').textContent();
  expect(flashText?.trim()).toMatch(/^[0-9a-f]{40}$/);
  await page.getByText(tokenName).isVisible();
});

// Test that validation errors on the repo-specific access token page retain all the entered field values when the
// error is displayed.
test('User: Add specific repo access token error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/applications');
  await page.getByRole('link', {name: 'New access token'}).click();

  await page.getByRole('textbox', {name: /^Token name/}).fill('Token A');
  await page.getByRole('combobox', {name: 'repository'}).selectOption('read:repository');
  await page.getByRole('radio', {name: 'Specific repositories'}).click();
  await page.getByRole('button', {name: 'Add org17/big_test_private_4'}).click();

  // Create the token, verify error, then check all the fields for retained values.
  await page.getByRole('button', {name: 'Generate token'}).click();
  await page.getByText('has been used as an application name already.').isVisible();

  await expect(page.getByRole('textbox', {name: /^Token name/})).toHaveValue('Token A');
  await expect(page.getByRole('radio', {name: 'Specific repositories'})).toBeChecked();
  await expect(page.getByRole('combobox', {name: 'repository'})).toHaveValue('read:repository');
  await expect(page.getByRole('button', {name: 'Remove org17/big_test_private_4'})).toBeVisible();
});

test('User: List authorized integrations', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  // Check for fixture data; check has to be safe for the presence of other authorized integrations
  // created by previous test runs.
  await expect(page.locator('.flex-item-title').filter({hasText: 'Example AI'})).not.toHaveCount(0);
  await expect(page.locator('.flex-item-body').filter({hasText: 'Added on 2026-05-16'})).not.toHaveCount(0);
  await expect(page.locator('.flex-item-body').filter({hasText: 'No recent activity'})).not.toHaveCount(0);
});

async function validateClaimRules(page: Page, expected: string) {
  await expect(async () => {
    const internal = await page.evaluate(() => Array.from(window.codeEditors)[0].state.doc.toString());
    expect(internal).toStrictEqual(expected);
  }).toPass({timeout: 3000});
  await expect(page.locator('#claim_rules')).toHaveValue(expected);
}

async function editFixtureAuthorizedIntegration(page: Page) {
  // When tests are run on multiple platforms, more than one authorized integration will be present from the "Add"
  // tests that don't have a way to cleanup after themselves (no delete capability yet); find the right target
  // to edit:
  await page.locator('.flex-item')
    .filter({has: page.locator('.flex-item-title', {hasText: 'Example AI'})})
    .getByRole('link', {name: 'Edit'}).click();
}

test('User: View authorized integration', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);

  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('Example AI');
  await expect(page.getByRole('textbox', {name: 'Description'})).toHaveValue('This is an authorized integration.\nThis example is just for viewing and editing.');
  await expect(page.getByRole('textbox', {name: 'Audience (aud Claim)'})).toHaveValue('u:2:7a6a47fb-6252-48b2-b0bb-e39158b11a36');
  await expect(page.getByRole('textbox', {name: 'Issuer (iss Claim)'})).toHaveValue('urn:forgejo:authorized-integrations:actions');

  // Claim rules JSON codemirror editor:
  const editor = page.locator('.cm-content');
  await expect(editor).toHaveAttribute('data-language', 'json', {timeout: 3000});
  await validateClaimRules(page, '{\n  "rules": null\n}');

  await expect(page.getByRole('radio', {name: 'All (public, private, and limited)'})).toBeChecked();
  await expect(page.getByRole('radio', {name: 'Public only'})).not.toBeChecked();
  await expect(page.getByRole('radio', {name: 'Specific repositories'})).not.toBeChecked();

  await expect(page.getByRole('combobox', {name: 'issue'})).toHaveValue('read:issue');
  await expect(page.getByRole('combobox', {name: 'repository'})).toHaveValue('write:repository');
  await expect(page.getByRole('combobox', {name: 'user'})).toHaveValue('');
  await expect(page.getByRole('combobox', {name: 'admin'})).toBeHidden(); // not an admin user
});

test('User: Edit authorized integration basic fields', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);

  await page.getByRole('textbox', {name: 'Name'}).fill('Example AI (Updated!)');
  await page.getByRole('textbox', {name: 'Description'}).fill('Updated by Edit authorized integration basic field test');

  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  // Returns to the list page; validate the updated name is present, and that it isn't marked
  // as "used" just because it was edited:
  await expect(page.locator('.flex-item-title').filter({hasText: 'Example AI (Updated!)'})).not.toHaveCount(0);
  await expect(page.locator('.flex-item-body').filter({hasText: 'Added on 2026-05-16'})).not.toHaveCount(0);
  await expect(page.locator('.flex-item-body').filter({hasText: 'No recent activity'})).not.toHaveCount(0);

  // Reopen to check description:
  await editFixtureAuthorizedIntegration(page);
  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('Example AI (Updated!)');
  await expect(page.getByRole('textbox', {name: 'Description'})).toHaveValue('Updated by Edit authorized integration basic field test');

  // Restore values to avoid affecting other tests and other platforms:
  await page.getByRole('textbox', {name: 'Name'}).fill('Example AI');
  await page.getByRole('textbox', {name: 'Description'}).fill('This is an authorized integration.\nThis example is just for viewing and editing.');
  await page.getByRole('button', {name: 'Save authorized integration'}).click();
  await expect(page.locator('.flex-item-title').filter({hasText: 'Example AI'})).not.toHaveCount(0); // ensure save completes and we land on list page
});

test('User: Edit authorized integration basic fields validation error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);
  await page.getByRole('textbox', {name: 'Name'}).fill('\t'); // trims to empty
  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  await expect(page.locator('.flash-error')).toContainText('Authorized integration name is required.');
  await expect(page.getByRole('textbox', {name: 'Name'}).locator('..')).toHaveClass('required field error');
});

test('User: Edit authorized integration issuer validation error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);
  await page.getByRole('textbox', {name: 'Issuer (iss Claim)'}).fill('ftp://example.org'); // designed to hit "unsupported URL scheme" error, no external traffic involved
  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  await expect(page.locator('.flash-error')).toContainText(/Issuer validation failed:/);
  await expect(page.getByRole('textbox', {name: 'Issuer (iss Claim)'}).locator('..')).toHaveClass('required field error');
});

test('User: Edit authorized integration claim rules', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);

  const editor = page.locator('.cm-content');
  await editor.click(); // Focus codemirror editor
  await page.keyboard.press('ControlOrMeta+A'); // select all
  await page.keyboard.press('Backspace'); // delete
  await page.keyboard.type('{"rules": [{"claim": "sub", "compare": "eq", "value": "a subject"}]}', {delay: 10});

  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  // Reopen to check claim rules saved:
  await editFixtureAuthorizedIntegration(page);
  await validateClaimRules(page, '{\n  "rules": [\n    {\n      "claim": "sub",\n      "compare": "eq",\n      "value": "a subject"\n    }\n  ]\n}');

  // Restore values to avoid affecting other tests and other platforms:
  await editor.click(); // Focus codemirror editor
  await page.keyboard.press('ControlOrMeta+A'); // select all
  await page.keyboard.press('Backspace'); // delete
  await page.keyboard.type('{"rules": null}', {delay: 10});
  await page.getByRole('button', {name: 'Save authorized integration'}).click();
  await expect(page.locator('.flex-item-title').filter({hasText: 'Example AI'})).not.toHaveCount(0); // ensure save completes and we land on list page
});

test('User: Edit authorized integration claim rules validation error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);

  const editor = page.locator('.cm-content');
  await editor.click(); // Focus codemirror editor
  await page.keyboard.type('{{{{{{', {delay: 10}); // type some incomplete garbage at the end
  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  await expect(page.locator('.flash-error')).toContainText(/Claim Rules validation failed:/);
});

test('User: Edit authorized integration specific repo', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await editFixtureAuthorizedIntegration(page);

  // clicking specific repositories will display currently available repositories:
  await expect(page.getByText('org17/big_test_private_4')).toBeHidden();
  await page.getByRole('radio', {name: 'Specific repositories'}).click();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await expect(page.getByText('user2/commits_search_test')).toBeVisible(); // another repo, will be used to verify search worked

  await page.getByPlaceholder('Search repos…').fill('big_test_private_4');
  await page.getByRole('button', {name: 'Search…'}).click();

  // verify search results visible:
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await expect(page.getByText('user2/commits_search_test')).toBeHidden();

  // after performing a search, verify that the name, 'selected repositories', and selected permissions are maintained
  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue(/^Example AI/);
  await expect(page.getByRole('radio', {name: 'Specific repositories'})).toBeChecked();
  await expect(page.getByRole('combobox', {name: 'repository'})).toHaveValue('write:repository');
  await expect(page.getByPlaceholder('Search repos…')).toHaveValue('big_test_private_4'); // search box still populated after reload

  // Add the big_test_private_4 repo.
  await page.getByRole('button', {name: 'Add org17/big_test_private_4'}).click();
  await expect(page.getByText('Selected repository (1)')).toBeVisible();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();

  // Remove it to test remove, and then re-add
  await page.getByRole('button', {name: 'Remove org17/big_test_private_4'}).click();
  await expect(page.getByText('Selected repositories (0)')).toBeVisible();
  await expect(page.getByText('org17/big_test_private_4')).toBeVisible();
  await page.getByRole('button', {name: 'Add org17/big_test_private_4'}).click();

  // Save authorized integration
  await page.getByRole('button', {name: 'Save authorized integration'}).click();

  // Reopen to check change to repo-specific was saved:
  await editFixtureAuthorizedIntegration(page);
  await expect(page.getByRole('radio', {name: 'All (public, private, and limited)'})).not.toBeChecked();
  await expect(page.getByRole('radio', {name: 'Public only'})).not.toBeChecked();
  await expect(page.getByRole('radio', {name: 'Specific repositories'})).toBeChecked();
  await expect(page.getByRole('button', {name: 'Remove org17/big_test_private_4'})).toBeVisible();

  // Restore values to avoid affecting other tests and other platforms:
  await page.getByRole('radio', {name: 'All (public, private, and limited)'}).click();
  await page.getByRole('button', {name: 'Save authorized integration'}).click();
  await expect(page.locator('.flex-item-title').filter({hasText: 'Example AI'})).not.toHaveCount(0); // ensure save completes and we land on list page
});

test('User: Add authorized integration', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await page.getByRole('menu').filter({hasText: 'Add authorized integration'}).click();
  await page.getByRole('menuitem', {name: 'Generic JWT'}).click();

  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('');
  await expect(page.getByRole('textbox', {name: 'Description'})).toHaveValue('');
  await expect(page.getByRole('textbox', {name: 'Audience (aud Claim)'})).toBeHidden();
  await expect(page.getByRole('textbox', {name: 'Issuer (iss Claim)'})).toHaveValue('');

  await page.getByRole('textbox', {name: 'Name'}).fill('New Authorized Integration!');
  await page.getByRole('textbox', {name: 'Description'}).fill('Description that carefully describes things.');
  await page.getByRole('textbox', {name: 'Issuer (iss Claim)'}).fill('urn:forgejo:authorized-integrations:actions');
  await page.getByRole('combobox', {name: 'repository'}).selectOption('read:repository');
  await page.getByRole('button', {name: 'Create authorized integration'}).click();

  // Create will reload the page with a success banner, and the audience now populated:
  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('New Authorized Integration!');
  await expect(page.getByRole('textbox', {name: 'Description'})).toHaveValue('Description that carefully describes things.');
  await expect(page.getByRole('textbox', {name: 'Audience (aud Claim)'})).toHaveValue(/^u:[0-9]+/);
  await expect(page.getByRole('textbox', {name: 'Issuer (iss Claim)'})).toHaveValue('urn:forgejo:authorized-integrations:actions');

  // Flash banner:
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const flashText = await page.locator('.ui.message.flash-success').textContent();
  expect(flashText?.trim()).toBe('Created authorized integration: New Authorized Integration!');

  // Delete the added integration, minimizing left-over test data and also validating the delete UI:
  await page.goto('/user/settings/authorized-integrations');
  await page.locator('.flex-item')
    .filter({has: page.locator('.flex-item-title', {hasText: 'New Authorized Integration!'})})
    .getByRole('button', {name: 'Delete'}).click();
  await page.getByRole('button', {name: 'Yes'}).click();
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const deleteFlashText = await page.locator('.ui.message.flash-success').textContent();
  expect(deleteFlashText?.trim()).toBe('Authorized integration has been deleted successfully.');
});

test('User: Add authorized integration validation error', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await page.getByRole('menu').filter({hasText: 'Add authorized integration'}).click();
  await page.getByRole('menuitem', {name: 'Generic JWT'}).click();

  await page.getByRole('textbox', {name: 'Name'}).fill('\t\t');
  await page.getByRole('textbox', {name: 'Issuer (iss Claim)'}).fill('urn:forgejo:authorized-integrations:actions');
  await page.getByRole('button', {name: 'Create authorized integration'}).click();

  // Should have errors from having just whitespace in the Name field:
  await expect(page.locator('.flash-error')).toContainText('Authorized integration name is required.');
  await expect(page.getByRole('textbox', {name: 'Name'}).locator('..')).toHaveClass('required field error');

  // Fill out missing field and resubmit:
  await page.getByRole('textbox', {name: 'Name'}).fill('Forgot to fill this out!');
  await page.getByRole('button', {name: 'Create authorized integration'}).click();

  // Flash banner:
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const flashText = await page.locator('.ui.message.flash-success').textContent();
  expect(flashText?.trim()).toBe('Created authorized integration: Forgot to fill this out!');

  // Delete the added integration, minimizing left-over test data and also validating the delete UI:
  await page.goto('/user/settings/authorized-integrations');
  await page.locator('.flex-item')
    .filter({has: page.locator('.flex-item-title', {hasText: 'Forgot to fill this out!'})})
    .getByRole('button', {name: 'Delete'}).click();
  await page.getByRole('button', {name: 'Yes'}).click();
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const deleteFlashText = await page.locator('.ui.message.flash-success').textContent();
  expect(deleteFlashText?.trim()).toBe('Authorized integration has been deleted successfully.');
});

test('User: Add authorized integration (actions local)', async ({browser}, workerInfo) => {
  const page = await login({browser}, workerInfo);
  await page.goto('/user/settings/authorized-integrations');

  await page.getByRole('menu').filter({hasText: 'Add authorized integration'}).click();
  await page.getByRole('menuitem', {name: 'Forgejo Actions (Local)'}).click();

  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('');
  await expect(page.getByRole('textbox', {name: 'Description'})).toHaveValue('');

  await page.getByRole('textbox', {name: 'Name'}).fill('New Forgejo Actions (Local) Integration!');
  await page.getByRole('textbox', {name: 'Description'}).fill('Description that carefully describes things.');
  await page.getByRole('combobox', {name: 'repository'}).selectOption('read:repository');
  await page.getByRole('button', {name: 'Create authorized integration'}).click();

  // Didn't select a repository as the source, so an error is expected:
  await expect(page.locator('.flash-error')).toContainText('Forgejo Actions source repository must be selected.');

  // Verify that initial search results for the "Select repository:" section are visible
  const actionsLocalSectionGetter = async () => await page.locator('.ui.attached.segment').filter({has: page.locator('p', {hasText: 'Forgejo Actions will be able to access Forgejo'})});
  let actionsLocalSection = await actionsLocalSectionGetter();
  await expect(actionsLocalSection.getByText('user2/diff-test')).toBeVisible();
  await expect(actionsLocalSection.getByText('user2/huge-diff-test')).toBeVisible(); // another repo, will be used to verify search worked
  await actionsLocalSection.getByPlaceholder('Search repos…').fill('huge-diff');
  await actionsLocalSection.getByRole('button', {name: 'Search…'}).click();

  // verify search results visible:
  actionsLocalSection = await actionsLocalSectionGetter();
  await expect(actionsLocalSection.getByText('user2/huge-diff-test')).toBeVisible();
  await expect(actionsLocalSection.getByText('user2/diff-test')).toBeHidden();

  // after performing a search, verify that other fields haven't lost their form values:
  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('New Forgejo Actions (Local) Integration!');
  await expect(page.getByRole('combobox', {name: 'repository'})).toHaveValue('read:repository');
  await expect(actionsLocalSection.getByPlaceholder('Search repos…')).toHaveValue('huge-diff'); // search box still populated after reload

  // Add the big_test_private_4 repo.
  await actionsLocalSection.getByRole('button', {name: 'Add user2/huge-diff-test'}).click();
  actionsLocalSection = await actionsLocalSectionGetter();
  await expect(actionsLocalSection.getByText('Source repository:')).toBeVisible();
  await expect(actionsLocalSection.getByText('user2/huge-diff-test')).toBeVisible();

  await page.getByRole('button', {name: 'Create authorized integration'}).click();

  // Create will reload the page with a success banner, and the audience now populated; re-verify the Forgejo Actions repo selection as well:
  await expect(page.getByRole('textbox', {name: 'Name'})).toHaveValue('New Forgejo Actions (Local) Integration!');
  await expect(page.getByRole('textbox', {name: 'Audience (aud Claim)'})).toHaveValue(/^u:[0-9]+/);
  actionsLocalSection = await actionsLocalSectionGetter();
  await expect(actionsLocalSection.getByText('Source repository:')).toBeVisible();
  await expect(actionsLocalSection.getByText('user2/huge-diff-test')).toBeVisible();

  // Flash banner:
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const flashText = await page.locator('.ui.message.flash-success').textContent();
  expect(flashText?.trim()).toBe('Created authorized integration: New Forgejo Actions (Local) Integration!');

  // Delete the added integration, minimizing left-over test data and also validating the delete UI:
  await page.goto('/user/settings/authorized-integrations');
  await page.locator('.flex-item')
    .filter({has: page.locator('.flex-item-title', {hasText: 'New Forgejo Actions (Local) Integration!'})})
    .getByRole('button', {name: 'Delete'}).click();
  await page.getByRole('button', {name: 'Yes'}).click();
  await expect(page.locator('.ui.message.flash-success')).toBeVisible();
  const deleteFlashText = await page.locator('.ui.message.flash-success').textContent();
  expect(deleteFlashText?.trim()).toBe('Authorized integration has been deleted successfully.');
});
