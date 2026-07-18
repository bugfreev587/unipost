import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadTikTokAnalyticsRowsModule() {
  const source = readFileSync(resolve("src/components/analytics/tiktok-analytics-rows.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const { buildTikTokPostRows } = await loadTikTokAnalyticsRowsModule();

test("TikTok post rows show unavailable metrics as null when analytics did not match a video", () => {
  const rows = buildTikTokPostRows(
    [
      {
        id: "post_1",
        caption: "Launch recap",
        status: "published",
        results: [
          {
            social_account_id: "sa_tiktok_1",
            status: "published",
            external_id: "7390000000000000001",
          },
        ],
      },
    ],
    [
      {
        status: "fulfilled",
        value: { data: null },
      },
    ],
    "sa_tiktok_1",
  );

  assert.deepEqual(rows, [
    {
      title: "Launch recap",
      status: "published",
      videoId: "7390000000000000001",
      views: null,
      likes: null,
      comments: null,
      shares: null,
    },
  ]);
});

test("TikTok post rows preserve legitimate zero metrics for a matched video", () => {
  const rows = buildTikTokPostRows(
    [
      {
        id: "post_2",
        caption: "Fresh post",
        status: "published",
        results: [{ social_account_id: "sa_tiktok_1", status: "published", external_id: "publish_2" }],
      },
    ],
    [
      {
        status: "fulfilled",
        value: {
          data: [{
            social_account_id: "sa_tiktok_1",
            video_views: 0,
            likes: 0,
            comments: 0,
            shares: 0,
            platform_specific: { tiktok_video_id: "7663542984343883021" },
          }],
        },
      },
    ],
    "sa_tiktok_1",
  );

  assert.equal(rows[0].videoId, "7663542984343883021");
  assert.equal(rows[0].views, 0);
  assert.equal(rows[0].likes, 0);
  assert.equal(rows[0].comments, 0);
  assert.equal(rows[0].shares, 0);
});
