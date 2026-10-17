// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// @watch start
// web_src/js/webcomponents/citation-information.ts
// @watch end

import {expect} from '@playwright/test';
import {test} from './utils_e2e.ts';

test('CITATION.cff switch', async ({page}) => {
  const previewPath = '/user2/rendering-test/src/branch/master/CITATION.cff';

  const response = await page.goto(previewPath);
  expect(response?.status()).toBe(200);

  await expect(page.getByText('cff-version: 1.2.0')).toBeVisible();

  await page.getByRole('button', {name: 'BibTeX'}).click();
  await expect(page.getByText('cff-version: 1.2.0')).toBeHidden();
  await expect(
    page.getByText('howpublished = {https://forgejo.org/},'),
  ).toBeVisible();

  await page.getByRole('button', {name: 'Citation File Format'}).click();
  await expect(page.getByText('cff-version: 1.2.0')).toBeVisible();
});

test('glb file with 3D rendering', async ({page}, workerInfo) => {
  test.skip(
    workerInfo.project.name !== 'chromium',
    'needs some investigation to run on other platforms',
    // https://codeberg.org/forgejo/forgejo/actions/runs/113344/jobs/3/attempt/1
  );

  const previewPath =
    '/user2/rendering-test/src/branch/master/Unicode❤♻Test.glb';

  const response = await page.goto(previewPath);
  expect(response?.status()).toBe(200);

  await page
    .getByRole('img', {
      name: '3D model. Use mouse, touch or arrow keys to move.',
    })
    .click();
});
