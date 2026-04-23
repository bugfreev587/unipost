"use client";

import {
  ApiReferencePage,
  ApiEndpointCard,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../_components/doc-components";

const HEADER_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "Required",
    description: "Send your UniPost API key on every request.",
  },
];

const KEY_FIELDS: ApiFieldItem[] = [
  {
    name: "up_live_",
    type: "production",
    description: "Real publishing and production traffic.",
  },
  {
    name: "up_test_",
    type: "test",
    description: "Development, staging, and non-production work.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "401 UNAUTHORIZED",
    type: "auth",
    description: "The API key is missing, invalid, or revoked.",
  },
  {
    name: "403 FORBIDDEN",
    type: "policy",
    description: "The key is valid but the workspace or plan blocks the action.",
  },
  {
    name: "429 RATE_LIMITED",
    type: "limits",
    description: "The key exceeded request limits for the current time window.",
  },
];

const AUTH_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: accounts } = await client.accounts.list();`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])
accounts = client.accounts.list()`,
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

  accounts, err := client.Accounts.List(context.Background(), nil)
  if err != nil {
    log.Fatal(err)
  }

  _ = accounts
}`,
  },
];

export default function AuthenticationPage() {
  return (
    <ApiReferencePage
      section="core"
      title="Authentication"
      description="Every public UniPost API request uses Bearer authentication with your API key. If the key is missing or invalid, the request fails before any business logic runs."
    >
      <div style={{ display: "grid", gap: 18 }}>
        <ApiEndpointCard method="GET" path="authorization">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Header</div>
            <ApiFieldList items={HEADER_FIELDS} />
          </div>
        </ApiEndpointCard>

        <ApiEndpointCard method="GET" path="authorization">
          <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Key types</div>
            <ApiFieldList items={KEY_FIELDS} />
          </div>
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Common failures</div>
            <ApiFieldList items={ERROR_FIELDS} />
          </div>
        </ApiEndpointCard>

        <CodeTabs snippets={AUTH_SNIPPETS} />
      </div>
    </ApiReferencePage>
  );
}
