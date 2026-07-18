import assert from "node:assert/strict";
import { test } from "node:test";

import {
  PreviewPendingError,
  PreviewTerminalError,
  selectReadyRailwayAPI,
} from "./railway-deployments.mjs";

const sha = "a".repeat(40);

test("selects the successful Railway API deployment for the exact SHA", () => {
  const result = selectReadyRailwayAPI([
    {
      id: 1,
      sha,
      environment: "pr-42",
      statuses: [{
        state: "success",
        environment_url: "https://api-pr-42.up.railway.app",
        created_at: "2026-07-17T18:00:00Z",
      }],
    },
    {
      id: 2,
      sha,
      environment: "pr-42-worker",
      statuses: [{ state: "success", environment_url: "" }],
    },
    {
      id: 3,
      sha,
      environment: "Preview",
      statuses: [{
        state: "success",
        environment_url: "https://unipost.vercel.app",
      }],
    },
  ], sha);

  assert.deepEqual(result, {
    apiURL: "https://api-pr-42.up.railway.app",
    deploymentId: 1,
    environment: "pr-42",
    sha,
  });
});

test("rejects a successful Railway URL attached to another SHA", () => {
  assert.throws(
    () => selectReadyRailwayAPI([
      {
        id: 1,
        sha: "b".repeat(40),
        environment: "pr-42",
        statuses: [{
          state: "success",
          environment_url: "https://api-pr-42.up.railway.app",
        }],
      },
    ], sha),
    (error) => error instanceof PreviewPendingError && /exact head SHA/.test(error.message),
  );
});

test("rejects a terminal Railway failure for the exact SHA", () => {
  assert.throws(
    () => selectReadyRailwayAPI([
      {
        id: 1,
        sha,
        environment: "pr-42",
        statuses: [{
          state: "failure",
          environment_url: "https://api-pr-42.up.railway.app",
        }],
      },
    ], sha),
    (error) => error instanceof PreviewTerminalError && /failure/.test(error.message),
  );
});

test("does not accept persistent or missing Railway URLs", () => {
  assert.throws(
    () => selectReadyRailwayAPI([
      {
        id: 1,
        sha,
        environment: "dev",
        statuses: [{
          state: "success",
          environment_url: "https://dev-api.unipost.dev",
        }],
      },
      {
        id: 2,
        sha,
        environment: "pr-42-worker",
        statuses: [{ state: "success", environment_url: "" }],
      },
    ], sha),
    (error) => error instanceof PreviewPendingError && /ready Railway PR API/.test(error.message),
  );
});

test("rejects multiple distinct Railway API URLs for one SHA", () => {
  assert.throws(
    () => selectReadyRailwayAPI([
      {
        id: 1,
        sha,
        environment: "pr-42",
        statuses: [{
          state: "success",
          environment_url: "https://api-pr-42.up.railway.app",
        }],
      },
      {
        id: 2,
        sha,
        environment: "pr-43",
        statuses: [{
          state: "success",
          environment_url: "https://api-pr-43.up.railway.app",
        }],
      },
    ], sha),
    (error) => error instanceof PreviewTerminalError && /multiple Railway PR API/.test(error.message),
  );
});
