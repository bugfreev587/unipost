import assert from "node:assert/strict";
import { test } from "node:test";
import {
  AGENT_CATALOG_VERSION,
  MCP_AGENT_TOOLS,
  UNIPOST_AGENT_INTENTS,
  intentByName,
  planForIntent,
} from "../dist/agent-contract.js";

test("Phase 5 MCP agent contract mirrors CLI intent names and safety levels", () => {
  assert.equal(AGENT_CATALOG_VERSION, "2026-06-03.phase5");

  const intentNames = UNIPOST_AGENT_INTENTS.map((intent) => intent.name);
  assert.deepEqual(intentNames, [
    "diagnose_setup",
    "diagnose_account",
    "create_draft_post",
    "plan_publish_post",
    "connect_account",
    "upload_media",
    "generate_post_example",
  ]);

  assert.equal(intentByName("create_draft_post").safety_level, "draft_write");
  assert.equal(intentByName("plan_publish_post").requires_user_confirmation, true);
  assert.equal(intentByName("upload_media").canonical_actions[0], "media.upload");

  const toolNames = MCP_AGENT_TOOLS.map((tool) => tool.name);
  assert.deepEqual(toolNames, [
    "unipost_agent_capabilities",
    "unipost_agent_context",
    "unipost_agent_plan",
  ]);
});

test("Phase 5 MCP agent plans preserve intent-specific confirmation names", () => {
  const publishPlan = planForIntent({
    intent: "plan_publish_post",
    account_ids: ["sa_1"],
    caption: "Ready for review",
  });
  assert.deepEqual(publishPlan.required_user_confirmations, ["approve_live_publish"]);

  const uploadPlan = planForIntent({
    intent: "upload_media",
    file_path: "/tmp/clip.mp4",
  });
  assert.deepEqual(uploadPlan.required_user_confirmations, ["approve_local_file_upload"]);
});
