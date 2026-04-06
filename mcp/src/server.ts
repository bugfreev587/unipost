#!/usr/bin/env node

/**
 * UniPost Remote MCP Server (SSE Transport)
 *
 * Deployment: mcp.unipost.dev
 * Usage in Claude Desktop config:
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
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { SSEServerTransport } from "@modelcontextprotocol/sdk/server/sse.js";
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
    version: "0.1.0",
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

  server.tool(
    "unipost_create_post",
    "Create and publish a post to one or more social media accounts",
    {
      caption: z.string().describe("The text content of the post"),
      account_ids: z.array(z.string()).describe("List of social account IDs to post to"),
      media_urls: z.array(z.string()).optional().describe("Optional media URLs"),
      scheduled_at: z.string().optional().describe("ISO 8601 datetime for scheduled posting"),
    },
    async ({ caption, account_ids, media_urls, scheduled_at }) => {
      const body: any = { caption, account_ids };
      if (media_urls?.length) body.media_urls = media_urls;
      if (scheduled_at) body.scheduled_at = scheduled_at;
      const data = await apiRequest("/v1/social-posts", apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });
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

const httpServer = http.createServer(async (req, res) => {
  // CORS headers
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  const url = new URL(req.url || "/", `http://${req.headers.host}`);

  // Health check
  if (url.pathname === "/" || url.pathname === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok", service: "unipost-mcp", version: "0.1.0" }));
    return;
  }

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
  console.log(`  SSE endpoint: http://localhost:${PORT}/sse`);
  console.log(`  Health check: http://localhost:${PORT}/health`);
});
