import assert from "node:assert/strict";
import test from "node:test";

import {
  createShareableLink,
  extractShareableURL,
} from "./vercel-share-link.mjs";

test("extracts an alias-scoped Vercel shareable URL", () => {
  const expectedHost = "unipost-dev-pr-215-12345-1.vercel.app";
  const url = extractShareableURL(
    {
      protectionBypassUrl: `https://${expectedHost}/?_vercel_share=temporary-token`,
    },
    expectedHost,
  );

  assert.equal(
    url,
    `https://${expectedHost}/?_vercel_share=temporary-token`,
  );
});

test("accepts Vercel short share URLs and rejects unsafe responses", () => {
  assert.equal(
    extractShareableURL(
      { protectionBypassUrl: "https://vercel.sh/s/temporary-token" },
      "unipost-dev-pr-215-12345-1.vercel.app",
    ),
    "https://vercel.sh/s/temporary-token",
  );

  assert.throws(
    () =>
      extractShareableURL(
        { protectionBypassUrl: "http://example.com/share" },
        "unipost-dev-pr-215-12345-1.vercel.app",
      ),
    /valid Vercel shareable URL/,
  );
});

test("builds an alias-scoped URL from Vercel's raw share token response", () => {
  const expectedHost = "unipost-dev-pr-215-12345-1.vercel.app";
  assert.equal(
    extractShareableURL(
      { value: "temporary_share-token.123456" },
      expectedHost,
    ),
    `https://${expectedHost}/?_vercel_share=temporary_share-token.123456`,
  );

  assert.throws(
    () => extractShareableURL({ value: "unsafe token" }, expectedHost),
    /valid Vercel shareable URL/,
  );
});

test("creates a one-day shareable link for the isolated alias", async () => {
  let request;
  const host = "unipost-dev-pr-215-12345-1.vercel.app";
  const url = await createShareableLink({
    host,
    teamId: "team-id",
    token: "token",
    fetchImpl: async (endpoint, options) => {
      request = {
        endpoint: endpoint.toString(),
        method: options.method,
        authorization: options.headers.Authorization,
        body: JSON.parse(options.body),
      };
      return new Response(
        JSON.stringify("temporary_share-token.123456"),
        { status: 200 },
      );
    },
  });

  assert.equal(
    url,
    `https://${host}/?_vercel_share=temporary_share-token.123456`,
  );
  assert.match(
    request.endpoint,
    /\/aliases\/unipost-dev-pr-215-12345-1\.vercel\.app\/protection-bypass\?teamId=team-id$/,
  );
  assert.equal(request.method, "PATCH");
  assert.equal(request.authorization, "Bearer token");
  assert.deepEqual(request.body, { ttl: 86_400 });
});
