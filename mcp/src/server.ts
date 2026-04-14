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

type UploadMediaResult =
  | { error: string }
  | {
      media_id: string;
      content_type: string;
      size_bytes: number;
      media: any;
    };

async function uploadMediaToLibrary(
  apiKey: string,
  args: {
    filename: string;
    content_type: string;
    base64_data?: string;
    url?: string;
  }
): Promise<UploadMediaResult> {
  if (!args.base64_data && !args.url) {
    return { error: "Error: pass either base64_data or url." } as const;
  }

  let sizeBytes: number;
  let bytes: Uint8Array | null = null;
  if (args.base64_data) {
    bytes = Uint8Array.from(Buffer.from(args.base64_data, "base64"));
    sizeBytes = bytes.length;
  } else {
    const head = await fetch(args.url!, { method: "HEAD" });
    const lenHeader = head.headers.get("content-length");
    sizeBytes = lenHeader ? parseInt(lenHeader, 10) : 0;
    if (!sizeBytes) {
      return { error: "Error: could not determine size of remote URL via HEAD." };
    }
  }

  const created = await apiRequest("/v1/media", apiKey, {
    method: "POST",
    body: JSON.stringify({
      filename: args.filename,
      content_type: args.content_type,
      size_bytes: sizeBytes,
    }),
  });
  if (!created?.data?.upload_url) {
    return {
      error: "Error: API did not return an upload URL: " + JSON.stringify(created),
    };
  }

  let bodyToPut: BodyInit;
  if (bytes) {
    bodyToPut = new Blob([new Uint8Array(bytes)], { type: args.content_type });
  } else {
    const remote = await fetch(args.url!);
    bodyToPut = await remote.arrayBuffer();
  }

  const putRes = await fetch(created.data.upload_url, {
    method: "PUT",
    headers: { "Content-Type": args.content_type },
    body: bodyToPut,
  });
  if (!putRes.ok) {
    return {
      error: `Error: PUT to R2 failed (${putRes.status}): ${await putRes.text()}`,
    };
  }

  // Mirror the dashboard path: re-read the media row so the API can
  // HEAD the object, hydrate metadata, and flip status from pending
  // to uploaded before a later publish references media_ids.
  const hydrated = await apiRequest(`/v1/media/${created.data.id}`, apiKey);

  return {
    media_id: created.data.id as string,
    content_type: args.content_type,
    size_bytes: sizeBytes,
    media: hydrated?.data ?? null,
  };
}

