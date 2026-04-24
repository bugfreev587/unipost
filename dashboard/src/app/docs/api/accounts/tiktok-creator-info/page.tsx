"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected TikTok account ID such as sa_tiktok_123." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "creator_avatar_url", type: "string", description: "Avatar URL returned by TikTok." },
  { name: "creator_username", type: "string", description: "TikTok username." },
  { name: "creator_nickname", type: "string", description: "TikTok display name." },
  { name: "privacy_level_options", type: "string[]", description: "Allowed publish privacy settings for the account." },
  { name: "comment_disabled", type: "boolean", description: "Whether the creator disabled comments in TikTok settings." },
  { name: "duet_disabled", type: "boolean", description: "Whether duet is disabled." },
  { name: "stitch_disabled", type: "boolean", description: "Whether stitch is disabled." },
  { name: "max_video_post_duration_sec", type: "number", description: "Maximum video length TikTok currently allows for this account." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED", "NOT_FOUND", "WRONG_PLATFORM", "NEEDS_RECONNECT", and "TIKTOK_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "wrong_platform" or "needs_reconnect".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts/sa_tiktok_123/tiktok/creator-info" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const info = await client.accounts.tiktokCreatorInfo("sa_tiktok_123");
console.log(info.privacy_level_options);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "creator_avatar_url": "https://p16-sign-va.tiktokcdn.com/avatar.jpg",
    "creator_username": "studioalex",
    "creator_nickname": "Studio Alex",
    "privacy_level_options": ["PUBLIC_TO_EVERYONE", "MUTUAL_FOLLOW_FRIENDS"],
    "comment_disabled": false,
    "duet_disabled": false,
    "stitch_disabled": true,
    "max_video_post_duration_sec": 600
  }
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "NEEDS_RECONNECT",
    "normalized_code": "needs_reconnect",
    "message": "Your TikTok connection has expired. Please reconnect the account."
  },
  "request_id": "req_123"
}`,
  },
];

export default function TikTokCreatorInfoPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get TikTok creator info"
      description="Returns the TikTok creator_info payload for one connected TikTok account. Use it to populate privacy options, interaction toggles, and upload-length validation before publish."
      method="GET"
      path="/v1/social-accounts/:account_id/tiktok/creator-info"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
