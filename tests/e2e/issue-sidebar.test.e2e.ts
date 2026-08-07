// @watch start
// templates/repo/issue/view_content/**
// web_src/css/repo/issue-**
// web_src/js/features/repo-issue**
// @watch end

/* eslint playwright/expect-expect: ["error", { "assertFunctionNames": ["check_wip", "accessibilityCheck"] }] */

import {expect, type Page} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';
import {accessibilityCheck} from './shared/accessibility.ts';

test.use({user: 'user2'});

test.describe('Pull: Toggle WIP', () => {
  const prTitle = 'pull5';

  async function toggle_wip_to({page}: {page: Page}, should: boolean) {
    const loadPromise = page.waitForEvent('load');
    if (should) {
      await page.getByText('Still in progress?').click();
    } else {
      await page.getByText('Ready for review?').click();
    }
    await loadPromise;
  }

  async function check_wip({page}: {page: Page}, is: boolean) {
    const elemTitle = 'h1';
    const stateLabel = '.issue-state-label';
    await expect(page.locator(elemTitle)).toContainText(prTitle);
    await expect(page.locator(elemTitle)).toContainText('#5');
    const wipRegex = /(wip|\[WIP\])/i;
    if (is) {
      await expect(page.locator(elemTitle)).toContainText(wipRegex);
      await expect(page.locator(stateLabel)).toContainText('Draft');
    } else {
      await expect(page.locator(elemTitle)).not.toContainText(wipRegex);
      await expect(page.locator(stateLabel)).toContainText('Open');
    }
  }

  async function setTitle({page}: {page: Page}, title: string) {
    await page.locator('#issue-title-edit-show').click();
    await page.locator('#issue-title-editor input').fill(title);
    const loadPromise = page.waitForEvent('load');
    await page.locator('#issue-title-editor').getByText('Save').click();
    await loadPromise;
  }

  test.beforeEach(async ({page}) => {
    const response = await page.goto('/user2/repo1/pulls/5');
    expect(response?.status()).toBe(200); // Status OK
    // ensure original title
    await setTitle({page}, prTitle);
    await check_wip({page}, false);
  });

  test('simple toggle', async ({page}) => {
    // toggle to WIP
    await toggle_wip_to({page}, true);
    await check_wip({page}, true);
    // remove WIP
    await toggle_wip_to({page}, false);
    await check_wip({page}, false);
  });

  test('manual edit', async ({page}) => {
    // manually edit title to another prefix
    await setTitle({page}, `[WIP] ${prTitle}`);
    await check_wip({page}, true);
    // remove again
    await toggle_wip_to({page}, false);
    await check_wip({page}, false);
  });

  test('maximum title length', async ({page}) => {
    // check maximum title length is handled gracefully
    const maxLenStr = prTitle + 'a'.repeat(240);
    await setTitle({page}, maxLenStr);
    await expect(page.locator('h1')).toContainText(maxLenStr);
    await check_wip({page}, false);
    await toggle_wip_to({page}, true);
    await check_wip({page}, true);
    await expect(page.locator('h1')).toContainText(maxLenStr);
    await toggle_wip_to({page}, false);
    await check_wip({page}, false);
    await expect(page.locator('h1')).toContainText(maxLenStr);
  });

  test('wip prefix casing', async ({page}) => {
    await setTitle({page}, `wIP:${prTitle}`);
    await expect(page.locator('h1')).toContainText(`wIP:${prTitle}`);
    await check_wip({page}, true);
    await toggle_wip_to({page}, false);
    await check_wip({page}, false);
    await setTitle({page}, `[Wip]:${prTitle}`);
    await expect(page.locator('h1')).toContainText(`[Wip]:${prTitle}`);
    await check_wip({page}, true);
    await toggle_wip_to({page}, false);
    await check_wip({page}, false);
    await expect(page.locator('h1')).toContainText(prTitle);
  });
});