// ── Create MCP server with tools (API key injected per-session) ──
function createMcpServer(apiKey: string): McpServer {
  const server = new McpServer({
    name: "unipost",
    version: "0.6.0",
    // Sprint 3 PR9: added unipost_create_connect_session,
    // unipost_reschedule_post, unipost_cancel_post.
    // Sprint 4 PR9: added unipost_bulk_create_posts,
    // unipost_list_managed_users. The single-post create tool also
    // gained the optional first_comment field on platform_posts[].
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
            media_ids: z
              .array(z.string())
              .optional()
              .describe("R2-uploaded media IDs from unipost_upload_media. Resolved server-side."),
            platform_options: z.record(z.string(), z.any()).optional(),
            in_reply_to: z.string().optional(),
            thread_position: z
              .number()
              .int()
              .optional()
              .describe(
                "1-indexed position in a multi-post thread. All entries with the same account_id and any non-zero thread_position form one thread. Twitter + Bluesky as of Sprint 3."
              ),
            first_comment: z
              .string()
              .optional()
              .describe(
                "Sprint 4: optional reply / comment posted immediately after the main post lands. Twitter (self-reply), LinkedIn (own-post comment), Instagram (first comment API). Bluesky and Threads reject this field — use thread_position instead."
              ),
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
          "Optional idempotency key. Same key + same workspace within 24h returns the prior response unchanged."
        ),
      status: z
        .enum(["draft"])
        .optional()
        .describe("Set to \"draft\" to persist without publishing. Use unipost_publish_draft to ship later."),
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
      if (args.status) body.status = args.status;

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
            media_ids: z.array(z.string()).optional(),
            platform_options: z.record(z.string(), z.any()).optional(),
            in_reply_to: z.string().optional(),
            thread_position: z.number().int().optional(),
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
    "List recent posts with optional filters and cursor pagination. Use cursor from a previous call's next_cursor field to walk through more than 25 results.",
    {
      status: z
        .string()
        .optional()
        .describe("Comma-separated list of statuses to include (draft, scheduled, publishing, published, partial, failed)."),
      from: z.string().optional().describe("RFC3339 lower bound on created_at (inclusive)."),
      to: z.string().optional().describe("RFC3339 upper bound on created_at (exclusive)."),
      limit: z.number().int().optional().describe("Page size, default 25, max 100."),
      cursor: z.string().optional().describe("Opaque cursor from a previous call's next_cursor."),
    },
    async (args) => {
      const params = new URLSearchParams();
      if (args.status) params.set("status", args.status);
      if (args.from) params.set("from", args.from);
      if (args.to) params.set("to", args.to);
      if (args.limit) params.set("limit", String(args.limit));
      if (args.cursor) params.set("cursor", args.cursor);
      const qs = params.toString();
      const data = await apiRequest("/v1/social-posts" + (qs ? "?" + qs : ""), apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    }
  );

  // ── Sprint 2 tools ──

  // unipost_upload_media accepts EITHER base64_data OR url:
  // - base64_data is for Claude Desktop / SDK callers that have a
  //   local file. The MCP server forwards the bytes to R2 via the
  //   API's two-step presign flow. Cap is ~4 MB after base64 inflation.
  // - url is for callers that already have the file hosted (Slack
  //   attachment, public URL, etc). The API fetches it server-side
  //   via the legacy media_urls path and stores it under a media_id.
  // base64_data takes precedence if both are set.
  server.tool(
    "unipost_upload_media",
    [
      "Upload an image or video to UniPost's media library and return a media_id.",
      "",
      "Pass EITHER base64_data (for local files, ≤ 4 MB after base64 inflation)",
      "OR url (for files already hosted publicly somewhere). base64_data wins",
      "if both are set.",
      "",
      "The returned media_id can be used in subsequent unipost_create_post or",
      "unipost_create_draft calls under platform_posts[].media_ids.",
    ].join("\n"),
    {
      filename: z.string().describe("Original filename (used to derive the storage extension)."),
      content_type: z.string().describe("MIME type, e.g. image/png, video/mp4."),
      base64_data: z
        .string()
        .optional()
        .describe("Base64-encoded file body. Required when url is not set."),
      url: z
        .string()
        .optional()
        .describe("Publicly fetchable URL the API can download from. Required when base64_data is not set."),
    },
    async (args) => {
      const uploaded = await uploadMediaToLibrary(apiKey, {
        filename: args.filename,
        content_type: args.content_type,
        base64_data: args.base64_data,
        url: args.url,
      });
      if ("error" in uploaded) {
        return {
          content: [
            { type: "text" as const, text: uploaded.error },
          ],
        };
      }

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(
              {
                media_id: uploaded.media_id,
                content_type: uploaded.content_type,
                size_bytes: uploaded.size_bytes,
                media: uploaded.media,
                note: "Reference this media_id under platform_posts[].media_ids in unipost_create_post or unipost_create_draft. The row has already been hydrated like the dashboard flow.",
              },
              null,
              2,
            ),
          },
        ],
      };
    },
  );

  server.tool(
    "unipost_get_media",
    "Get a media row by id. Use this to confirm status=uploaded before publishing or to inspect the hydrated media metadata after upload.",
    {
      media_id: z.string().describe("Media library id returned by unipost_upload_media."),
    },
    async ({ media_id }) => {
      const data = await apiRequest(`/v1/media/${media_id}`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_create_youtube_video_post",
    [
      "Create a YouTube video post with the friendliest current MCP workflow.",
      "",
      "Pass EITHER media_id, OR video_url, OR base64_data + filename + content_type.",
      "If video_url or base64_data is provided, UniPost uploads into the media library",
      "first, hydrates the media row, then publishes to YouTube with media_ids.",
      "",
      "This wraps the same upload-then-publish workflow the dashboard uses.",
      "For large local videos, prefer video_url or an already-uploaded media_id.",
      "Remote MCP servers are not ideal for moving large local binary files inline.",
    ].join("\n"),
    {
      account_id: z.string().describe("YouTube social account id."),
      caption: z.string().optional().describe("Caption or description context."),
      media_id: z.string().optional().describe("Existing UniPost media id for the uploaded video."),
      video_url: z.string().optional().describe("Publicly fetchable video URL to upload and publish."),
      filename: z.string().optional().describe("Required when base64_data is set."),
      content_type: z.string().optional().describe("Required when base64_data is set. Usually video/mp4."),
      base64_data: z.string().optional().describe("Base64-encoded video body for smaller local files."),
      privacy_status: z.enum(["private", "public", "unlisted"]).optional(),
      shorts: z.boolean().optional(),
      category_id: z.string().optional(),
      tags: z.array(z.string()).optional(),
      scheduled_at: z.string().optional(),
      idempotency_key: z.string().optional(),
    },
    async (args) => {
      let mediaId = args.media_id;

      if (!mediaId) {
        if (!args.video_url && !args.base64_data) {
          return {
            content: [
              {
                type: "text" as const,
                text: "Error: pass media_id, video_url, or base64_data + filename + content_type.",
              },
            ],
          };
        }

        if (args.base64_data && (!args.filename || !args.content_type)) {
          return {
            content: [
              {
                type: "text" as const,
                text: "Error: filename and content_type are required when base64_data is set.",
              },
            ],
          };
        }

        const uploaded = await uploadMediaToLibrary(apiKey, {
          filename: args.filename || "youtube-video.mp4",
          content_type: args.content_type || "video/mp4",
          base64_data: args.base64_data,
          url: args.video_url,
        });
        if ("error" in uploaded) {
          return {
            content: [{ type: "text" as const, text: uploaded.error }],
          };
        }
        mediaId = uploaded.media_id;
      }

      const body: any = {
        platform_posts: [
          {
            account_id: args.account_id,
            caption: args.caption ?? "",
            media_ids: [mediaId],
            platform_options: {
              youtube: {
                ...(args.privacy_status ? { privacy_status: args.privacy_status } : {}),
                ...(typeof args.shorts === "boolean" ? { shorts: args.shorts } : {}),
                ...(args.category_id ? { category_id: args.category_id } : {}),
                ...(args.tags?.length ? { tags: args.tags } : {}),
              },
            },
          },
        ],
      };

      if (args.scheduled_at) body.scheduled_at = args.scheduled_at;
      if (args.idempotency_key) body.idempotency_key = args.idempotency_key;

      const data = await apiRequest("/v1/social-posts", apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(
              {
                media_id: mediaId,
                post: data.data ?? null,
                note: "Used the media-library-first YouTube workflow. For very large local files, prefer video_url or upload once and reuse media_id.",
              },
              null,
              2,
            ),
          },
        ],
      };
    },
  );

  server.tool(
    "unipost_create_draft",
    "Create a draft post (status='draft'). Same body shape as unipost_create_post. Drafts persist without dispatching to platforms — useful for review / preview / approval workflows. Pair with unipost_publish_draft when you're ready to ship.",
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
            media_ids: z.array(z.string()).optional(),
            platform_options: z.record(z.string(), z.any()).optional(),
            in_reply_to: z.string().optional(),
            thread_position: z.number().int().optional(),
          })
        )
        .optional(),
      scheduled_at: z.string().optional(),
    },
    async (args) => {
      const body: any = { status: "draft" };
      if (args.platform_posts?.length) body.platform_posts = args.platform_posts;
      else if (args.account_ids?.length) {
        body.caption = args.caption ?? "";
        body.account_ids = args.account_ids;
        if (args.media_urls?.length) body.media_urls = args.media_urls;
      }
      if (args.scheduled_at) body.scheduled_at = args.scheduled_at;

      const data = await apiRequest("/v1/social-posts", apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_publish_draft",
    "Publish an existing draft. Atomically flips it to publishing and dispatches to all platforms — same publish path as unipost_create_post. Returns the post + per-platform results.",
    {
      draft_id: z.string().describe("ID of the draft to publish."),
      idempotency_key: z
        .string()
        .optional()
        .describe("Optional idempotency key for retry safety."),
    },
    async ({ draft_id, idempotency_key }) => {
      const body: any = {};
      if (idempotency_key) body.idempotency_key = idempotency_key;
      const data = await apiRequest(`/v1/social-posts/${draft_id}/publish`, apiKey, {
        method: "POST",
        body: JSON.stringify(body),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_get_account_health",
    "Return the health status of one connected social account: ok / degraded / disconnected, plus last successful post timestamp and most recent error if any. Derived from the account's last 10 publish results — no active probing.",
    {
      account_id: z.string().describe("Social account ID."),
    },
    async ({ account_id }) => {
      const data = await apiRequest(`/v1/social-accounts/${account_id}/health`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  // ── Sprint 3 v0.4.0 tools ──

  server.tool(
    "unipost_create_connect_session",
    [
      "Create a UniPost Connect session — a hosted OAuth link for onboarding an end user's social account.",
      "",
      "Returns {session_id, url, expires_at}. Email the URL to your end user; they",
      "complete the OAuth handshake (or Bluesky app-password form) on the hosted",
      "page, after which the new managed account appears in your workspace.",
      "",
      "Sprint 3 supports twitter, linkedin, bluesky. Sessions expire after 30 minutes.",
    ].join("\n"),
    {
      platform: z.enum(["twitter", "linkedin", "bluesky", "instagram", "threads"]).describe("Target social platform. Instagram and Threads require CONNECT_INSTAGRAM_ENABLED / CONNECT_THREADS_ENABLED on the server."),
      external_user_id: z.string().describe("Your stable identifier for the end user — used to look up the resulting account later."),
      external_user_email: z.string().optional().describe("Optional email for record-keeping; not used by UniPost beyond storage."),
      return_url: z.string().optional().describe("Where to redirect the user after they complete the flow. Required for embedded apps."),
    },
    async (args) => {
      const data = await apiRequest("/v1/connect/sessions", apiKey, {
        method: "POST",
        body: JSON.stringify(args),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_reschedule_post",
    "Move a scheduled post to a new scheduled_at timestamp. Only works on status='scheduled' posts; drafts use unipost_update_draft instead, and published posts cannot be rescheduled.",
    {
      post_id: z.string().describe("ID of the scheduled post."),
      scheduled_at: z.string().describe("New RFC3339 timestamp; must be at least 60 seconds in the future."),
    },
    async ({ post_id, scheduled_at }) => {
      const data = await apiRequest(`/v1/social-posts/${post_id}`, apiKey, {
        method: "PATCH",
        body: JSON.stringify({ scheduled_at }),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_cancel_post",
    "Cancel a draft or scheduled post. Cancelled posts are skipped by the scheduler; cancellation cannot be undone. Returns 409 for posts that are already publishing or published.",
    {
      post_id: z.string().describe("ID of the draft or scheduled post."),
    },
    async ({ post_id }) => {
      const data = await apiRequest(`/v1/social-posts/${post_id}/cancel`, apiKey, {
        method: "POST",
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  // ── Sprint 4 v0.5.0 tools ──

  server.tool(
    "unipost_bulk_create_posts",
    [
      "Publish up to 50 social posts in a single call. Returns one result entry per input post (success or per-post error).",
      "",
      "Each post in the `posts` array is the same shape as a single unipost_create_post call: pass platform_posts[] with",
      "per-account captions, or use the legacy caption + account_ids fan-out shape. Drafts and scheduled posts are NOT supported in bulk —",
      "use unipost_create_post for those.",
      "",
      "Per-post idempotency_key still works; re-sending the same batch with the same keys safely retries failed posts.",
      "Quota counts each post individually. Mid-batch quota exhaustion fails only the remaining posts, not the whole batch.",
    ].join("\n"),
    {
      posts: z
        .array(z.record(z.string(), z.unknown()))
        .max(50)
        .describe("Array of post bodies (1-50). Each entry is a complete /v1/social-posts request body."),
    },
    async ({ posts }) => {
      const data = await apiRequest("/v1/social-posts/bulk", apiKey, {
        method: "POST",
        body: JSON.stringify({ posts }),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_list_managed_users",
    "List end users onboarded via UniPost Connect, grouped by external_user_id with per-platform account counts. Use this when an AI agent needs a workspace-level view of all managed users (e.g. 'who has a Twitter account connected via Connect?').",
    {
      limit: z.number().int().min(1).max(100).optional().describe("Page size, default 25, max 100."),
    },
    async ({ limit }) => {
      const qs = limit ? `?limit=${limit}` : "";
      const data = await apiRequest(`/v1/users${qs}`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_get_managed_user",
    "Get the detail view of one managed end user: every social account they've connected, with per-account status and platform info. Pair with unipost_list_managed_users to walk a workspace's end-user list.",
    {
      external_user_id: z.string().describe("The external_user_id you used when creating the Connect session."),
    },
    async ({ external_user_id }) => {
      const data = await apiRequest(`/v1/users/${encodeURIComponent(external_user_id)}`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
  );

  // ── Sprint 5 v0.6.0 tools ──

  server.tool(
    "unipost_get_analytics_rollup",
    [
      "Dimensional analytics rollup — aggregate publish metrics over a",
      "date range with configurable granularity (day/week/month) and",
      "GROUP BY dimensions (platform, social_account_id, external_user_id,",
      "status). Returns time-bucketed rows with total + succeeded + failed",
      "counts. Max range 366 days.",
      "",
      "Example: get daily publish counts by platform for the last 30 days:",
      "  { from: '2026-03-08T00:00:00Z', to: '2026-04-08T00:00:00Z',",
      "    granularity: 'day', group_by: 'platform' }",
    ].join("\n"),
    {
      from: z.string().describe("RFC3339 lower bound (inclusive)."),
      to: z.string().describe("RFC3339 upper bound (exclusive)."),
      granularity: z.enum(["day", "week", "month"]).optional().describe("Time bucket size (default: day)."),
      group_by: z.string().optional().describe("Comma-separated dimensions: platform, social_account_id, external_user_id, status."),
    },
    async (args) => {
      const params = new URLSearchParams();
      params.set("from", args.from);
      params.set("to", args.to);
      if (args.granularity) params.set("granularity", args.granularity);
      if (args.group_by) params.set("group_by", args.group_by);
      const data = await apiRequest(`/v1/analytics/rollup?${params.toString()}`, apiKey);
      return { content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }] };
    },
  );

  server.tool(
    "unipost_update_workspace_quota",
    [
      "Set or clear the per-social-account monthly publish cap for the",
      "current workspace. When set, the publish path counts each account's",
      "successful posts this calendar month and refuses dispatch when",
      "the count reaches the cap. Pass null to disable the cap (the",
      "default — unlimited per-account, only the workspace-wide quota",
      "applies). Pass 0 to emergency-lock a runaway account for the",
      "rest of the month.",
      "",
      "Upper bound: 1,000,000. Negative values are rejected.",
    ].join("\n"),
    {
      per_account_monthly_limit: z
        .number()
        .int()
        .min(0)
        .max(1_000_000)
        .nullable()
        .describe("Monthly publish cap per social account. null = unlimited (default)."),
    },
    async ({ per_account_monthly_limit }) => {
      // PATCH /v1/workspace uses the workspace_id from the API key context.
      const data = await apiRequest("/v1/workspace", apiKey, {
        method: "PATCH",
        body: JSON.stringify({ per_account_monthly_limit }),
      });
      return { content: [{ type: "text" as const, text: JSON.stringify(data.data, null, 2) }] };
    },
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
    res.end(JSON.stringify({ status: "ok", service: "unipost-mcp", version: "0.6.0" }));
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
