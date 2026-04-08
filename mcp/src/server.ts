#!/usr/bin/env node

/**
 * UniPost Remote MCP Server (SSE + Streamable HTTP)
 *
 * Deployment: mcp.unipost.dev
 *
 * Claude Desktop config (Streamable HTTP — recommended):
 *   {
 *     "mcpServers": {
 *       "unipost": {
 *         "url": "https://mcp.unipost.dev/mcp",
 *         "headers": {
 *           "Authorization": "Bearer up_live_xxx"
 *         }
 *       }
 *     }
 *   }
 *
 * Legacy SSE config:
 *   {
 *     "mcpServers": {
 *       "unipost": {
 *         "url": "https://mcp.unipost.dev/sse",
 *         "headers": {
 *           "Authorization": "Bearer up_live_xxx"
 *         }
 *       }
 *     }
 *   }
 */

import http from "node:http";
import { randomUUID } from "node:crypto";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { SSEServerTransport } from "@modelcontextprotocol/sdk/server/sse.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";

const API_URL = process.env.UNIPOST_API_URL || "https://api.unipost.dev";
const PORT = parseInt(process.env.PORT || "3001", 10);

// ── API helper (uses per-request API key) ──
async function apiRequest(path: string, apiKey: string, options?: RequestInit) {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
      ...options?.headers,
    },
  });
  return res.json();
}

