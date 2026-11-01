import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const workflow = await readFile(new URL('../../.github/workflows/regression-monitor.yml', import.meta.url), 'utf8');
const smoke = await readFile(new URL('../smoke-test.sh', import.meta.url), 'utf8');

test('UniPost API regression jobs sharing one workspace run sequentially', () => {
  assert.match(workflow, /sdk_js:\n\s+needs: smoke\n\s+if: always\(\)/);
  assert.match(workflow, /sdk_python:\n\s+needs: sdk_js\n\s+if: always\(\)/);
  assert.match(workflow, /sdk_go:\n\s+needs: sdk_python\n\s+if: always\(\)/);
  assert.match(workflow, /sdk_java:\n\s+needs: sdk_go\n\s+if: always\(\)/);
});

test('smoke regression only selects active account fixtures', () => {
  for (const variable of ['TWITTER_ID', 'IG_ID', 'TEXT_ID', 'ANY_ID']) {
    assert.match(
      smoke,
      new RegExp(variable + '=.*select\\(\\.status == "active" or \\.status == "connected"\\)'),
      variable + ' must ignore disconnected and reconnect-required accounts',
    );
  }
});

test('smoke regression treats plan gates as capability skips', () => {
  assert.match(smoke, /is_plan_gate_response\(\)/);
  assert.match(smoke, /is_validation_plan_gate_response\(\)/);
  assert.match(smoke, /skip "API key mint regression" "plan-gated"/);
  assert.match(smoke, /skip "Analytics summary regression" "plan-gated"/);
  assert.match(smoke, /skip "Audit log regression" "plan-gated"/);
  assert.match(smoke, /skip "YouTube connect session regression" "plan-gated"/);
  assert.match(smoke, /skip "Webhook CRUD regression" "plan-gated"/);
  assert.match(smoke, /skip "validate happy path" "platform plan-gated"/);
  assert.match(smoke, /skip "long-caption test" "platform plan-gated"/);
  assert.match(smoke, /skip "thread validation tests" "platform plan-gated"/);
});
