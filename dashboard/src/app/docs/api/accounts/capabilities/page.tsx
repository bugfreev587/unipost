"use client";

import { filterDocsNavigation } from "@/lib/docs-feature-flags";
import { usePublicDocsFeatureFlags } from "@/lib/use-public-docs-feature-flags";
import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected account ID such as sa_x_123." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "schema_version", type: "string", description: "Capability schema version." },
  { name: "account_id", type: "string", description: "Resolved account ID." },
  { name: "platform", type: "string", description: "Platform name for this account." },
  { name: "capability.display_name", type: "string", description: "Human-friendly platform name." },
  { name: "capability.text", type: "object", description: "Caption length and threading rules." },
  { name: "capability.media", type: "object", description: "Image/video limits and file format rules." },
  { name: "capability.thread", type: "object", description: "Whether reply-chain style threading is supported." },
  { name: "capability.scheduling", type: "object", description: "Whether UniPost can schedule posts for this platform." },
  { name: "capability.first_comment", type: "object", description: "Whether first comments are supported and any extra limits." },
  { name: "x_inbox.comments_enabled", type: "boolean", description: "Whether this X account can receive and answer eligible public replies." },
  { name: "x_inbox.dms_enabled", type: "boolean", description: "Whether the workspace rollout and account scopes permit bounded legacy X DM lookup/send. Private real-time subscription delivery is not currently provisioned." },
  { name: "x_inbox.missing_scopes", type: "string[]", description: "Permissions that require OAuth reconnect for currently available features. DM-only missing scopes are excluded while X DMs are unavailable." },
  { name: "x_inbox.reconnect_required", type: "boolean", description: "Whether the account must repeat X OAuth." },
  { name: "x_inbox.delivery_status", type: "string", description: "pending, active, paused_cap, paused_allowance, paused_plan, or error." },
  { name: "x_inbox.app_mode", type: "string", description: "unipost_managed_app, workspace_x_app, or legacy_unknown." },
  { name: "x_inbox.missing_app_credentials", type: "string[]", description: "For workspace_x_app, any missing client_id, client_secret, app_bearer_token, or consumer_secret." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED" and "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_x_123/capabilities" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const caps = await client.accounts.capabilities("sa_instagram_123");
console.log(caps.capability.media.images.max_count);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

caps = client.accounts.capabilities("sa_instagram_123")
print(caps["data"]["capability"]["text"]["max_length"])`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "fmt"
  "log"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

  caps, err := client.Accounts.Capabilities(context.Background(), "sa_instagram_123")
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(caps["capability"])
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var caps = client.accounts().capabilities("sa_instagram_123");
System.out.println(caps.get("capability").get("text").get("max_length").asInt());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "schema_version": "1.7",
    "account_id": "sa_x_123",
    "platform": "twitter",
    "capability": {
      "display_name": "X / Twitter",
      "text": {
        "max_length": 280,
        "min_length": 0,
        "required": false,
        "supports_threads": false
      },
      "media": {
        "requires_media": false,
        "allow_mixed": false
      },
      "thread": { "supported": false },
      "scheduling": { "supported": true },
      "first_comment": { "supported": true, "max_length": 280 }
    },
    "x_inbox": {
      "comments_enabled": true,
      "dms_enabled": false,
      "missing_scopes": ["dm.read", "dm.write"],
      "reconnect_required": true,
      "delivery_status": "pending",
      "app_mode": "unipost_managed_app",
      "missing_app_credentials": []
    }
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Account not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function AccountCapabilitiesPage() {
  const publicFeatureFlags = usePublicDocsFeatureFlags();
  const xDMsEnabled = publicFeatureFlags.x_dms_v1;
  const responseFields = RESPONSE_200_FIELDS.filter((field) => (
    xDMsEnabled
    || !["x_inbox.dms_enabled", "x_inbox.missing_scopes"].includes(field.name)
  ));
  const responseSnippets = xDMsEnabled
    ? RESPONSE_SNIPPETS
    : [
        {
          lang: "json",
          label: "200",
          code: `{
  "data": {
    "schema_version": "1.7",
    "account_id": "sa_x_123",
    "platform": "twitter",
    "x_inbox": {
      "comments_enabled": true,
      "reconnect_required": false,
      "delivery_status": "active",
      "app_mode": "unipost_managed_app",
      "missing_app_credentials": []
    }
  }
}`,
        },
        RESPONSE_SNIPPETS[1],
      ];
  const guideLinks = filterDocsNavigation([
    { label: "Reconnect X Inbox permissions", href: "/docs/guides/x/reconnect-permissions" },
    { label: "Receive X comments", href: "/docs/guides/x/comments" },
    { label: "Receive X direct messages", href: "/docs/guides/x/direct-messages" },
  ], publicFeatureFlags);

  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get account capabilities"
      description="Returns the publishing capability map for the platform behind one connected account. Use it to drive client-side validation or UI affordances before you call create post."
      method="GET"
      path="/v1/accounts/:account_id/capabilities"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: responseFields },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={responseSnippets}
      guideLinks={guideLinks}
    />
  );
}
