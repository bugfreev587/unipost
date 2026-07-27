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

function createHarness(
  startPublishingRestrictionsRefresh,
  { getToken = async () => "token", initialRestrictions = [] } = {},
) {
  const requests = [];
  const committed = [];
  const loaded = [];
  const errors = [];
  const cleared = [];
  let currentRestrictions = initialRestrictions;
  let focusListener = null;
  let removedListener = null;

  const cleanup = startPublishingRestrictionsRefresh({
    getToken,
    loadRestrictions: async () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    clearRestrictions: () => {
      currentRestrictions = [];
      cleared.push(true);
    },
    commitRestrictions: (restrictions) => {
      currentRestrictions = restrictions;
      committed.push(restrictions);
    },
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
    cleared,
    get currentRestrictions() {
      return currentRestrictions;
    },
    cleanup,
    focus: () => {
      assert.ok(focusListener, "focus listener was not registered");
      focusListener();
    },
    listenerWasRemoved: () => removedListener === focusListener,
  };
}

async function loadRefreshEffectWiring() {
  const source = await readFile(drawerSourcePath, "utf8");
  const sourceFile = ts.createSourceFile(
    drawerSourcePath.pathname,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const drawerNode = sourceFile.statements.find(
    (statement) =>
      ts.isFunctionDeclaration(statement) && statement.name?.text === "CreatePostDrawer",
  );
  assert.ok(drawerNode, "create-post drawer component must be declared");

  let refreshEffect = null;
  const visit = (node) => {
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === "useEffect" &&
      node.arguments.some((argument) =>
        argument.getText(sourceFile).includes("startPublishingRestrictionsRefresh"),
      )
    ) {
      refreshEffect = node;
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(drawerNode);
  assert.ok(refreshEffect, "drawer must wire the restrictions coordinator through useEffect");
  return { sourceFile, refreshEffect };
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

test("cleanup while authentication is pending prevents the restrictions request", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const pendingToken = deferred();
  const harness = createHarness(startPublishingRestrictionsRefresh, {
    getToken: () => pendingToken.promise,
  });

  harness.cleanup();
  pendingToken.resolve("token");
  await flushMicrotasks();

  assert.equal(harness.requests.length, 0, "closed drawer must not request restrictions after authentication resolves");
  assert.deepEqual(harness.committed, []);
  assert.deepEqual(harness.loaded, []);
  assert.deepEqual(harness.errors, []);
});

test("a newer focus generation prevents an older authenticated generation from requesting restrictions", async () => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const tokens = [deferred(), deferred()];
  let tokenIndex = 0;
  const harness = createHarness(startPublishingRestrictionsRefresh, {
    getToken: () => tokens[tokenIndex++].promise,
  });

  harness.focus();
  tokens[0].resolve("stale-token");
  await flushMicrotasks();
  assert.equal(harness.requests.length, 0, "stale generation must stop before the restrictions API call");

  tokens[1].resolve("current-token");
  await flushMicrotasks();
  assert.equal(harness.requests.length, 1, "current focus generation must request restrictions");
  harness.requests[0].resolve([]);
  await flushMicrotasks();
  harness.cleanup();
});

test("a new workspace lifecycle clears the prior workspace snapshot when refresh cannot replace it", async (t) => {
  const startPublishingRestrictionsRefresh = await loadRefreshCoordinator();
  const restrictedA = [{ platform: "tiktok", enabled: true }];

  for (const mode of ["missing token", "request failure"]) {
    await t.test(mode, async () => {
      const workspaceA = createHarness(startPublishingRestrictionsRefresh);
      await flushMicrotasks();
      workspaceA.requests[0].resolve(restrictedA);
      await flushMicrotasks();
      assert.deepEqual(workspaceA.currentRestrictions, restrictedA);
      workspaceA.cleanup();

      const workspaceB = createHarness(startPublishingRestrictionsRefresh, {
        getToken: mode === "missing token" ? async () => null : async () => "workspace-b-token",
        initialRestrictions: workspaceA.currentRestrictions,
      });
      await flushMicrotasks();
      if (mode === "request failure") {
        workspaceB.requests[0].reject(new Error("workspace B projection unavailable"));
        await flushMicrotasks();
      }

      assert.deepEqual(
        workspaceB.currentRestrictions,
        [],
        "workspace B must never apply workspace A restrictions",
      );
      assert.deepEqual(workspaceB.cleared, [true], "each lifecycle must clear exactly once");
      assert.deepEqual(workspaceB.loaded, [true], "advisory refresh failure must not block workspace B");
      workspaceB.cleanup();
    });
  }
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

test("the drawer effect returns coordinator cleanup and restarts it when workspace changes", async () => {
  const { sourceFile, refreshEffect } = await loadRefreshEffectWiring();
  const [effectCallback, dependencies] = refreshEffect.arguments;
  assert.ok(
    ts.isArrowFunction(effectCallback) || ts.isFunctionExpression(effectCallback),
    "refresh effect must use an executable callback",
  );
  assert.ok(ts.isBlock(effectCallback.body), "refresh effect callback must have a cleanup-capable body");

  const returnsCoordinatorCleanup = effectCallback.body.statements.some(
    (statement) =>
      ts.isReturnStatement(statement) &&
      statement.expression !== undefined &&
      statement.expression.getText(sourceFile).startsWith("startPublishingRestrictionsRefresh("),
  );
  assert.equal(
    returnsCoordinatorCleanup,
    true,
    "refresh effect must return the coordinator cleanup so old workspace generations become inactive",
  );

  const coordinatorCall = effectCallback.body.statements
    .filter(ts.isReturnStatement)
    .map((statement) => statement.expression)
    .find(
      (expression) =>
        expression &&
        ts.isCallExpression(expression) &&
        expression.expression.getText(sourceFile) === "startPublishingRestrictionsRefresh",
    );
  assert.ok(coordinatorCall, "refresh effect must return the restrictions coordinator call");
  const coordinatorOptions = coordinatorCall.arguments[0];
  assert.ok(ts.isObjectLiteralExpression(coordinatorOptions), "coordinator must receive lifecycle options");
  const clearProperty = coordinatorOptions.properties.find(
    (property) => property.name?.getText(sourceFile) === "clearRestrictions",
  );
  assert.ok(clearProperty, "workspace lifecycle must wire a restrictions snapshot reset");
  assert.match(
    clearProperty.getText(sourceFile),
    /setPublishingRestrictions\(\[\]\)/,
    "workspace reset must clear the prior restrictions snapshot",
  );

  assert.ok(ts.isArrayLiteralExpression(dependencies), "refresh effect must declare dependencies");
  const dependencyNames = dependencies.elements.map((element) => element.getText(sourceFile));
  assert.deepEqual(
    dependencyNames,
    ["open", "getToken", "workspaceId"],
    "open, auth, and workspace identity must each restart the refresh lifecycle",
  );
});
