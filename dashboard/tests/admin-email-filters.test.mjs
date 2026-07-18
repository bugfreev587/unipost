import assert from "node:assert/strict";
import test from "node:test";

import { buildAttemptedDateRange } from "../src/app/admin/email/filters.ts";
import { listAdminEmailNotifications } from "../src/lib/api.ts";

test("builds an inclusive same-day range in the viewer timezone", () => {
  const originalTimezone = process.env.TZ;
  process.env.TZ = "America/Los_Angeles";
  try {
    assert.deepEqual(
      buildAttemptedDateRange("2026-07-18", "2026-07-18"),
      {
        start_at: "2026-07-18T07:00:00.000Z",
        end_at: "2026-07-19T07:00:00.000Z",
      },
    );
  } finally {
    process.env.TZ = originalTimezone;
  }
});

test("constructs the next local midnight across a midnight DST transition", () => {
  const originalTimezone = process.env.TZ;
  process.env.TZ = "America/Santiago";
  try {
    assert.deepEqual(
      buildAttemptedDateRange("", "2026-09-06"),
      { end_at: "2026-09-07T03:00:00.000Z" },
    );
  } finally {
    process.env.TZ = originalTimezone;
  }
});

test("rejects a reversed calendar range", () => {
  assert.deepEqual(
    buildAttemptedDateRange("2026-07-19", "2026-07-18"),
    { error: "End date must be on or after start date." },
  );
});

test("serializes recipient and attempted boundaries while omitting All emails", async () => {
  const originalFetch = globalThis.fetch;
  const requestedURLs = [];
  globalThis.fetch = async (input) => {
    requestedURLs.push(String(input));
    return new Response(JSON.stringify({ data: [], meta: { total: 0 } }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };

  try {
    await listAdminEmailNotifications("token", { email: "all" });
    await listAdminEmailNotifications("token", {
      email: "person+filter@example.com",
      start_at: "2026-07-18T07:00:00.000Z",
      end_at: "2026-07-19T07:00:00.000Z",
    });
  } finally {
    globalThis.fetch = originalFetch;
  }

  const allURL = new URL(requestedURLs[0]);
  assert.equal(allURL.searchParams.has("email"), false);

  const filteredURL = new URL(requestedURLs[1]);
  assert.equal(filteredURL.searchParams.get("email"), "person+filter@example.com");
  assert.equal(filteredURL.searchParams.get("start_at"), "2026-07-18T07:00:00.000Z");
  assert.equal(filteredURL.searchParams.get("end_at"), "2026-07-19T07:00:00.000Z");
});
