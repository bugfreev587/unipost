"use client";

// Language-tabbed code block for the post_with_api tutorial.
//
// Visual parity with the docs API reference (e.g.
// /docs/api/profiles/list): rounded card + tabs header + inset code
// surface, fully theme-aware in light/dark. We don't pull in the
// docs' Monaco-backed component because it brings a multi-MB bundle
// for what's a small, read-only snippet inside a modal — instead we
// match the docs styling using project-wide theme tokens
// (--surface, --surface-inset, --text, --primary, etc.) so light
// theme renders dark text on a light surface and dark theme renders
// light text on a dark surface, instead of the previous hardcoded
// dark code block that was unreadable on light theme.

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
  requestBody,
}: {
  apiBase: string;
  apiKey: string;
  requestBody: object;
}) {
  const [lang, setLang] = useState<Language>("curl");
  const [copied, setCopied] = useState(false);

  const snippet = useMemo(
    () => buildSnippet(lang, { apiBase, apiKey, requestBody }),
    [lang, apiBase, apiKey, requestBody],
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
      border: "1px solid var(--border-soft)",
      borderRadius: 14,
      overflow: "hidden",
      background: "var(--surface)",
      boxShadow: "var(--marketing-shadow-soft, 0 1px 2px rgba(15, 23, 42, 0.04))",
    }}>
      {/* Tabs + copy header */}
      <div style={{
        display: "flex", alignItems: "center", justifyContent: "space-between",
        gap: 12,
        width: "100%",
        minWidth: 0,
        padding: "10px 12px",
        background: "var(--surface)",
      }}>
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap", minWidth: 0 }}>
          {LANGUAGES.map((l) => {
            const active = l.id === lang;
            return (
              <button
                key={l.id}
                type="button"
                onClick={() => setLang(l.id)}
                className="dt-body-sm"
                style={{
                  padding: "6px 11px",
                  borderRadius: 8,
                  border: "1px solid",
                  borderColor: active
                    ? "color-mix(in srgb, var(--primary) 32%, var(--border-soft))"
                    : "var(--border-soft)",
                  background: active
                    ? "color-mix(in srgb, var(--primary) 10%, var(--surface))"
                    : "var(--surface)",
                  color: active ? "var(--primary)" : "var(--text-muted)",
                  fontFamily: "var(--font-fira-code), var(--font-geist-mono), monospace",
                  fontSize: 12,
                  fontWeight: active ? 600 : 500,
                  cursor: "pointer",
                  transition: "all 0.12s",
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
          aria-label="Copy code to clipboard"
          style={{
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
            width: 30,
            height: 30,
            borderRadius: 8,
            border: "1px solid var(--border-soft)",
            background: "var(--surface)",
            color: copied ? "var(--primary)" : "var(--text-muted)",
            cursor: "pointer",
            transition: "all 0.12s",
          }}
        >
          {copied ? <Check style={{ width: 14, height: 14 }} /> : <Copy style={{ width: 14, height: 14 }} />}
        </button>
      </div>
      {/* Code surface */}
      <div style={{
        margin: "0 12px 12px",
        padding: "14px 16px",
        background: "var(--surface-inset)",
        borderRadius: 10,
        overflowX: "auto",
      }}>
        <pre style={{
          margin: 0,
          fontFamily: "var(--font-fira-code), var(--font-geist-mono), monospace",
          fontSize: 12.5,
          lineHeight: 1.7,
          color: "var(--text)",
          whiteSpace: "pre",
        }}>{snippet}</pre>
      </div>
    </div>
  );
}

function buildSnippet(
  lang: Language,
  {
    apiBase,
    apiKey,
    requestBody,
  }: {
    apiBase: string;
    apiKey: string;
    requestBody: object;
  },
): string {
  const requestBodyJson = JSON.stringify(requestBody, null, 2);
  const indentedNodeBody = indent(requestBodyJson, 2);
  const indentedPythonBody = indent(requestBodyJson, 4);
  const indentedGoBody = indent(requestBodyJson, 1);

  switch (lang) {
    case "curl":
      return `curl -X POST "${apiBase}/v1/posts" \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '${escapeShell(requestBodyJson)}'`;

    case "node":
      return `const payload = ${indentedNodeBody};

const response = await fetch("${apiBase}/v1/posts", {
  method: "POST",
  headers: {
    "Authorization": "Bearer ${apiKey}",
    "Content-Type": "application/json",
  },
  body: JSON.stringify(payload),
});

if (!response.ok) {
  throw new Error(await response.text());
}

const post = await response.json();
console.log("Published:", post.data?.id ?? post.id);`;

    case "python":
      return `import json
import requests

payload = json.loads(r'''${indentedPythonBody}''')

response = requests.post(
    "${apiBase}/v1/posts",
    headers={
        "Authorization": "Bearer ${apiKey}",
        "Content-Type": "application/json",
    },
    json=payload,
)
response.raise_for_status()

post = response.json()
print("Published:", post.get("data", {}).get("id") or post.get("id"))`;

    case "go":
      return `package main

import (
\t"bytes"
\t"context"
\t"fmt"
\t"io"
\t"net/http"
)

func main() {
\tbody := []byte(\`${indentedGoBody}\`)
\treq, err := http.NewRequestWithContext(
\t\tcontext.Background(),
\t\thttp.MethodPost,
\t\t"${apiBase}/v1/posts",
\t\tbytes.NewReader(body),
\t)
\tif err != nil {
\t\tpanic(err)
\t}
\treq.Header.Set("Authorization", "Bearer ${apiKey}")
\treq.Header.Set("Content-Type", "application/json")

\tresp, err := http.DefaultClient.Do(req)
\tif err != nil {
\t\tpanic(err)
\t}
\tdefer resp.Body.Close()

\tresponseBody, _ := io.ReadAll(resp.Body)
\tif resp.StatusCode < 200 || resp.StatusCode >= 300 {
\t\tpanic(string(responseBody))
\t}
\tfmt.Println("Published:", string(responseBody))
}`;
  }
}

function indent(s: string, level: number): string {
  const prefix = "\t".repeat(level);
  return s
    .split("\n")
    .map((line, index) => (index === 0 ? line : `${prefix}${line}`))
    .join("\n");
}

function escapeShell(s: string): string {
  // Escape single quotes for inclusion in a '...' shell string.
  return s.replace(/'/g, "'\\''");
}
