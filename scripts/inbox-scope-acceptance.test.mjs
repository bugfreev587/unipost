import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const acceptance = readFileSync(join(scriptDirectory, "inbox-scope-acceptance.mjs"), "utf8");
const coverageMatrix = readFileSync(join(scriptDirectory, "../docs/sdk-api-coverage-matrix.md"), "utf8");

function section(startMarker, endMarker) {
  const start = acceptance.indexOf(startMarker);
  const end = acceptance.indexOf(endMarker, start + startMarker.length);
  assert.ok(start >= 0 && end > start, `acceptance section ${startMarker} must be inspectable`);
  return acceptance.slice(start, end);
}

test("cross-scope reply is validation-safe before any provider path", () => {
  const replyProbe = section('await assertNotFound("cross-scope reply"', 'await assertNotFound("cross-scope thread-state"');
  assert.match(replyProbe, /\/reply`, \{\s*\}\s*\);/s, "cross-scope reply must send an empty JSON object");
  assert.doesNotMatch(replyProbe, /\btext\s*:/, "cross-scope reply must not contain deliverable content");
  assert.match(coverageMatrix, /empty JSON object[^\n]{0,240}validation[^\n]{0,160}provider/i);
  assert.doesNotMatch(coverageMatrix, /reply credential must be non-deliverable/i);
});

test("acceptance I/O has configurable finite abort, destroy, and kill cleanup", () => {
  for (const name of [
    "INBOX_ACCEPT_HTTP_TIMEOUT_MS",
    "INBOX_ACCEPT_WS_UPGRADE_TIMEOUT_MS",
    "INBOX_ACCEPT_WS_EVENT_TIMEOUT_MS",
    "INBOX_ACCEPT_WS_READY_TIMEOUT_MS",
    "INBOX_ACCEPT_PSQL_TIMEOUT_MS",
    "INBOX_ACCEPT_PSQL_KILL_GRACE_MS",
  ]) {
    assert.match(acceptance, new RegExp(name), `${name} must be configurable`);
  }
  assert.match(acceptance, /function timeoutSetting/);

  const api = section("async function api(", "function responseData");
  assert.match(api, /new AbortController\(\)/);
  assert.match(api, /controller\.abort\(\)/);
  assert.match(api, /signal:\s*controller\.signal/);
  assert.match(api, /await response\.text\(\)/);
  assert.match(api, /finally\s*\{[\s\S]*clearTimeout\(/);

  const connect = section("  async connect()", "  consume(chunk)");
  assert.match(connect, /request\.destroy\(/, "upgrade timeout must destroy the ClientRequest");
  assert.match(connect, /socket\.destroy\(\)/, "rejected upgrades must close the socket");
  assert.match(connect, /clearTimeout\(/);
  assert.match(connect, /request\.off\(/, "upgrade listeners must be cleaned up");

  const psql = section("async function emitFixtureEvents", "const managedAQuery");
  assert.match(psql, /child\.kill\("SIGTERM"\)/);
  assert.match(psql, /child\.kill\("SIGKILL"\)/);
  assert.match(psql, /child\.off\(/, "psql listeners must be cleaned up");
  assert.match(psql, /clearTimeout\(/);
  assert.match(psql, /code === 0/);
  assert.match(psql, /shell:\s*false/);

  const waiters = section("  waitFor(", "  close()");
  assert.match(waiters, /setTimeout\(/, "every event waiter needs a deadline");
  assert.match(waiters, /this\.waiters\.delete\(waiter\)/);
  assert.match(waiters, /clearTimeout\(waiter\.timer\)/);

  const close = section("  close()", "function maskedClientFrame");
  assert.match(close, /catch\s*\{\s*\}/, "cleanup must absorb a close-frame write failure");
  assert.match(close, /finally\s*\{[\s\S]*socket\.destroy\(\)/, "cleanup must always destroy the socket");
});

test("WebSocket isolation uses readiness and a same-session causal barrier", () => {
  assert.doesNotMatch(acceptance, /new Promise\(\(resolve\) => setTimeout\(resolve,/i, "fixed sleeps are not readiness evidence");
  assert.doesNotMatch(acceptance, /setTimeout\([^\n]*(?:150|500)\b/, "legacy timing windows must stay removed");
  assert.match(acceptance, /async function establishWebSocketReadiness/);
  assert.match(acceptance, /Promise\.allSettled\(/, "readiness retries must let every bounded waiter clean up");
  assert.match(acceptance, /ready-a-/);
  assert.match(acceptance, /await establishWebSocketReadiness\(/);

  for (const field of ["type", "probe_id", "workspace_id", "external_user_id"]) {
    assert.match(acceptance, new RegExp(`message\\?\\.${field} ===`), `fixture matcher must bind ${field}`);
  }

  const psql = section("async function emitFixtureEvents", "const managedAQuery");
  assert.match(psql, /events\.map\(/, "one psql session must accept ordered event batches");
  assert.match(psql, /BEGIN READ ONLY/);
  assert.match(psql, /COMMIT/);

  assert.match(acceptance, /const orderedIsolationEvents = \[/);
  assert.match(acceptance, /fixtureEvent\(workspaceID, externalUserB, probeB\)[\s\S]*fixtureEvent\(workspaceID, externalUserA, barrierA\)/);
  assert.match(acceptance, /await emitFixtureEvents\(orderedIsolationEvents\)/);
  assert.match(acceptance, /aggregateBIndex < aggregateBarrierIndex/);
  assert.match(acceptance, /slice\(0, managedBarrierIndex \+ 1\)\.some\(matchesB\)/);
  assert.match(acceptance, /finally\s*\{[\s\S]*managedSocket\.close\(\)[\s\S]*aggregateSocket\.close\(\)/);
});
