// @watch start
// templates/repo/actions/**
// web_src/css/actions.css
// web_src/js/components/ActionRunStatus.vue
// web_src/js/components/RepoActionView.vue
// modules/actions/**
// modules/structs/workflow.go
// routers/api/v1/repo/action.go
// routers/web/repo/actions/**
// @watch end

import {expect, type Page} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

const workflow_trigger_notification_text = 'This workflow has a workflow_dispatch event trigger.';

async function dispatchSuccess(page: Page) {
  await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

  await page.locator('#workflow_dispatch_dropdown>button').click();

  await page.fill('input[name="inputs[string2]"]', 'abc');
  await screenshot(page, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
  await page.locator('#workflow-dispatch-submit').click();

  await expect(page.getByText('Workflow run was successfully requested.')).toBeVisible();

  await expect(page.locator('.run-list>:first-child .run-list-meta', {hasText: 'now'})).toBeVisible();
  await screenshot(page, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
}

test.describe('Workflow Authenticated user2', () => {
  test.use({user: 'user2'});

  test('workflow dispatch present', async ({page}) => {
    await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    await expect(page.getByText(workflow_trigger_notification_text)).toBeVisible();

    const run_workflow_btn = page.locator('#workflow_dispatch_dropdown>button');
    await expect(run_workflow_btn).toBeVisible();

    const menu = page.locator('#workflow_dispatch_dropdown>.menu');
    await expect(menu).toBeHidden();
    await run_workflow_btn.click();
    await expect(menu).toBeVisible();
    await screenshot(page, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
  });

  test('dispatch error: missing inputs', async ({page}) => {
    await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    await page.locator('#workflow_dispatch_dropdown>button').click();

    // Remove the required attribute so we can trigger the error message!
    await page.evaluate(() => {
      const elem = document.querySelector('input[name="inputs[string2]"]');
      elem?.removeAttribute('required');
    });

    await page.locator('#workflow-dispatch-submit').click();

    await expect(page.getByText('Require value for input "String w/o. default".')).toBeVisible();
    await screenshot(page, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
  });

  // no assertions as the login in this test case is extracted for reuse
  // eslint-disable-next-line playwright/expect-expect
  test('dispatch success', async ({page}) => {
    await dispatchSuccess(page);
  });

  test('Disable/enable workflow', async ({page}) => {
    await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml');

    const menuOpener = page.locator('.filter.menu details.dropdown > summary');
    const disableButton = page.locator('a[data-url^="/user2/test_workflows/actions/disable"]');
    const enableButton = page.locator('a[data-url^="/user2/test_workflows/actions/enable"]');
    const disabledLabel = page.locator('.vertical.menu .item.active .ui.label').getByText('Disabled');
    const flashBanner = page.locator('#flash-message');

    // Overflow menu is hidden
    await expect(disableButton).toBeHidden();
    await expect(enableButton).toBeHidden();

    await expect(menuOpener).toHaveAttribute('aria-label', 'More actions');
    await menuOpener.click();

    // The current "Enabled" state is what previous tests left, but this test is built to not care
    if (await disableButton.isVisible()) {
      // Assert elemeents on page
      await expect(enableButton).toBeHidden();
      await expect(disabledLabel).toBeHidden();

      // Flip the state
      await disableButton.click();
      await flashBanner.waitFor();
      await menuOpener.click();

      // Assert elemeents on page
      await expect(enableButton).toBeVisible();
      await expect(disableButton).toBeHidden();
      await expect(disabledLabel).toBeVisible();
    } else {
      // Assert elemeents on page
      await expect(enableButton).toBeVisible();
      await expect(disabledLabel).toBeVisible();

      // Flip the state
      await enableButton.click();
      await flashBanner.waitFor();
      await menuOpener.click();

      // Assert elemeents on page
      await expect(enableButton).toBeHidden();
      await expect(disableButton).toBeVisible();
      await expect(disabledLabel).toBeHidden();
    }
  });
});

test('workflow dispatch box not available for unauthenticated users', async ({page}) => {
  await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

  await expect(page.locator('body')).not.toContainText(workflow_trigger_notification_text);
  await screenshot(page, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
});

test('job run links to its defining file and all other runs from the same file', async ({page}) => {
  await page.goto('/user2/test_workflows/actions/runs/1');
  await expect(page.locator('.action-summary a').getByText('test-dispatch.yml', {exact: true}))
    .toHaveAttribute('href', '/user2/test_workflows/src/commit/774f93df12d14931ea93259ae93418da4482fcc1/.forgejo/workflows/test-dispatch.yml');
  await expect(page.locator('.action-summary a').getByText('all runs', {exact: true}))
    .toHaveAttribute('href', '/user2/test_workflows/actions?workflow=test-dispatch.yml');
});

async function completeDynamicRefresh(page: Page) {
  // Ensure that the reloading indicator isn't active, indicating that dynamic refresh is done.
  await expect(page.locator('#reloading-indicator')).not.toHaveClass(/(^|\s)is-loading(\s|$)/);
}

async function simulatePollingInterval(page: Page) {
  // In order to simulate the background page sitting around for > 30s, a custom event `simulate-polling-interval` is
  // fired into the document to mimic the polling interval expiring -- although this isn't a perfectly great E2E test
  // with this kind of mimicry, it's better than having multiple >30s execution-time tests.
  await page.evaluate(() => {
    document.dispatchEvent(new Event('simulate-polling-interval'));
  });
  await completeDynamicRefresh(page);
}

test.describe('workflow list dynamic refresh', () => {
  test.use({user: 'user2'});

  test('refreshes on visibility change', async ({page}) => {
    // Test operates by creating two pages; one which is sitting idle on the workflows list (backgroundPage), and one
    // which triggers a workflow dispatch.  Then a document visibilitychange event is fired on the background page to
    // mimic a user returning to the tab on their browser, which should trigger the workflow list to refresh and display
    // the newly dispatched workflow from the other page.

    const backgroundPage = await page.context().newPage();
    await backgroundPage.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    // Mirror the `Workflow Authenticated user2 > dispatch success` test:
    await dispatchSuccess(page);
    const latestDispatchedRun = await page.locator('.run-list>:first-child .flex-item-body>b').textContent();
    expect(latestDispatchedRun).toMatch(/^#/); // workflow ID, eg. "#53"

    // Synthetically trigger a visibilitychange event, as if we were returning to backgroundPage:
    await backgroundPage.evaluate(() => {
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await completeDynamicRefresh(page);
    await expect(backgroundPage.locator('.run-list>:first-child .flex-item-body>b', {hasText: latestDispatchedRun})).toBeVisible();
    await screenshot(backgroundPage, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
  });

  test('refreshes on interval', async ({page}) => {
    // Test operates by creating two pages; one which is sitting idle on the workflows list (backgroundPage), and one
    // which triggers a workflow dispatch.  After the polling, the page should refresh and show the newly dispatched
    // workflow from the other page.

    const backgroundPage = await page.context().newPage();
    await backgroundPage.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    // Mirror the `Workflow Authenticated user2 > dispatch success` test:
    await dispatchSuccess(page);
    const latestDispatchedRun = await page.locator('.run-list>:first-child .flex-item-body>b').textContent();
    expect(latestDispatchedRun).toMatch(/^#/); // workflow ID, eg. "#53"

    await simulatePollingInterval(backgroundPage);
    await expect(backgroundPage.locator('.run-list>:first-child .flex-item-body>b', {hasText: latestDispatchedRun})).toBeVisible();
    await screenshot(backgroundPage, page.locator('div.ui.container').filter({hasText: 'All workflows'}));
  });

  test('post-refresh the dropdowns continue to operate', async ({page}) => {
    // Verify that after the page is dynamically refreshed, the 'Actor', 'Status', and 'Run workflow' dropdowns work
    // correctly -- that the htmx morph hasn't messed up any JS event handlers.
    await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    // Mirror the `Workflow Authenticated user2 > dispatch success` test -- this creates data for the 'Actor' dropdown
    await dispatchSuccess(page);

    // Perform a dynamic refresh before checking the functionality of each dropdown.
    await simulatePollingInterval(page);

    // Workflow run dialog
    await expect(page.locator('input[name="inputs[string2]"]')).toBeHidden();
    await page.locator('#workflow_dispatch_dropdown>button').click();
    await expect(page.locator('input[name="inputs[string2]"]')).toBeVisible();
    await page.locator('#workflow_dispatch_dropdown>button').click();

    // Status dropdown
    const dropdown = page.locator('#status_dropdown');
    const dropdown_menu = dropdown.locator('.menu');
    await expect(dropdown_menu).toBeHidden();
    await dropdown.click();
    await expect(dropdown_menu).toBeVisible();
    await expect(dropdown_menu.getByText('All status')).toHaveAttribute('href', /&status=0$/);
    await expect(dropdown_menu.getByText('Success')).toHaveAttribute('href', /&status=1$/);
    await expect(dropdown_menu.getByText('Failure')).toHaveAttribute('href', /&status=2$/);
    await expect(dropdown_menu.getByText('Waiting')).toHaveAttribute('href', /&status=5$/);
    await expect(dropdown_menu.getByText('Running')).toHaveAttribute('href', /&status=6$/);
    await expect(dropdown_menu.getByText('Blocked')).toHaveAttribute('href', /&status=7$/);
    await expect(dropdown_menu.getByText('Canceled')).toHaveAttribute('href', /&status=3$/);
    await expect(dropdown_menu.getByText('Skipped')).toHaveAttribute('href', /&status=4$/);

    // Actor dropdown
    await expect(page.getByText('All actors')).toBeHidden();
    await page.locator('#actor_dropdown').click();
    await expect(page.getByText('All Actors')).toBeVisible();
  });

  test('refresh does not break interacting with open drop-downs', async ({page}) => {
    // Verify that if the polling refresh occurs while interacting with any multi-step dropdown on the page, the
    // multi-step interaction continues to be visible and functional.  This is implemented by preventing the refresh,
    // but that isn't the subject of the test here -- as long as the dropdown isn't broken by the refresh, that's fine.
    await page.goto('/user2/test_workflows/actions?workflow=test-dispatch.yml&actor=0&status=0');

    // Mirror the `Workflow Authenticated user2 > dispatch success` test -- this creates data for the 'Actor' dropdown
    await dispatchSuccess(page);

    // Workflow run dialog
    await expect(page.locator('input[name="inputs[string2]"]')).toBeHidden();
    await page.locator('#workflow_dispatch_dropdown>button').click();
    await expect(page.locator('input[name="inputs[string2]"]')).toBeVisible();
    await simulatePollingInterval(page);
    await expect(page.locator('input[name="inputs[string2]"]')).toBeVisible();

    // Status dropdown
    await expect(page.getByText('Waiting')).toBeHidden();
    await expect(page.getByText('Failure')).toBeHidden();
    await page.locator('#status_dropdown').click();
    await expect(page.getByText('Waiting')).toBeVisible();
    await expect(page.getByText('Failure')).toBeVisible();
    await expect(page.locator('[aria-expanded="true"]')).toHaveCount(1);
    await simulatePollingInterval(page);
    await expect(page.getByText('Waiting')).toBeVisible();
    await expect(page.getByText('Failure')).toBeVisible();
    await expect(page.locator('[aria-expanded="true"]')).toHaveCount(1);

    // Actor dropdown
    await expect(page.getByText('All actors')).toBeHidden();
    await page.locator('#actor_dropdown').click();
    await expect(page.getByText('All Actors')).toBeVisible();
    await expect(page.locator('[aria-expanded="true"]')).toHaveCount(1);
    await simulatePollingInterval(page);
    await expect(page.getByText('All Actors')).toBeVisible();
    await expect(page.locator('[aria-expanded="true"]')).toHaveCount(1);
  });
});

test('check that options dropdown not overflows', async ({page}) => {
  await page.setViewportSize({
    width: 500,
    height: 1440,
  });
  await page.goto('/user2/test_workflows/actions');

  await page.locator('.run-list-item-right + details.dropdown').first().click();

  await expect(page.locator('.run-list-item-right + details.dropdown > .content').first()).toBeInViewport({
    ratio: 1,
  });
});
