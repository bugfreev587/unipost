import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import assert from 'node:assert/strict';

const files = [
  'scripts/sdk-validation/js/unipost-sdk-test.mjs',
  'scripts/sdk-validation/python/unipost_sdk_test.py',
  'scripts/sdk-validation/go/main.go',
  'scripts/sdk-validation/java/src/main/java/dev/unipost/validation/UnipostSdkTest.java',
];

test('platform credentials round-trip is explicit and uses only supported platforms', async () => {
  for (const file of files) {
    const source = await readFile(new URL(`../../${file}`, import.meta.url), 'utf8');
    assert.match(source, /TEST_PLATFORM_CREDENTIALS_PLATFORM/);
    assert.doesNotMatch(source, /const platformKey = `sdk-js-\$\{Date\.now\(\)\}`/);
    assert.doesNotMatch(source, /platform_name = f"sdk-py-/);
    assert.doesNotMatch(source, /platformName := fmt\.Sprintf\("sdk-go-%d"/);
    assert.doesNotMatch(source, /String platformKey = "sdk-java-" \+/);
    assert.match(source, /twitter.*linkedin.*bluesky.*youtube.*tiktok.*instagram.*threads.*facebook.*pinterest/s);
  }
});

test('go usage regression accepts unlimited post limits', async () => {
  const source = await readFile(new URL('../../scripts/sdk-validation/go/main.go', import.meta.url), 'utf8');
  assert.doesNotMatch(source, /usage\.PostLimit < 0/);
  assert.match(source, /usage\.PostCount < 0/);
});