test('Issue: Labels', async ({page}) => {
  async function submitLabels({page}: {page: Page}) {
    const submitted = page.waitForResponse('/user2/repo1/issues/labels');
    await page.locator('textarea').first().click(); // close via unrelated element
    await submitted;
    await page.waitForLoadState();
  }

  // select label list in sidebar only
  const labelList = page.locator('.issue-content-right .labels-list a');
  const response = await page.goto('/user2/repo1/issues/1');
  expect(response?.status()).toBe(200);

  // restore initial state
  await page.locator('.select-label').click();
  const responsePromise = page.waitForResponse('/user2/repo1/issues/labels');
  await page.getByText('Clear labels').click();
  await responsePromise;
  await expect(labelList.filter({hasText: 'label1'})).toBeHidden();
  await expect(labelList.filter({hasText: 'label2'})).toBeHidden();

  // add both labels
  await page.locator('.select-label').click();
  // label search could be tested this way:
  // await page.locator('.select-label input').fill('label2');
  await page.locator('.select-label .item').filter({hasText: 'label2'}).click();
  await page.locator('.select-label .item').filter({hasText: 'label1'}).click();
  await submitLabels({page});
  await expect(labelList.filter({hasText: 'label2'})).toBeVisible();
  await expect(labelList.filter({hasText: 'label1'})).toBeVisible();

  // test removing label2 again
  // due to a race condition, the page could still be "reloading",
  // closing the dropdown after it was clicked.
  // Retry the interaction as a group
  // also see https://playwright.dev/docs/test-assertions#expecttopass
  await expect(async () => {
    await page.locator('.select-label').click();
    await page.locator('.select-label .item').filter({hasText: 'label2'}).click();
  }).toPass();
  await submitLabels({page});
  await expect(labelList.filter({hasText: 'label2'})).toBeHidden();
  await expect(labelList.filter({hasText: 'label1'})).toBeVisible();
});

test('Issue: Assignees', async ({page}) => {
  // select label list in sidebar only
  const assigneesList = page.locator('.issue-content-right .assignees.list .selected .item a');

  const response = await page.goto('/org3/repo3/issues/1');
  expect(response?.status()).toBe(200);
  // Clear all assignees
  await page.locator('.select-assignees-modify.dropdown').click();
  await page.locator('.select-assignees-modify.dropdown .no-select.item').click();
  await expect(assigneesList.filter({hasText: 'user2'})).toBeHidden();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeHidden();
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeVisible();
  await expect(page.locator('.select-assign-me')).toBeVisible();

  // Assign other user (with searchbox)
  await page.locator('.select-assignees-modify.dropdown').click();
  await page.type('.select-assignees-modify .menu .search input', 'user4');
  await expect(page.locator('.select-assignees-modify .menu .item').filter({hasText: 'user2'})).toBeHidden();
  await expect(page.locator('.select-assignees-modify .menu .item').filter({hasText: 'user4'})).toBeVisible();
  await page.locator('.select-assignees-modify .menu .item').filter({hasText: 'user4'}).click();
  await page.locator('.select-assignees-modify.dropdown').click();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeVisible();

  // remove user4
  await page.locator('.select-assignees-modify.dropdown').click();
  await page.locator('.select-assignees-modify .menu .item').filter({hasText: 'user4'}).click();
  await page.locator('.select-assignees-modify.dropdown').click();
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeVisible();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeHidden();

  // Test assign me
  await page.locator('.ui.assignees .select-assign-me').click();
  await expect(assigneesList.filter({hasText: 'user2'})).toBeVisible();
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeHidden();
});

