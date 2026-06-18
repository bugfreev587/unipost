import { z } from "zod";

export const AGENT_CATALOG_VERSION = "2026-06-03.phase5";

export const UNIPOST_AGENT_INTENTS = [
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
  {
    name: "diagnose_logs",
    safety_level: "read_only",
    requires_user_confirmation: false,
    required_user_confirmations: [],
    required_inputs: [],
    optional_inputs: ["status", "category", "request_id", "since", "after_id"],
    canonical_actions: ["logs.list", "logs.stream"],
  },
  {
    name: "explain_request_failure",
    safety_level: "read_only",
    requires_user_confirmation: false,
    required_user_confirmations: [],
    required_inputs: [],
    optional_inputs: ["request_id", "log_id"],
    canonical_actions: ["doctor.explain", "logs.get"],
  },
] as const;

export const MCP_AGENT_TOOLS = [
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
  {
    name: "unipost_debug_recent_logs",
    description: "Fetch recent workspace-scoped UniPost logs for agent debugging.",
  },
  {
    name: "unipost_debug_explain_request",
    description: "Explain one UniPost log entry or request id and suggest safe next debugging actions.",
  },
  {
    name: "unipost_debug_stream_info",
    description: "Return SSE log stream connection instructions for live agent debugging.",
  },
] as const;

export function intentByName(name: string) {
  const intent = UNIPOST_AGENT_INTENTS.find((item) => item.name === name);
  if (!intent) {
    throw new Error(`Unsupported UniPost intent: ${name}`);
  }
  return intent;
}

export function agentCapabilitiesPayload() {
  return {
    catalog_version: AGENT_CATALOG_VERSION,
    status_enums: {
      post: ["draft", "scheduled", "publishing", "published", "partial", "failed", "canceled"],
      connect_session: ["pending", "completed", "expired", "canceled"],
      media: ["pending", "processing", "ready", "failed"],
    },
    intents: UNIPOST_AGENT_INTENTS,
    mcp_tools: MCP_AGENT_TOOLS,
  };
}