// ── Create MCP server with tools (API key injected per-session) ──
function createMcpServer(apiKey: string): McpServer {
  const server = new McpServer({
    name: "unipost",
    version: "0.2.0",
  });

  server.tool(
    "unipost_list_accounts",
    "List all connected social media accounts",
    {
      platform: z
        .string()
        .optional()
        .describe("Filter by platform (bluesky, linkedin, instagram, threads, tiktok, youtube, twitter)"),
    },
    async ({ platform }) => {
      const data = await apiRequest("/v1/social-accounts", apiKey);
      let accounts = data.data || [];
      if (platform) {
        accounts = accounts.filter((a: any) => a.platform === platform);
      }
      return { content: [{ type: "text" as const, text: JSON.stringify(accounts, null, 2) }] };
    }
  );

  // unipost_create_post accepts BOTH the legacy shape (caption +
  // account_ids — single caption fanned out to every account) and the
  // new AgentPost shape (platform_posts[] — different caption per
  // account, optional per-post media + options). Pass exactly one of
  // them. The new shape is preferred when generating "different copy
  // per platform" — the LLM-readable description below tells Claude
  // / GPT to reach for it whenever the message should differ across
  // platforms (Twitter terse, LinkedIn long-form, etc.).
  server.tool(
    "unipost_create_post",
    [
      "Publish a social post to one or more accounts.",
      "",
      "TWO REQUEST SHAPES — pass exactly one:",
      "",
      "1. Same caption everywhere (legacy):",
      "     { caption, account_ids, media_urls?, scheduled_at? }",
      "",
      "2. Different caption per platform (preferred for multi-platform fan-out):",
      "     { platform_posts: [{ account_id, caption, media_urls?, platform_options? }] }",
      "",
      "Use shape 2 whenever you want to tailor the message to each",
      "network (e.g. terse on Twitter, long-form on LinkedIn). Each",
      "platform_posts entry becomes ONE platform post — listing the",
      "same account_id twice produces two posts on that account.",
      "",
      "Both shapes accept an optional idempotency_key — passing the",
      "same key within 24h returns the original response unchanged",
      "(no duplicate posts created).",
      "",
      "Call unipost_get_capabilities first to learn each platform's",
      "caption length and media limits, or unipost_validate_post to",
      "preflight a draft before publishing.",
    ].join("\n"),
    {
      // Legacy fields.
      caption: z
        .string()
        .optional()
        .describe("Legacy: single caption used for every account_id."),
      account_ids: z
        .array(z.string())
        .optional()
        .describe("Legacy: accounts to fan out to. Use platform_posts instead for per-platform captions."),
      media_urls: z
        .array(z.string())
        .optional()
        .describe("Legacy: shared media URLs. Ignored when platform_posts is set."),

      // New shape.
      platform_posts: z
        .array(
          z.object({
            account_id: z.string(),
            caption: z.string(),
            media_urls: z.array(z.string()).optional(),
            platform_options: z.record(z.string(), z.any()).optional(),
            in_reply_to: z.string().optional(),
          })
        )
        .optional()
        .describe(
          "Preferred: array of per-account posts with their own captions, media, and options."
        ),

      // Common.
      scheduled_at: z
        .string()
        .optional()
        .describe("RFC3339 timestamp. If set, post is queued and published by the scheduler at that time."),
      idempotency_key: z
        .string()
        .optional()
        .describe(
          "Optional idempotency key. Same key + same project within 24h returns the prior response unchanged."
        ),
    },
    async (args) => {
      const body: any = {};
      if (args.platform_posts?.length) {
        body.platform_posts = args.platform_posts;
      } else if (args.account_ids?.length) {
        body.caption = args.caption ?? "";
        body.account_ids = args.account_ids;
        if (args.media_urls?.length) body.media_urls = args.media_urls;
      } else {
        return {
          content: [
            {
              type: "text" as const,
              text: 'Error: pass either { platform_posts: [...] } or { caption, account_ids: [...] }',
            },
          ],
        };
      }
      if (args.scheduled_at) body.scheduled_at = args.scheduled_at;
      if (args.idempotency_key) body.idempotency_key = args.idempotency_key;

      const data = await apiRequest("/v1/social-posts", apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  // Pure preflight — no DB writes, no platform API calls. Returns
  // the same { valid, errors, warnings } shape as the REST endpoint.
  // LLM clients should call this BEFORE create_post when generating
  // a draft so they can self-correct length / media / threading
  // errors without burning a publish round-trip.
  server.tool(
    "unipost_validate_post",
    "Pre-flight a draft post against every per-platform limit. Returns { valid, errors, warnings } without writing anything or calling any platform API. Same body shape as unipost_create_post.",
    {
      caption: z.string().optional(),
      account_ids: z.array(z.string()).optional(),
      media_urls: z.array(z.string()).optional(),
      platform_posts: z
        .array(
          z.object({
            account_id: z.string(),
            caption: z.string(),
            media_urls: z.array(z.string()).optional(),
            platform_options: z.record(z.string(), z.any()).optional(),
            in_reply_to: z.string().optional(),
          })
        )
        .optional(),
      scheduled_at: z.string().optional(),
    },
    async (args) => {
      const body: any = {};
      if (args.platform_posts?.length) {
        body.platform_posts = args.platform_posts;
      } else if (args.account_ids?.length) {
        body.caption = args.caption ?? "";
        body.account_ids = args.account_ids;
        if (args.media_urls?.length) body.media_urls = args.media_urls;
      }
      if (args.scheduled_at) body.scheduled_at = args.scheduled_at;
      const data = await apiRequest("/v1/social-posts/validate", apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  // Capabilities map — caption length, image / video count caps,
  // first-comment + threading + scheduling support, etc. The LLM
  // should call this before drafting so it can size content correctly.
  server.tool(
    "unipost_get_capabilities",
    "Return the per-platform publish-time capabilities map (caption length, media counts, file size hints, threading, scheduling, first comment). LLMs should call this before drafting a post so the content respects each platform's limits.",
    {},
    async () => {
      const data = await apiRequest("/v1/platforms/capabilities", apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  server.tool(
    "unipost_get_post",
    "Get the status and details of a published post",
    { post_id: z.string().describe("The post ID") },
    async ({ post_id }) => {
      const data = await apiRequest(`/v1/social-posts/${post_id}`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  server.tool(
    "unipost_get_analytics",
    "Get engagement metrics for a published post",
    { post_id: z.string().describe("The post ID") },
    async ({ post_id }) => {
      const data = await apiRequest(`/v1/social-posts/${post_id}/analytics`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  server.tool(
    "unipost_list_posts",
    "List recent posts with their status",
    {
      status: z.string().optional().describe("Filter by status: scheduled, published, failed"),
    },
    async ({ status }) => {
      let path = "/v1/social-posts";
      if (status) path += `?status=${status}`;
      const data = await apiRequest(path, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  return server;
}

// ── HTTP Server ──
// Each SSE connection gets its own MCP server instance with the user's API key.
// The SSE transport assigns a sessionId internally and sends it to the client
// via the `endpoint` event. The client then includes it in POST /messages?sessionId=xxx.
// We track transports by matching the sessionId from the query param.
const transports = new Set<SSEServerTransport>();
const streamableSessions = new Map<string, { transport: StreamableHTTPServerTransport; server: McpServer }>();

function extractApiKey(req: http.IncomingMessage): string | null {
  const authHeader = req.headers.authorization || "";
  const apiKey = authHeader.replace(/^Bearer\s+/i, "").trim();
  return apiKey || null;
}

const httpServer = http.createServer(async (req, res) => {
  // CORS headers
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type, mcp-session-id");
  res.setHeader("Access-Control-Expose-Headers", "mcp-session-id");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  const url = new URL(req.url || "/", `http://${req.headers.host}`);

  // Health check
  if (url.pathname === "/" || url.pathname === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok", service: "unipost-mcp", version: "0.2.0" }));
    return;
  }

  // ── Streamable HTTP endpoint (recommended for Claude Desktop) ──
  if (url.pathname === "/mcp") {
    const apiKey = extractApiKey(req);
    if (!apiKey) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Authorization header with Bearer token required" }));
      return;
    }

    // Existing session
    const sessionId = req.headers["mcp-session-id"] as string | undefined;
    if (sessionId && streamableSessions.has(sessionId)) {
      const session = streamableSessions.get(sessionId)!;
      await session.transport.handleRequest(req, res);
      return;
    }

    // New session (must be a POST with initialize)
    if (req.method === "POST") {
      const transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: () => randomUUID(),
        onsessioninitialized: (newSessionId) => {
          streamableSessions.set(newSessionId, { transport, server });
        },
      });

      transport.onclose = () => {
        if (transport.sessionId) {
          streamableSessions.delete(transport.sessionId);
        }
      };

      const server = createMcpServer(apiKey);
      await server.connect(transport);
      await transport.handleRequest(req, res);
      return;
    }

    res.writeHead(400, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Invalid or missing session. Send an initialize request first." }));
    return;
  }

  // ── Legacy SSE endpoint ──
  // SSE connection endpoint
  if (url.pathname === "/sse" && req.method === "GET") {
    const authHeader = req.headers.authorization || "";
    const apiKey = authHeader.replace(/^Bearer\s+/i, "");

    if (!apiKey) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Authorization header with Bearer token required" }));
      return;
    }

    const transport = new SSEServerTransport("/messages", res);
    const server = createMcpServer(apiKey);

    transports.add(transport);
    res.on("close", () => {
      transports.delete(transport);
    });

    await server.connect(transport);
    return;
  }

  // Message endpoint (POST from MCP client)
  if (url.pathname === "/messages" && req.method === "POST") {
    const sessionId = url.searchParams.get("sessionId");
    if (!sessionId) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "sessionId query parameter required" }));
      return;
    }

    // Find the transport that owns this sessionId by trying handlePostMessage
    // on each transport — only the matching one will process it
    let handled = false;
    for (const transport of transports) {
      try {
        await transport.handlePostMessage(req, res);
        handled = true;
        break;
      } catch {
        // Not this transport's session — continue
      }
    }

    if (!handled) {
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Session not found. Reconnect to /sse" }));
    }
    return;
  }

  // 404
  res.writeHead(404, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: "Not found" }));
});

httpServer.listen(PORT, () => {
  console.log(`UniPost MCP Server (SSE) listening on port ${PORT}`);
  console.log(`  Streamable HTTP: http://localhost:${PORT}/mcp`);
  console.log(`  SSE (legacy):    http://localhost:${PORT}/sse`);
  console.log(`  Health check:    http://localhost:${PORT}/health`);
});