test('New Issue: Assignees', async ({page}) => {
  // select label list in sidebar only
  const assigneesList = page.locator('.issue-content-right .assignees.list .selected .item');

  const response = await page.goto('/org3/repo3/issues/new');
  expect(response?.status()).toBe(200);
  // preconditions
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeVisible();
  await expect(assigneesList.filter({hasText: 'user2'})).toBeHidden();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeHidden();

  // Assign other user (with searchbox)
  await page.locator('.select-assignees.dropdown').click();
  await page.fill('.select-assignees .menu .search input', 'user4');
  await expect(page.locator('.select-assignees .menu .item').filter({hasText: 'user2'})).toBeHidden();
  await expect(page.locator('.select-assignees .menu .item').filter({hasText: 'user4'})).toBeVisible();
  await page.locator('.select-assignees .menu .item').filter({hasText: 'user4'}).click();
  await page.locator('.select-assignees.dropdown').click();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeVisible();
  await screenshot(page, page.locator('.issue-content-right'));

  // remove user4
  await page.locator('.select-assignees.dropdown').click();
  await page.locator('.select-assignees .menu .item').filter({hasText: 'user4'}).click();
  await page.locator('.select-assignees.dropdown').click();
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeVisible();
  await expect(assigneesList.filter({hasText: 'user4'})).toBeHidden();

  // Test assign me
  await page.locator('.ui.assignees .select-assign-me').click();
  await expect(assigneesList.filter({hasText: 'user2'})).toBeVisible();
  await expect(page.locator('.ui.assignees.list .item.no-select')).toBeHidden();

  await page.locator('.select-assignees.dropdown').click();
  await page.fill('.select-assignees .menu .search input', '');
  await page.locator('.select-assignees.dropdown .no-select.item').click();
  await expect(page.locator('.select-assign-me')).toBeVisible();
  await screenshot(page, page.locator('div.filter.menu[data-id="#assignee_ids"]'), 30);
});

test('Issue: Milestone', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues/1');
  expect(response?.status()).toBe(200);

  const selectedMilestone = page.locator('.issue-content-right #selected-milestone');
  const milestoneDropdown = page.locator('.issue-content-right .select-milestone.dropdown');
  await expect(selectedMilestone).toContainText('No milestone');

  // Add milestone.
  await milestoneDropdown.click();
  await page.getByRole('option', {name: 'milestone1'}).click();
  await expect(selectedMilestone).toContainText('milestone1');
  await expect(page.locator('.timeline-item.event').last()).toContainText('user2 added this to the milestone1 milestone');

  // Clear milestone.
  await milestoneDropdown.click();
  await page.getByText('Clear milestone', {exact: true}).click();
  await expect(selectedMilestone).toContainText('No milestone');
  await expect(page.locator('.timeline-item.event').last()).toContainText('user2 removed this from the milestone1 milestone');
});

test('New Issue: Milestone', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues/new');
  expect(response?.status()).toBe(200);

  const selectedMilestone = page.locator('.issue-content-right .select-milestone.list');
  const milestoneDropdown = page.locator('.issue-content-right .select-milestone.dropdown');
  await expect(selectedMilestone).toContainText('No milestone');
  await screenshot(page, page.locator('.issue-content-right'));

  // Add milestone.
  await milestoneDropdown.click();
  await screenshot(page, page.locator('.menu.transition.visible'), 30);
  await page.getByRole('option', {name: 'milestone1'}).click();
  await expect(selectedMilestone).toContainText('milestone1');
  await screenshot(page, page.locator('.issue-content-right'));

  // Clear milestone.
  await milestoneDropdown.click();
  await page.getByText('Clear milestone', {exact: true}).click();
  await expect(selectedMilestone).toContainText('No milestone');
  await screenshot(page, page.locator('.issue-content-right'));
});