export function planForIntent(input: Record<string, unknown>) {
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

export function registerAgentContractTools(
  server: any,
  apiRequest: (path: string, options?: RequestInit) => Promise<any>,
  options: { apiUrl?: string } = {}
) {
  server.tool(
    "unipost_agent_capabilities",
    MCP_AGENT_TOOLS[0].description,
    {},
    async () => textContent(agentCapabilitiesPayload())
  );

  server.tool(
    "unipost_agent_context",
    MCP_AGENT_TOOLS[1].description,
    {},
    async () => {
      const [workspace, profiles, accounts] = await Promise.all([
        apiRequest("/v1/workspace"),
        apiRequest("/v1/profiles"),
        apiRequest("/v1/accounts"),
      ]);
      return textContent({
        catalog_version: AGENT_CATALOG_VERSION,
        workspace: workspace?.data ?? workspace,
        profiles: profiles?.data ?? [],
        accounts: accounts?.data ?? [],
      });
    }
  );

  server.tool(
    "unipost_agent_plan",
    MCP_AGENT_TOOLS[2].description,
    {
      intent: z.string().describe("One of the UniPost agent capability intent names."),
      account_id: z.string().optional(),
      account_ids: z.array(z.string()).optional(),
      caption: z.string().optional(),
      platform: z.string().optional(),
      file_path: z.string().optional(),
      content_type: z.string().optional(),
      scheduled_at: z.string().optional(),
      idempotency_key: z.string().optional(),
    },
    async (args: Record<string, unknown>) => textContent({
      catalog_version: AGENT_CATALOG_VERSION,
      ...planForIntent(args),
    })
  );

  server.tool(
    "unipost_debug_recent_logs",
    MCP_AGENT_TOOLS[3].description,
    logFilterSchema(),
    async (args: Record<string, unknown>) => {
      const path = logsListPath(args);
      const response = await apiRequest(path);
      return textContent({
        catalog_version: AGENT_CATALOG_VERSION,
        query: path,
        logs: response?.data ?? [],
        meta: response?.meta ?? {},
        request_id: response?.request_id ?? "",
      });
    }
  );

  server.tool(
    "unipost_debug_explain_request",
    MCP_AGENT_TOOLS[4].description,
    {
      request_id: z.string().optional().describe("UniPost request_id to find in the workspace logs."),
      log_id: z.string().optional().describe("Exact UniPost log id to fetch."),
    },
    async (args: Record<string, unknown>) => {
      const logID = stringValue(args.log_id);
      const requestID = stringValue(args.request_id);
      if (!logID && !requestID) {
        return textContent({
          catalog_version: AGENT_CATALOG_VERSION,
          error: "Pass either log_id or request_id.",
          safe_next_actions: ["Run unipost_debug_recent_logs with status=error, then retry with a log_id."],
        });
      }

      const response = logID
        ? await apiRequest(`/v1/logs/${encodeURIComponent(logID)}`)
        : await apiRequest(`/v1/logs?request_id=${encodeURIComponent(requestID)}&limit=1`);
      const log = logID ? response?.data : (response?.data ?? [])[0] ?? null;
      return textContent({
        catalog_version: AGENT_CATALOG_VERSION,
        log,
        summary: explainLog(log),
        safe_next_actions: safeNextActionsForLog(log),
        request_id: response?.request_id ?? "",
      });
    }
  );

  server.tool(
    "unipost_debug_stream_info",
    MCP_AGENT_TOOLS[5].description,
    {
      ...logFilterSchema(),
      after_id: z.string().optional().describe("Replay retained log rows with id greater than this value before live events."),
    },
    async (args: Record<string, unknown>) => {
      const url = new URL("/v1/logs/stream", options.apiUrl || "https://api.unipost.dev");
      appendLogFilters(url.searchParams, args);
      const afterID = stringValue(args.after_id);
      if (afterID) url.searchParams.set("after_id", afterID);

      return textContent({
        catalog_version: AGENT_CATALOG_VERSION,
        stream: {
          url: url.toString(),
          method: "GET",
          headers: {
            Accept: "text/event-stream",
            Authorization: "Bearer <UNIPOST_API_KEY>",
          },
        },
        event_shape: {
          id: "log id",
          event: "log",
          data: "IntegrationLog JSON",
        },
        reconnect: {
          after_id: "Pass after_id=<last_log_id> to replay retained rows newer than that id.",
          last_event_id: "Use the last received SSE event id as Last-Event-ID on reconnect.",
          precedence: "after_id wins over Last-Event-ID when both are present.",
        },
      });
    }
  );
}

function logFilterSchema() {
  return {
    status: z.string().optional().describe("Filter logs by status: success, warning, or error."),
    category: z.string().optional().describe("Filter logs by category."),
    action: z.string().optional().describe("Filter logs by action."),
    source: z.string().optional().describe("Filter logs by source."),
    level: z.string().optional().describe("Filter logs by level."),
    platform: z.string().optional().describe("Filter logs by platform."),
    profile_id: z.string().optional().describe("Filter logs by profile id."),
    social_account_id: z.string().optional().describe("Filter logs by social account id."),
    post_id: z.string().optional().describe("Filter logs by post id."),
    request_id: z.string().optional().describe("Filter logs by request id."),
    error_code: z.string().optional().describe("Filter logs by error code."),
    from: z.string().optional().describe("RFC3339 lower timestamp bound."),
    to: z.string().optional().describe("RFC3339 upper timestamp bound."),
    limit: z.number().int().min(1).max(100).optional().describe("Maximum rows to return."),
  };
}

const LOG_FILTER_KEYS = [
  "status",
  "category",
  "action",
  "source",
  "level",
  "platform",
  "profile_id",
  "social_account_id",
  "post_id",
  "request_id",
  "error_code",
  "from",
  "to",
  "limit",
] as const;

function logsListPath(args: Record<string, unknown>) {
  const params = new URLSearchParams();
  appendLogFilters(params, args);
  const qs = params.toString();
  return qs ? `/v1/logs?${qs}` : "/v1/logs";
}

function appendLogFilters(params: URLSearchParams, args: Record<string, unknown>) {
  for (const key of LOG_FILTER_KEYS) {
    const value = args[key];
    if (value === undefined || value === null || value === "" || value === "all") continue;
    params.set(key, String(value));
  }
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function explainLog(log: any) {
  if (!log) {
    return "No matching log was found in the retained workspace logs.";
  }
  if (log.status === "error") {
    return "The request failed with error status.";
  }
  if (log.status === "warning") {
    return "The request completed with warning status.";
  }
  return "The request completed successfully.";
}

function safeNextActionsForLog(log: any) {
  if (!log) {
    return [
      "List recent error logs with unipost_debug_recent_logs.",
      "Confirm the request id belongs to the same UniPost workspace as the API key.",
    ];
  }
  const actions = [
    "Compare the log endpoint, action, status, error_code, and message against the integration code.",
    "Run the smallest non-destructive request that reproduces the same request_id or error_code.",
  ];
  const message = String(log.message || log.error_code || "").toLowerCase();
  if (message.includes("auth") || message.includes("unauthorized") || log.http_status_code === 401) {
    actions.unshift("Verify the API key prefix, base URL, and Authorization: Bearer header.");
  }
  if (message.includes("account") || log.social_account_id) {
    actions.unshift("Check account health and reconnect the social account if required.");
  }
  return actions;
}

function textContent(value: unknown) {
  return {
    content: [
      {
        type: "text" as const,
        text: JSON.stringify(value, null, 2),
      },
    ],
  };
}
