import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadInboxModel() {
  const source = readFileSync(resolve("src/lib/inbox-model.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const base = {
  workspace_id: "ws_1",
  social_account_id: "sa_x",
  thread_status: "open",
  is_read: false,
  is_own: false,
  received_at: "2026-07-16T12:00:00Z",
  created_at: "2026-07-16T12:00:00Z",
};

test("X replies group by canonical X conversation id across reply depths", async () => {
  const { canonicalInboxConversationKey, groupInboxItemsByConversation } = await loadInboxModel();
  const items = [
    {
      ...base,
      id: "one",
      source: "x_reply",
      external_id: "tweet_2",
      parent_external_id: "tweet_1",
      thread_key: "conversation_1",
      body: "First reply",
    },
    {
      ...base,
      id: "two",
      source: "x_reply",
      external_id: "tweet_3",
      parent_external_id: "tweet_2",
      thread_key: "conversation_1",
      body: "Nested reply",
      received_at: "2026-07-16T12:02:00Z",
    },
  ];

  assert.equal(
    canonicalInboxConversationKey(items[0], items),
    "sa_x:x_reply:conversation_1",
  );
  const groups = groupInboxItemsByConversation(items, "x_reply");
  assert.equal(groups.length, 1);
  assert.deepEqual(groups[0].items.map((item) => item.id), ["one", "two"]);
});

test("X DMs group by canonical DM conversation id instead of sender", async () => {
  const { groupInboxItemsByConversation } = await loadInboxModel();
  const items = [
    {
      ...base,
      id: "inbound",
      source: "x_dm",
      external_id: "dm_1",
      author_id: "user_a",
      thread_key: "dm_conversation_4",
    },
    {
      ...base,
      id: "outbound",
      source: "x_dm",
      external_id: "dm_2",
      author_id: "unipost_account",
      thread_key: "dm_conversation_4",
      is_own: true,
      received_at: "2026-07-16T12:03:00Z",
    },
  ];

  const groups = groupInboxItemsByConversation(items, "x_dm");
  assert.equal(groups.length, 1);
  assert.equal(groups[0].threadKey, "dm_conversation_4");
  assert.deepEqual(groups[0].items.map((item) => item.id), ["inbound", "outbound"]);
});
