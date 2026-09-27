// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// templates/repo/diff/suggestion_diffs.tmpl
// templates/repo/diff/comments.tmpl
// templates/repo/diff/suggestion_apply_modal.tmpl
// web_src/js/features/repo-suggestion.js
// web_src/js/features/repo-legacy.js
// routers/web/repo/pull_review.go
// @watch end

import {expect, type Page} from '@playwright/test';
import {test, create_temp_user, dynamic_id} from './utils_e2e.ts';

// Comments on the diff line showing `lineText` with a single ```suggestion block.
async function addSuggestion(page: Page, lineText: string, body: string) {
  const line = page.getByRole('row').filter({has: page.getByText(lineText, {exact: true})});
  await line.getByRole('button', {name: 'Add line comment'}).click();
  // Each existing conversation carries a hidden reply form with the same placeholder; target the open one.
  await page.getByPlaceholder('Leave a comment').filter({visible: true}).fill(body);
  await page.getByRole('button', {name: 'Add single comment'}).click();
}

test('PR: batch apply suggestions (create, edit, batch, discard, apply)', async ({browser, request}, workerInfo) => {
  const {context, username} = await create_temp_user(browser, workerInfo, request);
  const page = await context.newPage();
  const auth = {Authorization: `Basic ${btoa(`${username}:password`)}`};
  const jsonAuth = {...auth, 'Content-Type': 'application/json'};

  const repo = dynamic_id();
  const api = `/api/v1/repos/${username}/${repo}`;
  const created = await request.post('/api/v1/user/repos', {headers: jsonAuth, data: {name: repo, auto_init: true}});
  expect(created.ok()).toBeTruthy();
  const base = (await created.json()).default_branch;

  try {
    const head = 'pr-head';
    expect((await request.post(`${api}/branches`, {headers: jsonAuth, data: {new_branch_name: head, old_branch_name: base}})).ok()).toBeTruthy();

    const fileContent = `${Array.from({length: 60}, (_, i) => `Line ${i + 1}`).join('\n')}\n`;
    expect((await request.post(`${api}/contents/file.md`, {headers: jsonAuth, data: {branch: head, message: 'add file.md', content: btoa(fileContent)}})).ok()).toBeTruthy();

    const pr = await request.post(`${api}/pulls`, {headers: jsonAuth, data: {head, base, title: 'batch test'}});
    expect(pr.ok()).toBeTruthy();
    const prNumber = (await pr.json()).number;

    await page.goto(`/${username}/${repo}/pulls/${prNumber}/files`);

    await addSuggestion(page, 'Line 20', '```suggestion\nLine 20--batched\n```');
    const firstDiff = page.locator('.suggestion-diff').first();
    await expect(firstDiff).toBeVisible();
    await expect(firstDiff.locator('tr.del-code')).toContainText('Line 20');
    await expect(firstDiff.locator('tr.add-code')).toContainText('Line 20--batched');

    await addSuggestion(page, 'Line 50', '```suggestion\nLine 50--batched\n```');
    const addToBatch = page.getByRole('button', {name: 'Add suggestion to batch'});
    const removeFromBatch = page.getByRole('button', {name: 'Remove from batch'});
    await expect(page.locator('.suggestion-diff')).toHaveCount(2);
    await expect(addToBatch).toHaveCount(2);

    // Editing a suggestion re-renders its diff and keeps its batch button, without a page reload.
    const firstComment = page.locator('.comment').filter({hasText: 'Line 20--batched'});
    await firstComment.getByLabel('Comment menu').click();
    await firstComment.getByRole('button', {name: 'Edit', exact: true}).click();
    await firstComment.getByRole('textbox').fill('```suggestion\nLine 20--edited\n```');
    await firstComment.getByRole('button', {name: 'Save'}).click();
    await expect(firstDiff.locator('tr.add-code')).toContainText('Line 20--edited');
    await expect(addToBatch).toHaveCount(2);

    // Starting a batch is exclusive: single-applies and the review box hide, the batch bar shows the count.
    const applySingle = page.getByRole('button', {name: 'Apply suggestion'}).first();
    const reviewBox = page.locator('#review-box');
    const discard = page.getByRole('button', {name: 'Discard'});
    const commitBatch = page.getByRole('button', {name: 'Commit suggestions'});
    await addToBatch.first().click();
    await addToBatch.first().click(); // each click relabels the button, so the next one is again the first
    await expect(removeFromBatch).toHaveCount(2);
    await expect(applySingle).toBeHidden();
    await expect(reviewBox).toBeHidden();
    await expect(commitBatch).toBeVisible();
    await expect(commitBatch).toHaveText('Commit suggestions (2)');

    // Discarding the batch restores the single-apply flow.
    await discard.click();
    await expect(commitBatch).toBeHidden();
    await expect(addToBatch).toHaveCount(2);
    await expect(applySingle).toBeVisible();
    await expect(reviewBox).toBeVisible();

    // Rebuild the batch and commit it through the shared apply modal.
    await addToBatch.first().click();
    await addToBatch.first().click();
    await commitBatch.click();
    const modal = page.getByRole('dialog');
    await expect(modal).toBeVisible();
    await modal.getByRole('button', {name: 'Apply suggestion'}).click();

    // The batch lands as a single commit on the head branch, carrying the edited content.
    const headContent = async () => {
      const resp = await request.get(`${api}/contents/file.md?ref=${head}`, {headers: auth});
      return resp.ok() ? atob((await resp.json()).content) : '';
    };
    await expect.poll(headContent).toContain('Line 20--edited');
    const final = await headContent();
    expect(final).toContain('Line 50--batched');
    expect(final).not.toContain('Line 20--batched'); // the pre-edit content was superseded

    const commits = await request.get(`${api}/commits?sha=${head}&limit=2`, {headers: auth});
    expect(commits.ok()).toBeTruthy();
    const [latest, previous] = await commits.json();
    expect(previous.commit.message).toContain('add file.md'); // exactly one commit was added by the batch
    expect(latest.commit.message).not.toContain('add file.md');
  } finally {
    await request.delete(api, {headers: auth});
  }
});
