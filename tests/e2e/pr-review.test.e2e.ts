// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/repo/diff/new_review.tmpl
// web_src/js/features/repo-diff.js
// web_src/js/features/repo-issue.js
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';
import {screenshot} from './shared/screenshots.ts';

test.use({user: 'user2'});

// The comment context menu is a JS-less <details>; it must keep working on conversations
// inserted via AJAX, which must not be initialized as Fomantic dropdowns.
async function expectCommentMenuToOpen(page) {
  const menu = page.locator('.conversation-holder .comment-header-right.actions details.dropdown');
  await menu.locator('summary').click();
  await expect(menu.locator('.content').getByText(/Copy link.*/)).toBeVisible();
  await menu.locator('summary').click();
  await expect(menu.locator('.content')).toBeHidden();
}

test('PR: Create review from files', async ({page}) => {
  const response = await page.goto('/user2/repo1/pulls/5/files');
  expect(response?.status()).toBe(200);

  await expect(page.locator('.tippy-box .review-box-panel')).toBeHidden();
  await screenshot(page);

  // Review panel should appear after clicking Finish review
  await page.locator('#review-box .js-btn-review').click();
  await expect(page.locator('.tippy-box .review-box-panel')).toBeVisible();
  await screenshot(page);

  await page.locator('.review-box-panel textarea#_combo_markdown_editor_0')
    .fill('This is a review');
  await page.locator('.review-box-panel button.btn-submit[value="approve"]').click();
  await page.waitForURL(/.*\/user2\/repo1\/pulls\/5#issuecomment-\d+/);
  await screenshot(page);
});

test('PR: Create review from commit', async ({page}) => {
  const response = await page.goto('/user2/repo1/pulls/3/commits/4a357436d925b5c974181ff12a994538ddc5a269');
  expect(response?.status()).toBe(200);

  await page.locator('button.add-code-comment').click();
  const code_comment = page.locator('.comment-code-cloud form textarea.markdown-text-editor');
  await expect(code_comment).toBeVisible();

  await code_comment.fill('This is a code comment');
  await screenshot(page);

  const start_button = page.locator('.comment-code-cloud form button.btn-start-review');
  // Workaround for #7152, where there might already be a pending review state from previous
  // test runs (most likely to happen when debugging tests).
  if (await start_button.isVisible({timeout: 100})) {
    await start_button.click();
  } else {
    await page.locator('.comment-code-cloud form button[name="pending_review"]').click();
  }

  await expect(page.locator('.comment-list .comment-container')).toBeVisible();
  await expectCommentMenuToOpen(page);

  // We need to wait for the review to be processed. Checking the comment counter
  // conveniently does that.
  await expect(page.locator('#review-box .js-btn-review > span.review-comments-counter')).toHaveText('1');

  await page.locator('#review-box .js-btn-review').click();
  await expect(page.locator('.tippy-box .review-box-panel')).toBeVisible();
  await screenshot(page);

  await page.locator('.review-box-panel textarea.markdown-text-editor')
    .fill('This is a review');
  await page.locator('.review-box-panel button.btn-submit[value="approve"]').click();
  await page.waitForURL(/.*\/user2\/repo1\/pulls\/3#issuecomment-\d+/);
  await screenshot(page);

  // #region Use all the resolve/show/hide features
  // The comment content is visible and offers to "Resolve conversation"
  await expect(page.locator('.comment-content')).toBeVisible();
  await page.getByText('Resolve conversation').click();

  // Resolving conversation hides the comment content and gives a "Show resolved" button
  await expect(page.locator('.comment-content')).toBeHidden();
  await page.getByText('Show resolved').click();

  // Clicking the "Shows resolved" button makes the comment content show up and
  // replaces the button with one saying "Hide resolved"
  await expect(page.locator('.comment-content')).toBeVisible();
  await expect(page.getByText('Show resolved')).toBeHidden();
  await page.getByText('Hide resolved').click();

  // Clicking the "Hide resolved" button reverses the previous action
  await expect(page.locator('.comment-content')).toBeHidden();
  await expect(page.getByText('Hide resolved')).toBeHidden();

  // Show the comment again to make the "Unresolve conversation" button appear
  await page.getByText('Show resolved').click();
  await page.getByText('Unresolve conversation').click();

  // We're back to where we started
  await expect(page.locator('.comment-content')).toBeVisible();
  await expect(page.getByText('Resolve conversation')).toBeVisible();
  await expectCommentMenuToOpen(page);
  // #endregion

  // In addition to testing the ability to delete comments, this also
  // performs clean up. If tests are run for multiple platforms, the data isn't reset
  // in-between, and subsequent runs of this test would fail, because when there already is
  // a comment, the on-hover button to start a conversation doesn't appear anymore.
  await page.goto('/user2/repo1/pulls/3/commits/4a357436d925b5c974181ff12a994538ddc5a269');
  await page.locator('.comment-header-right.actions details.dropdown summary').click();

  await expect(page.locator('.comment-header-right.actions details.dropdown .content').getByText(/Copy link.*/)).toBeVisible();
  // The button to delete a comment will prompt for confirmation using a browser alert.
  page.on('dialog', (dialog) => dialog.accept());
  await page.locator('.comment-header-right.actions details.dropdown .content .delete-comment').click();

  await expect(page.locator('.comment-list .comment-container')).toBeHidden();
  await screenshot(page);
});

test('PR: Navigate by single commit', async ({page}) => {
  const response = await page.goto('/user2/repo1/pulls/3/commits');
  expect(response?.status()).toBe(200);

  await page.locator('.commit .message-wrapper a').nth(1).click();
  await page.waitForURL(/.*\/user2\/repo1\/pulls\/3\/commits\/4a357436d925b5c974181ff12a994538ddc5a269/);
  await screenshot(page);

  let prevButton = page.locator('.commit-header-buttons').getByText(/Prev/);
  let nextButton = page.locator('.commit-header-buttons').getByText(/Next/);
  await prevButton.waitFor();
  await nextButton.waitFor();

  await expect(prevButton).toHaveClass(/disabled/);
  await expect(nextButton).not.toHaveClass(/disabled/);
  await expect(nextButton).toHaveAttribute('href', '/user2/repo1/pulls/3/commits/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd');
  await nextButton.click();

  await page.waitForURL(/.*\/user2\/repo1\/pulls\/3\/commits\/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd/);
  await screenshot(page);

  prevButton = page.locator('.commit-header-buttons').getByText(/Prev/);
  nextButton = page.locator('.commit-header-buttons').getByText(/Next/);
  await prevButton.waitFor();
  await nextButton.waitFor();

  await expect(prevButton).not.toHaveClass(/disabled/);
  await expect(nextButton).toHaveClass(/disabled/);
  await expect(prevButton).toHaveAttribute('href', '/user2/repo1/pulls/3/commits/4a357436d925b5c974181ff12a994538ddc5a269');
});

test('PR: Test mentions values', async ({page}) => {
  const response = await page.goto('/user2/repo1/pulls/5/files');
  expect(response?.status()).toBe(200);

  await page.locator('#review-box .js-btn-review').click();
  await expect(page.locator('.tippy-box .review-box-panel')).toBeVisible();

  await page.locator('.review-box-panel textarea#_combo_markdown_editor_0')
    .fill('@');
  await screenshot(page);

  await expect(page.locator('ul.suggestions li span:first-of-type')).toContainText([
    'user1',
    'user2',
  ]);

  await page.locator("ul.suggestions li[data-value='@user1']").click();
  await expect(page.locator('.review-box-panel textarea#_combo_markdown_editor_0')).toHaveValue('@user1 ');
});

test('multi-commit commenting', async ({page, request}) => {
  const response = await page.goto('/user2/long-diff-test');
  expect(response?.status()).toBe(200);

  try {
    await page.getByText('2 branches').click(); // navigate to branch list
    await page.getByText('New pull request').click(); // load compare view for the branch
    await page.locator('.show-form-container').getByText('New pull request').click(); // actually open the PR form
    await page.locator('.primary.button').getByText('Create pull request').click(); // submit PR creation

    // Test situation: adding a comment on a line that was created in the *second* commit, doing it from the "Files changed" view.
    await page.getByText('Files changed').click();
    await page.getByText('More  This line was changed in commit 2')
      .locator('..')
      .locator('button.add-code-comment')
      .click();
    await page.getByPlaceholder('Leave a comment').fill('Comment on line changed in commit 2');
    await page.getByText('Add single comment').click();

    // Test assertion: when viewing the comment from the 'Conversation' page, it's diff should look correct:
    await page.getByText('Conversation').click();
    await expect(page.locator('.pull.menu .item.active')).toContainText('Conversation'); // ensure we navigated back to Conversation page
    await expect(page.locator('.text.comment-content .render-content.markup')).toHaveText('Comment on line changed in commit 2');
    await expect(page.locator('.diff-file-box .code-diff')).toContainText('More  This line was changed in commit 2');

    // Test assertion: when viewing the comment from the second commit, it should be placed correctly in the UI:
    await page.getByRole('link', {name: 'Commits'}).click();
    await page.getByText('add commit to branch').nth(1).click();
    // FIXME: The intent of this test is to make sure that the comment box appears in the "right spot", which is *below*
    // the line of code that it was commented on.  This check uses the elements bounding boxes which... is pretty ugly
    // and could be done better.  Probably would be better to find the line of code that the box is rendered on,
    // instead.
    const codeLine = await page.getByText('More  This line was changed in commit 2').boundingBox();
    const commentBox = await page.locator('.render-content.markup').getByText('Comment on line changed in commit 2').boundingBox();
    expect(commentBox.y).toBeGreaterThan(codeLine.y);
  } finally {
    // Delete any PRs on the test repo so that this test can be rerun.
    const issuesResp = await request.get(`/api/v1/repos/user2/long-diff-test/issues`);
    expect(issuesResp.ok()).toBeTruthy();
    const issues = await issuesResp.json();
    for (const issue of issues) {
      const delResp = await request.delete(`/api/v1/repos/user2/long-diff-test/issues/${issue.number}`, {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Basic ${btoa(`user1:password`)}`,
        },
      });
      expect(delResp.ok()).toBeTruthy();
    }
  }
});
