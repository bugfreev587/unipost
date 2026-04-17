"use client";

// Language-tabbed code block for the post_with_api tutorial.
//
// Mirrors the Resend-style "run this to send your first post" widget:
// tabs for curl / Node.js / Python / Go, each rendering a snippet
// templated with the user's newly-created API key and one of their
// connected account IDs. Includes a copy button; the live "Send post"
// button is a separate component (send-button.tsx) since the code
// block is static while the send result has its own state machine.

import { useState, useMemo } from "react";
import { Copy, Check } from "lucide-react";

type Language = "curl" | "node" | "python" | "go";

const LANGUAGES: Array<{ id: Language; label: string }> = [
  { id: "curl", label: "cURL" },
  { id: "node", label: "Node.js" },
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
];

export function CodeBlock({
  apiBase,
  apiKey,
  accountId,
  caption,
}: {
  apiBase: string;
  apiKey: string;
  accountId: string;
  caption: string;
}) {
  const [lang, setLang] = useState<Language>("curl");
  const [copied, setCopied] = useState(false);

  const snippet = useMemo(
    () => buildSnippet(lang, { apiBase, apiKey, accountId, caption }),
    [lang, apiBase, apiKey, accountId, caption],
  );

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(snippet);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* clipboard unavailable — silent */ }
  }

  return (
    <div style={{
      width: "100%",
      minWidth: 0,
      boxSizing: "border-box",
      border: "1px solid var(--dborder)",
      borderRadius: 10,
      overflow: "hidden",
      background: "#0a0a0c",
    }}>
      {/* Tabs */}
      <div style={{
        display: "flex", alignItems: "center", justifyContent: "space-between",
        width: "100%",
        minWidth: 0,
        borderBottom: "1px solid var(--dborder)",
        padding: "0 8px",
      }}>
        <div style={{ display: "flex", minWidth: 0 }}>
          {LANGUAGES.map((l) => {
            const active = l.id === lang;
            return (
              <button
                key={l.id}
                type="button"
                onClick={() => setLang(l.id)}
                className="dt-body-sm"
                style={{
                  padding: "10px 14px",
                  border: "none",
                  background: "transparent",
                  color: active ? "var(--dtext)" : "var(--dmuted)",
                  borderBottom: active ? "2px solid var(--daccent)" : "2px solid transparent",
                  marginBottom: -1,
                  cursor: "pointer",
                  fontFamily: "inherit",
                  fontWeight: active ? 600 : 400,
                }}
              >
                {l.label}
              </button>
            );
          })}
        </div>
        <button
          type="button"
          onClick={handleCopy}
          className="dt-body-sm"
          style={{
            display: "inline-flex", alignItems: "center", gap: 6,
            padding: "6px 10px", borderRadius: 6,
            border: "1px solid var(--dborder)",
            background: "transparent",
            color: copied ? "var(--daccent)" : "var(--dmuted)",
            cursor: "pointer", fontFamily: "inherit", fontSize: 12,
          }}
        >
          {copied ? <Check style={{ width: 12, height: 12 }} /> : <Copy style={{ width: 12, height: 12 }} />}
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      {/* Snippet */}
      <pre style={{
        width: "100%",
        minWidth: 0,
        boxSizing: "border-box",
        margin: 0, padding: 14,
        fontFamily: "var(--font-geist-mono), ui-monospace, monospace",
        fontSize: 12, lineHeight: 1.55,
        color: "#d6d6de",
        overflowX: "auto",
        whiteSpace: "pre",
      }}>{snippet}</pre>
    </div>
  );
}

function buildSnippet(
  lang: Language,
  { apiBase, apiKey, accountId, caption }: { apiBase: string; apiKey: string; accountId: string; caption: string },
): string {
  switch (lang) {
    case "curl":
      return `curl -X POST "${apiBase}/v1/social-posts" \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "${escapeShell(caption)}",
    "account_ids": ["${accountId}"]
  }'`;

    case "node":
      return `import { UniPost } from "@unipost/sdk";

const client = new UniPost({ apiKey: "${apiKey}" });

const post = await client.posts.create({
  caption: ${JSON.stringify(caption)},
  accountIds: ["${accountId}"],
});

console.log("Published:", post.id);`;

    case "python":
      return `from unipost import UniPost

client = UniPost(api_key="${apiKey}")

post = client.posts.create(
    caption=${JSON.stringify(caption)},
    account_ids=["${accountId}"],
)

print("Published:", post.id)`;

    case "go":
      return `package main

import (
\t"context"
\t"fmt"

\t"github.com/unipost-dev/sdk-go"
)

func main() {
\tclient := unipost.NewClient("${apiKey}")
\tpost, err := client.Posts.Create(context.Background(), &unipost.PostCreateParams{
\t\tCaption:    ${JSON.stringify(caption)},
\t\tAccountIDs: []string{"${accountId}"},
\t})
\tif err != nil {
\t\tpanic(err)
\t}
\tfmt.Println("Published:", post.ID)
}`;
  }
}

function escapeShell(s: string): string {
  // Escape single quotes for inclusion in a '...' shell string.
  return s.replace(/'/g, "'\\''");
}
