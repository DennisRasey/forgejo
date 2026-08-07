import {expect, type Page} from '@playwright/test';
import {AxeBuilder} from '@axe-core/playwright';

export async function accessibilityCheck({page}: {page: Page}, includes: string[], excludes: string[], disabledRules: string[], withRules: string[] | null = null) {
  // contrast of inline links is still a global issue in Forgejo
  disabledRules.push('link-in-text-block');

  let accessibilityScanner = new AxeBuilder({page});

  if (withRules === null) {
    accessibilityScanner = accessibilityScanner.disableRules(disabledRules);
  } else {
    accessibilityScanner = accessibilityScanner.withRules(withRules);
  }

  // passing the whole array seems to be not supported,
  // iterating has the nice side-effectof skipping this if the array is empty
  for (const incl of includes) {
    // passing the whole array seems to be not supported
    accessibilityScanner = accessibilityScanner.include(incl);
  }
  for (const excl of excludes) {
    accessibilityScanner = accessibilityScanner.exclude(excl);
  }

  // Scan the page both in dark and light theme.
  //
  // Have observed failures during this scanning which are understood to be caused by CSS transitions, either applied to
  // whatever last action occurred on the page before `accessibilityCheck` was called, or during the transition from
  // dark to light.  As there are a variety of transitions in Forgejo's CSS files (primarily in fomantic) with ease
  // elements between 0.1 and 0.3 seconds, we give the accessibility scanner up to 2s to settle into success for each
  // scan.
  await expect(async () => {
    const accessibilityScanResults = await accessibilityScanner.analyze();
    expect(accessibilityScanResults.violations).toEqual([]);
  }).toPass({timeout: 2000});

  await page.emulateMedia({colorScheme: 'dark'});

  await expect(async () => {
    const accessibilityScanResults = await accessibilityScanner.analyze();
    expect(accessibilityScanResults.violations).toEqual([]);
  }).toPass({timeout: 2000});

  await page.emulateMedia({colorScheme: 'light'});
}
