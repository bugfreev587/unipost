import assert from "node:assert/strict";
import { test } from "node:test";

import {
  PreviewPendingError,
  PreviewTerminalError,
  selectRailwayEnvironment,
  selectRailwayPreviewAPI,
} from "./railway-deployments.mjs";

const sha = "a".repeat(40);

test("resolves the Railway environment ID from the exact successful GitHub deployment", () => {
  const result = selectRailwayEnvironment([
    {
      id: 41,
      sha,
      environment: "UniPost / unipost-pr-42",
      payload: { environmentId: "env-pr-42" },
      statuses: [{
        state: "success",
        environment_url: "https://railway.com/project/project-id?environmentId=env-pr-42",
      }],
    },
  ], sha);

  assert.deepEqual(result, {
    environmentId: "env-pr-42",
    deploymentId: 41,
    environment: "UniPost / unipost-pr-42",
    githubState: "success",
    sha,
  });
});

test("selects only the preview API whose Railway deployment matches the exact SHA", () => {
  const result = selectRailwayPreviewAPI({
    id: "env-pr-42",
    name: "unipost-pr-42",
    serviceInstances: {
      edges: [{
        node: {
          serviceName: "preview-api",
          latestDeployment: {
            status: "SUCCESS",
            meta: { commitHash: sha },
          },
          domains: {
            serviceDomains: [{ domain: "preview-api-unipost-pr-42.up.railway.app" }],
            customDomains: [],
          },
        },
      }],
    },
  }, sha);

  assert.deepEqual(result, {
    apiURL: "https://preview-api-unipost-pr-42.up.railway.app",
    railwayEnvironmentId: "env-pr-42",
    railwayEnvironmentName: "unipost-pr-42",
  });
});

test("rejects a Railway environment attached to another SHA", () => {
  assert.throws(
    () => selectRailwayEnvironment([
      {
        id: 1,
        sha: "b".repeat(40),
        environment: "UniPost / unipost-pr-42",
        payload: { environmentId: "env-pr-42" },
        statuses: [{ state: "success" }],
      },
    ], sha),
    (error) => error instanceof PreviewPendingError && /exact head SHA/.test(error.message),
  );
});

test("rejects a terminal Railway environment failure for the exact SHA", () => {
  assert.throws(
    () => selectRailwayEnvironment([
      {
        id: 1,
        sha,
        environment: "UniPost / unipost-pr-42",
        payload: { environmentId: "env-pr-42" },
        statuses: [{ state: "failure" }],
      },
    ], sha),
    (error) => error instanceof PreviewTerminalError && /failure/.test(error.message),
  );
});

test("uses an inactive GitHub deployment to discover the exact PR environment", () => {
  const result = selectRailwayEnvironment([
    {
      id: 1,
      sha,
      environment: "UniPost / unipost-pr-42",
      payload: { environmentId: "env-pr-42" },
      statuses: [{ state: "inactive" }],
    },
  ], sha);

  assert.equal(result.environmentId, "env-pr-42");
  assert.equal(result.githubState, "inactive");
});

test("rejects a persistent Railway environment", () => {
  assert.throws(
    () => selectRailwayPreviewAPI({
      id: "dev-id",
      name: "dev",
      serviceInstances: { edges: [] },
    }, sha),
    (error) => error instanceof PreviewTerminalError && /not an ephemeral/.test(error.message),
  );
});

test("rejects a successful preview API built from another SHA", () => {
  assert.throws(
    () => selectRailwayPreviewAPI({
      id: "env-pr-42",
      name: "unipost-pr-42",
      serviceInstances: {
        edges: [{
          node: {
            serviceName: "preview-api",
            latestDeployment: {
              status: "SUCCESS",
              meta: { commitHash: "b".repeat(40) },
            },
            domains: {
              serviceDomains: [{ domain: "preview-api-unipost-pr-42.up.railway.app" }],
            },
          },
        }],
      },
    }, sha),
    (error) => error instanceof PreviewPendingError && /exact head SHA/.test(error.message),
  );
});

test("accepts an exact preview API that has gone to sleep", () => {
  const result = selectRailwayPreviewAPI({
    id: "env-pr-42",
    name: "unipost-pr-42",
    serviceInstances: {
      edges: [{
        node: {
          serviceName: "preview-api",
          latestDeployment: {
            status: "SLEEPING",
            meta: { commitHash: sha },
          },
          domains: {
            serviceDomains: [{ domain: "preview-api-unipost-pr-42.up.railway.app" }],
          },
        },
      }],
    },
  }, sha);

  assert.equal(result.apiURL, "https://preview-api-unipost-pr-42.up.railway.app");
});
