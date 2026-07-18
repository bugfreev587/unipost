import assert from "node:assert/strict";
import test from "node:test";

import {
  cleanupPreviewAliases,
  selectPreviewAliases,
} from "./vercel-alias-cleanup.mjs";

test("selects only aliases owned by one pull request", () => {
  assert.deepEqual(
    selectPreviewAliases(
      [
        { alias: "unipost-dev-pr-215.vercel.app" },
        { alias: "unipost-dev-pr-215-12345-1.vercel.app" },
        { alias: "unipost-dev-pr-215-12346-1.vercel.app" },
        { alias: "unipost-dev-pr-215-not-a-run.vercel.app" },
        { alias: "unipost-dev-pr-216-12347-1.vercel.app" },
        { alias: "unipost-dev.vercel.app" },
      ],
      "215",
    ),
    [
      "unipost-dev-pr-215.vercel.app",
      "unipost-dev-pr-215-12345-1.vercel.app",
      "unipost-dev-pr-215-12346-1.vercel.app",
    ],
  );
});

test("lists and deletes only aliases owned by the pull request", async () => {
  const requests = [];
  const fetchImpl = async (url, options = {}) => {
    requests.push({
      url: url.toString(),
      method: options.method ?? "GET",
      authorization: options.headers?.Authorization,
    });
    if ((options.method ?? "GET") === "GET") {
      return new Response(
        JSON.stringify({
          aliases: [
            { alias: "unipost-dev-pr-215-12345-1.vercel.app" },
            { alias: "unipost-dev-pr-216-12345-1.vercel.app" },
          ],
          pagination: {},
        }),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify({ status: "SUCCESS" }), {
      status: 200,
    });
  };

  const deleted = await cleanupPreviewAliases({
    pullRequestNumber: "215",
    projectId: "project-id",
    teamId: "team-id",
    token: "token",
    fetchImpl,
  });

  assert.deepEqual(deleted, [
    "unipost-dev-pr-215-12345-1.vercel.app",
  ]);
  assert.equal(requests.length, 2);
  assert.match(requests[0].url, /\/v4\/aliases\?/);
  assert.match(requests[0].url, /projectId=project-id/);
  assert.match(
    requests[1].url,
    /\/v2\/aliases\/unipost-dev-pr-215-12345-1\.vercel\.app\?/,
  );
  assert.equal(requests[1].method, "DELETE");
  assert.equal(requests[1].authorization, "Bearer token");
});

test("treats an already-missing alias as successfully cleaned", async () => {
  const fetchImpl = async (_url, options = {}) => {
    if ((options.method ?? "GET") === "GET") {
      return new Response(
        JSON.stringify({
          aliases: [
            { alias: "unipost-dev-pr-215-12345-1.vercel.app" },
          ],
          pagination: {},
        }),
        { status: 200 },
      );
    }
    return new Response(
      JSON.stringify({ error: { message: "Alias not found" } }),
      { status: 404, statusText: "Not Found" },
    );
  };

  await assert.doesNotReject(
    cleanupPreviewAliases({
      pullRequestNumber: "215",
      projectId: "project-id",
      teamId: "team-id",
      token: "token",
      fetchImpl,
    }),
  );
});
