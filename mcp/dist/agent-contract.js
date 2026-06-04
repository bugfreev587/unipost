"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.MCP_AGENT_TOOLS = exports.UNIPOST_AGENT_INTENTS = exports.AGENT_CATALOG_VERSION = void 0;
exports.intentByName = intentByName;
exports.agentCapabilitiesPayload = agentCapabilitiesPayload;
exports.planForIntent = planForIntent;
exports.registerAgentContractTools = registerAgentContractTools;
const zod_1 = require("zod");
exports.AGENT_CATALOG_VERSION = "2026-06-03.phase5";
exports.UNIPOST_AGENT_INTENTS = [
    {
        name: "diagnose_setup",
        safety_level: "read_only",
        requires_user_confirmation: false,
        required_user_confirmations: [],
        required_inputs: [],
        optional_inputs: ["client"],
        canonical_actions: ["agent.bootstrap", "agent.context"],
    },
    {
        name: "diagnose_account",
        safety_level: "read_only",
        requires_user_confirmation: false,
        required_user_confirmations: [],
        required_inputs: ["account_id"],
        optional_inputs: [],
        canonical_actions: ["accounts.health", "accounts.capabilities", "accounts.metrics"],
    },
    {
        name: "create_draft_post",
        safety_level: "draft_write",
        requires_user_confirmation: false,
        required_user_confirmations: [],
        required_inputs: ["account_ids", "caption"],
        optional_inputs: ["media_ids", "platform_posts"],
        canonical_actions: ["posts.validate", "posts.draft"],
    },
    {
        name: "plan_publish_post",
        safety_level: "live_write_plan",
        requires_user_confirmation: true,
        required_user_confirmations: ["approve_live_publish"],
        required_inputs: ["account_ids", "caption"],
        optional_inputs: ["scheduled_at", "media_ids", "platform_posts", "idempotency_key"],
        canonical_actions: ["posts.validate", "posts.create_dry_run", "posts.create"],
    },
    {
        name: "connect_account",
        safety_level: "setup_write",
        requires_user_confirmation: false,
        required_user_confirmations: [],
        required_inputs: ["platform"],
        optional_inputs: ["profile_id", "return_url", "external_user_id", "external_user_email"],
        canonical_actions: ["connect.create", "connect.wait"],
    },
    {
        name: "upload_media",
        safety_level: "setup_write",
        requires_user_confirmation: true,
        required_user_confirmations: ["approve_local_file_upload"],
        required_inputs: ["file_path"],
        optional_inputs: ["content_type"],
        canonical_actions: ["media.upload", "media.wait"],
    },
    {
        name: "generate_post_example",
        safety_level: "read_only",
        requires_user_confirmation: false,
        required_user_confirmations: [],
        required_inputs: [],
        optional_inputs: ["language", "account_ids", "caption"],
        canonical_actions: ["examples.posts.create"],
    },
];
exports.MCP_AGENT_TOOLS = [
    {
        name: "unipost_agent_capabilities",
        description: "Return the UniPost agent intent catalog, safety levels, canonical actions, and status enums.",
    },
    {
        name: "unipost_agent_context",
        description: "Return workspace, profiles, and connected accounts for agent grounding.",
    },
    {
        name: "unipost_agent_plan",
        description: "Convert an explicit UniPost intent plus structured inputs into safe canonical actions.",
    },
];
function intentByName(name) {
    const intent = exports.UNIPOST_AGENT_INTENTS.find((item) => item.name === name);
    if (!intent) {
        throw new Error(`Unsupported UniPost intent: ${name}`);
    }
    return intent;
}
function agentCapabilitiesPayload() {
    return {
        catalog_version: exports.AGENT_CATALOG_VERSION,
        status_enums: {
            post: ["draft", "scheduled", "publishing", "published", "partial", "failed", "canceled"],
            connect_session: ["pending", "completed", "expired", "canceled"],
            media: ["pending", "processing", "ready", "failed"],
        },
        intents: exports.UNIPOST_AGENT_INTENTS,
        mcp_tools: exports.MCP_AGENT_TOOLS,
    };
}
function planForIntent(input) {
    const intentName = String(input.intent || "");
    const intent = intentByName(intentName);
    const missing_inputs = intent.required_inputs.filter((key) => {
        const value = input[key];
        return Array.isArray(value) ? value.length === 0 : !value;
    });
    return {
        intent: intent.name,
        safety_level: intent.safety_level,
        missing_inputs,
        required_user_confirmations: [...intent.required_user_confirmations],
        safe_to_execute_without_user: missing_inputs.length === 0 && !intent.requires_user_confirmation,
        actions: missing_inputs.length === 0
            ? intent.canonical_actions.map((canonical_action) => ({
                canonical_action,
                safety_level: intent.safety_level,
            }))
            : [],
    };
}
function registerAgentContractTools(server, apiRequest) {
    server.tool("unipost_agent_capabilities", exports.MCP_AGENT_TOOLS[0].description, {}, async () => textContent(agentCapabilitiesPayload()));
    server.tool("unipost_agent_context", exports.MCP_AGENT_TOOLS[1].description, {}, async () => {
        const [workspace, profiles, accounts] = await Promise.all([
            apiRequest("/v1/workspace"),
            apiRequest("/v1/profiles"),
            apiRequest("/v1/accounts"),
        ]);
        return textContent({
            catalog_version: exports.AGENT_CATALOG_VERSION,
            workspace: workspace?.data ?? workspace,
            profiles: profiles?.data ?? [],
            accounts: accounts?.data ?? [],
        });
    });
    server.tool("unipost_agent_plan", exports.MCP_AGENT_TOOLS[2].description, {
        intent: zod_1.z.string().describe("One of the UniPost agent capability intent names."),
        account_id: zod_1.z.string().optional(),
        account_ids: zod_1.z.array(zod_1.z.string()).optional(),
        caption: zod_1.z.string().optional(),
        platform: zod_1.z.string().optional(),
        file_path: zod_1.z.string().optional(),
        content_type: zod_1.z.string().optional(),
        scheduled_at: zod_1.z.string().optional(),
        idempotency_key: zod_1.z.string().optional(),
    }, async (args) => textContent({
        catalog_version: exports.AGENT_CATALOG_VERSION,
        ...planForIntent(args),
    }));
}
function textContent(value) {
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(value, null, 2),
            },
        ],
    };
}