test.describe('Dependency dropdown', () => {
  test.use({user: 'user11'});

  test('Issue: Dependencies', async ({page}) => {
    const response = await page.goto('/user11/dependency-test/issues/3');
    expect(response?.status()).toBe(200);

    const depsBlock = page.locator('.issue-content-right .depending');
    const deleteDepBtn = page.locator('.issue-content-right .depending .delete-dependency-button');

    const input = page.locator('#new-dependency-drop-list .search');
    const current = page.locator('#new-dependency-drop-list .text').first();
    const menu = page.locator('#new-dependency-drop-list .menu');
    const items = page.locator('#new-dependency-drop-list .menu .item');

    const confirmDelete = async () => {
      const modal = page.locator('.modal.remove-dependency');
      await expect(modal).toBeVisible();
      await expect(modal).toContainText('This will remove the dependency from this issue');
      await modal.locator('button.ok').click();
    };

    // A kludge to set the dropdown to the *wrong* value so it lets us select the correct one next.
    const resetDropdown = async () => {
      if (await current.textContent().then((s) => s.includes('#4'))) return;
      await input.click();
      await input.fill('unrelated');
      await expect(items.first()).toContainText('unrelated');
      await items.first().click();
      await expect(current).toContainText('#4');
      await input.click();
    };

    await expect(depsBlock).toBeVisible();
    while (await deleteDepBtn.first().isVisible()) {
      await deleteDepBtn.first().click(); // wipe added dependencies from any previously failed tests
      await confirmDelete();
    }
    await expect(depsBlock).toContainText('No dependencies set');

    await input.scrollIntoViewIfNeeded();
    await input.click();

    const first = 'first issue here';
    const second = 'second issue here';
    const newest = 'newest issue';

    // Without query, it should show issues in the same repo, sorted by date, except current one.
    await expect(menu).toBeVisible();
    await expect(items).toHaveCount(4); // 5 issues in this repo, minus current one
    await expect(items.first()).toContainText(newest);
    await expect(items.last()).toContainText(first);
    await resetDropdown();

    // With query, it should search all repos, but show current repo issues first.
    await input.fill('right');
    await expect(items.first()).toContainText(second);
    await expect.poll(() => items.count()).toBeGreaterThan(1); // there is an issue in user11/dependency-test-2 containing the word "right"
    await resetDropdown();

    // When entering an issue number, it should always show that one first, then all text matches.
    await input.fill('1');
    await expect(items.first()).toContainText(first);
    await expect(items.nth(1)).toBeVisible();
    await resetDropdown();

    // Should behave the same with a prefix
    await input.fill('#1');
    await expect(items.first()).toContainText(first);

    // Selecting an issue
    await items.first().click();
    await expect(current).toContainText(first);

    // Add dependency
    const link = page.locator('.issue-content-right .depending .dependency a.title');
    await page.locator('.issue-content-right .depending button').click();
    await expect(link).toHaveAttribute('href', '/user11/dependency-test/issues/1');

    // Remove dependency
    await expect(deleteDepBtn).toBeVisible();
    await deleteDepBtn.click();

    await confirmDelete();

    await expect(depsBlock).toContainText('No dependencies set');
  });
});

test('Issue: Reference', async ({page}) => {
  let response = await page.goto('/user2/repo1/pulls/5');
  expect(response?.status()).toBe(200);

  await expect(page.locator('.ui.reference .truncate')).toContainText(
    'user2/repo1!5',
  );

  response = await page.goto('/user2/repo1/issues/1');
  expect(response?.status()).toBe(200);

  await expect(page.locator('.ui.reference .truncate')).toContainText(
    'user2/repo1#1',
  );

  await page.getByRole('button', {name: 'Copy'}).click();

  await expect(async () => {
    const reference = await page.evaluate(() => navigator.clipboard.readText());
    expect(reference).toBe('user2/repo1#1');
  }).toPass();
});

test('Issue: Watch URL Retention', async ({page}) => {
  const response = await page.goto('/user2/repo1/pulls/5');
  expect(response?.status()).toBe(200);

  const button = page.locator('.ui.watching button');

  await button.click();
  expect(page.url()).not.toContain('/watch');

  await button.click();
  expect(page.url()).not.toContain('/watch');
});

test('accessibility - button-name', async ({page}) => {
  const response = await page.goto('/user2/repo1/issues/1');
  expect(response?.status()).toBe(200);
  await page.locator('.select-assignees-modify.dropdown').click();
  await page.locator('.select-assignees-modify.dropdown .no-select.item').click();
  await accessibilityCheck({page}, ['.issue-content-right'], [], [], ['button-name']);
});
