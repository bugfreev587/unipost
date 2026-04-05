#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const mcp_js_1 = require("@modelcontextprotocol/sdk/server/mcp.js");
const stdio_js_1 = require("@modelcontextprotocol/sdk/server/stdio.js");
const zod_1 = require("zod");
const API_URL = process.env.UNIPOST_API_URL || "https://api.unipost.dev";
const API_KEY = process.env.UNIPOST_API_KEY || "";
async function apiRequest(path, options) {
    const res = await fetch(`${API_URL}${path}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${API_KEY}`,
            ...options?.headers,
        },
    });
    return res.json();
}
const server = new mcp_js_1.McpServer({
    name: "unipost",
    version: "0.1.0",
});
// Tool: List connected social accounts
server.tool("unipost_list_accounts", "List all connected social media accounts", {
    platform: zod_1.z
        .string()
        .optional()
        .describe("Filter by platform (bluesky, linkedin, instagram, threads, tiktok, youtube, twitter)"),
}, async ({ platform }) => {
    const data = await apiRequest("/v1/social-accounts");
    let accounts = data.data || [];
    if (platform) {
        accounts = accounts.filter((a) => a.platform === platform);
    }
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(accounts, null, 2),
            },
        ],
    };
});
// Tool: Create and publish a post
server.tool("unipost_create_post", "Create and publish a post to one or more social media accounts", {
    caption: zod_1.z.string().describe("The text content of the post"),
    account_ids: zod_1.z
        .array(zod_1.z.string())
        .describe("List of social account IDs to post to"),
    media_urls: zod_1.z
        .array(zod_1.z.string())
        .optional()
        .describe("Optional media URLs (images for Instagram, videos for TikTok/YouTube)"),
    scheduled_at: zod_1.z
        .string()
        .optional()
        .describe("ISO 8601 datetime for scheduled posting (optional)"),
}, async ({ caption, account_ids, media_urls, scheduled_at }) => {
    const body = { caption, account_ids };
    if (media_urls?.length)
        body.media_urls = media_urls;
    if (scheduled_at)
        body.scheduled_at = scheduled_at;
    const data = await apiRequest("/v1/social-posts", {
        method: "POST",
        body: JSON.stringify(body),
    });
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(data.data, null, 2),
            },
        ],
    };
});
// Tool: Get post details and status
server.tool("unipost_get_post", "Get the status and details of a published post", {
    post_id: zod_1.z.string().describe("The post ID"),
}, async ({ post_id }) => {
    const data = await apiRequest(`/v1/social-posts/${post_id}`);
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(data.data, null, 2),
            },
        ],
    };
});
// Tool: Get post analytics
server.tool("unipost_get_analytics", "Get engagement metrics (views, likes, comments, shares) for a published post", {
    post_id: zod_1.z.string().describe("The post ID"),
}, async ({ post_id }) => {
    const data = await apiRequest(`/v1/social-posts/${post_id}/analytics`);
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(data.data, null, 2),
            },
        ],
    };
});
// Tool: List posts
server.tool("unipost_list_posts", "List recent posts with their status", {
    status: zod_1.z
        .string()
        .optional()
        .describe("Filter by status: scheduled, published, failed"),
    limit: zod_1.z.number().optional().describe("Maximum number of posts to return"),
}, async ({ status }) => {
    let path = "/v1/social-posts";
    if (status)
        path += `?status=${status}`;
    const data = await apiRequest(path);
    return {
        content: [
            {
                type: "text",
                text: JSON.stringify(data.data, null, 2),
            },
        ],
    };
});
// Start server
async function main() {
    if (!API_KEY) {
        console.error("UNIPOST_API_KEY is required. Set it in your environment or MCP config.");
        process.exit(1);
    }
    const transport = new stdio_js_1.StdioServerTransport();
    await server.connect(transport);
}
main().catch(console.error);
