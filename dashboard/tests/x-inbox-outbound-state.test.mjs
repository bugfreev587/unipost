import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadOutboundState() {
  const source = readFileSync(resolve("src/lib/x-inbox-outbound-state.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const logicalInput = {
  workspaceId: "ws_1",
  accountId: "sa_x",
  source: "x_dm",
  targetItemId: "inbox_1",
  threadKey: "dm_conversation_1",
  bodyHash: "sha256-body-one",
};

test("an uncertain retry reuses the same logical operation and idempotency key", async () => {
  const { beginXInboxOutboundOperation, updateXInboxOutboundOperation } = await loadOutboundState();
  const first = beginXInboxOutboundOperation([], logicalInput, () => "key_one", "2026-07-16T12:00:00Z");
  const uncertain = updateXInboxOutboundOperation(first.operations, first.operation.logicalKey, {
    status: "outcome_unknown",
    operationId: "op_1",
  });
  const retry = beginXInboxOutboundOperation(uncertain, logicalInput, () => "key_two", "2026-07-16T12:01:00Z");

  assert.equal(retry.reused, true);
  assert.equal(retry.operation.idempotencyKey, "key_one");
  assert.equal(retry.operation.operationId, "op_1");
  assert.equal(retry.operations.length, 1);
});

test("a deliberately changed body creates a new logical operation and key", async () => {
  const { beginXInboxOutboundOperation } = await loadOutboundState();
  const first = beginXInboxOutboundOperation([], logicalInput, () => "key_one", "2026-07-16T12:00:00Z");
  const changed = beginXInboxOutboundOperation(
    first.operations,
    { ...logicalInput, bodyHash: "sha256-body-two" },
    () => "key_two",
    "2026-07-16T12:01:00Z",
  );

  assert.equal(changed.reused, false);
  assert.equal(changed.operation.idempotencyKey, "key_two");
  assert.equal(changed.operations.length, 2);
});

test("refresh hydration retains unresolved operation id and key without reply body", async () => {
  const {
    beginXInboxOutboundOperation,
    loadXInboxOutboundOperations,
    saveXInboxOutboundOperations,
    updateXInboxOutboundOperation,
  } = await loadOutboundState();
  const values = new Map();
  const storage = {
    getItem(key) { return values.get(key) ?? null; },
    setItem(key, value) { values.set(key, value); },
    removeItem(key) { values.delete(key); },
  };
  const first = beginXInboxOutboundOperation([], logicalInput, () => "key_one", "2026-07-16T12:00:00Z");
  const reconciling = updateXInboxOutboundOperation(first.operations, first.operation.logicalKey, {
    status: "remote_succeeded",
    operationId: "op_1",
  });
  saveXInboxOutboundOperations(storage, reconciling);

  const hydrated = loadXInboxOutboundOperations(storage, "ws_1");
  assert.equal(hydrated.length, 1);
  assert.equal(hydrated[0].idempotencyKey, "key_one");
  assert.equal(hydrated[0].operationId, "op_1");
  assert.equal("body" in hydrated[0], false);
});

test("a definitive completion removes the retained logical operation", async () => {
  const { beginXInboxOutboundOperation, resolveXInboxOutboundOperation } = await loadOutboundState();
  const first = beginXInboxOutboundOperation([], logicalInput, () => "key_one", "2026-07-16T12:00:00Z");
  assert.deepEqual(
    resolveXInboxOutboundOperation(first.operations, first.operation.logicalKey),
    [],
  );
});

test("backend status classification blocks unresolved work and releases completed work", async () => {
  const { classifyXInboxOutboundStatus } = await loadOutboundState();
  assert.deepEqual(classifyXInboxOutboundStatus("outcome_unknown"), {
    terminal: false,
    manual: false,
  });
  assert.deepEqual(classifyXInboxOutboundStatus("needs_reconciliation"), {
    terminal: false,
    manual: true,
  });
  assert.deepEqual(classifyXInboxOutboundStatus("completed"), {
    terminal: true,
    manual: false,
  });
});
