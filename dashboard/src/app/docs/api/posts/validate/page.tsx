"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform_posts",
    type: "array",
    description: "Recommended unified request shape for per-account content.",
  },
  {
    name: "platform_posts[].account_id",
    type: "string",
    description: "Connected social account to validate against.",
  },
  {
    name: "platform_posts[].caption?",
    type: "string",
    description: "Caption or post text to validate.",
  },
  {
    name: "platform_posts[].media_urls?",
    type: "string[]",
    description: "Public asset URLs to validate for that destination.",
  },
  {
    name: "scheduled_at?",
    type: "string",
    description: "Optional scheduled publish time in ISO-8601 format.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "ok",
    type: "boolean",
    description: "Whether the request is safe to publish as-is.",
  },
  {
    name: "errors",
    type: "array",
    description: "Validation issues found while checking the payload.",
  },
  {
    name: "errors[].code",
    type: "string",
    description: "Machine-readable validation code.",
  },
  {
    name: "errors[].message",
    type: "string",
    description: "Human-readable validation message.",
  },
  {
    name: "errors[].fatal",
    type: "boolean",
    description: "Whether this issue should block publish.",
  },
];

const RESPONSE_401_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable auth error.",
  },
];

const RESPONSE_422_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "VALIDATION_FAILED".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Returned when the request body shape is invalid.",
  },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/social-posts/validate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_1",
        "caption": "Launch update for X"
      }
    ]
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const result = await client.posts.validate({
  platformPosts: [
    {
      accountId: "sa_twitter_1",
      caption: "Launch update for X",
    },
  ],
});`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

result = client.posts.validate(
  platform_posts=[
    {
      "account_id": "sa_twitter_1",
      "caption": "Launch update for X",
    }
  ]
)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  validation, err := client.Posts.Validate(context.Background(), &unipost.ValidatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_1",
        Caption:   "Launch update for X",
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = validation
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "ok": false,
    "errors": [
      {
        "code": "CAPTION_TOO_LONG",
        "message": "Caption exceeds the platform limit.",
        "fatal": true
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "platform_posts is required."
  }
}`,
  },
];

export default function ValidatePage() {
  return (
    <ApiReferencePage
      section="publishing"
      title="Validate"
      description="Runs preflight checks against the same payload shape as publish. Use it before automation or AI-driven posting so content problems are caught before quota is consumed."
    >
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiEndpointCard method="POST" path="/v1/social-posts/validate">
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#3b82f6", marginRight: 12 }}>POST</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>/v1/social-posts/validate</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method="POST" path="/v1/social-posts/validate">
              <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                <ApiFieldList items={AUTH_FIELDS} />
              </div>
              <div style={{ padding: "18px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Request Body</div>
                <ApiFieldList items={BODY_FIELDS} />
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method="POST" path="/v1/social-posts/validate">
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              <ApiAccordion title="200">
                <ApiFieldList items={RESPONSE_200_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="401">
                <ApiFieldList items={RESPONSE_401_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="422">
                <ApiFieldList items={RESPONSE_422_FIELDS} />
              </ApiAccordion>
            </ApiEndpointCard>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={SNIPPETS} />
            <CodeTabs snippets={RESPONSE_SNIPPETS} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}
