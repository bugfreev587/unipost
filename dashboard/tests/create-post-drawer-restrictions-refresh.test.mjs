import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const drawerSourcePath = new URL(
  "../src/components/posts/create-post/create-post-drawer.tsx",
  import.meta.url,
);

async function loadRefreshCoordinator() {
  const source = await readFile(drawerSourcePath, "utf8");
  const sourceFile = ts.createSourceFile(
    drawerSourcePath.pathname,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const functionNode = sourceFile.statements.find(
    (statement) =>
      ts.isFunctionDeclaration(statement) &&
      statement.name?.text === "startPublishingRestrictionsRefresh",
  );
  assert.ok(
    functionNode,
    "create-post drawer must define the executable restrictions refresh coordinator",
  );

  const functionSource = source.slice(functionNode.getStart(sourceFile), functionNode.end);
  const transpiled = ts.transpileModule(
    `${functionSource}\nmodule.exports = { startPublishingRestrictionsRefresh };`,
    {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    },
  ).outputText;
  const loadedModule = { exports: {} };
  new Function("module", "exports", transpiled)(loadedModule, loadedModule.exports);
  return loadedModule.exports.startPublishingRestrictionsRefresh;
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

async function flushMicrotasks() {
  await Promise.resolve();
  await Promise.resolve();
}

function createHarness(startPublishingRestrictionsRefresh) {
  const requests = [];
  const committed = [];
  const loaded = [];
  const errors = [];
  let focusListener = null;
  let removedListener = null;

  const cleanup = startPublishingRestrictionsRefresh({
    getToken: async () => "token",
    loadRestrictions: async () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    commitRestrictions: (restrictions) => committed.push(restrictions),
    markLoaded: () => loaded.push(true),
    reportError: (error) => errors.push(error),
    addFocusListener: (listener) => {
      focusListener = listener;
    },
    removeFocusListener: (listener) => {
      removedListener = listener;
    },
  });

  return {
    requests,
    committed,
    loaded,
    errors,
    cleanup,
    focus: () => {
      assert.ok(focusListener, "focus listener was not registered");
      focusListener();
    },
    listenerWasRemoved: () => removedListener === focusListener,
  };
}

test("only the latest open/focus generation commits restrictions when responses finish out of order", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const harness = createHarness(startPublishingRestrictionsRefresh);
  await flushMicrotasks();
  assert.equal(harness.requests.length, 1, "drawer open must start the initial refresh");

  harness.focus();
  await flushMicrotasks();
  assert.equal(harness.requests.length, 2, "window focus must start a new refresh generation");

  const newest = [{ platform: "tiktok", enabled: true }];
  harness.requests[1].resolve(newest);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, [newest]);
  assert.deepEqual(harness.loaded, [true]);

  harness.requests[0].resolve([{ platform: "tiktok", enabled: false }]);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, [newest], "stale initial response must not overwrite the focus snapshot");
  assert.deepEqual(harness.loaded, [true], "stale finally block must not update loaded state");
  assert.deepEqual(harness.errors, []);
  harness.cleanup();
  assert.equal(harness.listenerWasRemoved(), true);
});

test("a stale completion cannot mark restrictions loaded while the latest focus refresh is pending", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const harness = createHarness(startPublishingRestrictionsRefresh);
  await flushMicrotasks();
  harness.focus();
  await flushMicrotasks();

  harness.requests[0].resolve([{ platform: "tiktok", enabled: false }]);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, []);
  assert.deepEqual(harness.loaded, []);

  const newest = [{ platform: "tiktok", enabled: true }];
  harness.requests[1].resolve(newest);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, [newest]);
  assert.deepEqual(harness.loaded, [true]);
  harness.cleanup();
});

test("cleanup prevents state writes after close or unmount", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const harness = createHarness(startPublishingRestrictionsRefresh);
  await flushMicrotasks();
  harness.cleanup();
  assert.equal(harness.listenerWasRemoved(), true);

  harness.requests[0].resolve([{ platform: "tiktok", enabled: true }]);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, []);
  assert.deepEqual(harness.loaded, []);
  assert.deepEqual(harness.errors, []);
});

test("the latest refresh failure releases advisory loading and future focus refresh still works", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const harness = createHarness(startPublishingRestrictionsRefresh);
  await flushMicrotasks();

  const failure = new Error("projection unavailable");
  harness.requests[0].reject(failure);
  await flushMicrotasks();
  assert.deepEqual(harness.errors, [failure]);
  assert.deepEqual(harness.loaded, [true], "latest failure must not block the advisory projection forever");

  harness.focus();
  await flushMicrotasks();
  const recovered = [{ platform: "tiktok", enabled: false }];
  harness.requests[1].resolve(recovered);
  await flushMicrotasks();
  assert.deepEqual(harness.committed, [recovered]);
  assert.deepEqual(harness.loaded, [true, true]);
  harness.cleanup();
});
