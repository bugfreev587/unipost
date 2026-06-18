import assert from "node:assert/strict";
import { test } from "node:test";
import {
  AGENT_CATALOG_VERSION,
  MCP_AGENT_TOOLS,
  UNIPOST_AGENT_INTENTS,
  intentByName,
  planForIntent,
  registerAgentContractTools,
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
    "diagnose_logs",
    "explain_request_failure",
  ]);

  assert.equal(intentByName("create_draft_post").safety_level, "draft_write");
  assert.equal(intentByName("plan_publish_post").requires_user_confirmation, true);
  assert.equal(intentByName("upload_media").canonical_actions[0], "media.upload");
  assert.equal(intentByName("diagnose_logs").canonical_actions[1], "logs.stream");
  assert.equal(intentByName("explain_request_failure").safety_level, "read_only");

  const toolNames = MCP_AGENT_TOOLS.map((tool) => tool.name);
  assert.deepEqual(toolNames, [
    "unipost_agent_capabilities",
    "unipost_agent_context",
    "unipost_agent_plan",
    "unipost_debug_recent_logs",
    "unipost_debug_explain_request",
    "unipost_debug_stream_info",
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

test("Phase 5 MCP debug tools expose recent logs, request explanations, and stream instructions", async () => {
  const calls = [];
  const tools = new Map();
  const server = {
    tool(name, description, schema, handler) {
      tools.set(name, { description, schema, handler });
    },
  };
  registerAgentContractTools(server, async (path) => {
    calls.push(path);
    if (path.startsWith("/v1/logs/42")) {
      return { data: { id: 42, status: "error", request_id: "req_42", message: "Bad account" } };
    }
    return {
      data: [{ id: 42, status: "error", request_id: "req_42", message: "Bad account" }],
      meta: { has_more: false },
    };
  }, { apiUrl: "https://dev-api.unipost.dev" });

  const recent = JSON.parse((await tools.get("unipost_debug_recent_logs").handler({ status: "error", limit: 5 })).content[0].text);
  assert.equal(calls[0], "/v1/logs?status=error&limit=5");
  assert.equal(recent.logs[0].id, 42);

  const explained = JSON.parse((await tools.get("unipost_debug_explain_request").handler({ log_id: "42" })).content[0].text);
  assert.equal(calls[1], "/v1/logs/42");
  assert.equal(explained.summary, "The request failed with error status.");

  const stream = JSON.parse((await tools.get("unipost_debug_stream_info").handler({ status: "error", after_id: "41" })).content[0].text);
  assert.equal(stream.stream.url, "https://dev-api.unipost.dev/v1/logs/stream?status=error&after_id=41");
  assert.equal(stream.stream.headers.Authorization, "Bearer <UNIPOST_API_KEY>");
  assert.equal(stream.reconnect.last_event_id, "Use the last received SSE event id as Last-Event-ID on reconnect.");
});
