"use client";
import { useMemo, useState } from "react";
import { ChevronDown, Loader2 } from "lucide-react";
import { CodeBlock, codeBlockStyles } from "../../../_components/code-block";
import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../../_components/doc-components";

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Publishable UniPost account ID.",
  },
  {
    name: "platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "account_name",
    type: "string | null",
    description: "Handle or display name.",
  },
  {
    name: "status",
    type: "string",
    description: 'Connection state such as "active" or "reconnect_required".',
  },
  {
    name: "connection_type",
    type: "string",
    description: '"byo" or "managed".',
  },
  {
    name: "connected_at",
    type: "string",
    description: "Connection timestamp.",
  },
  {
    name: "external_user_id",
    type: "string | null",
    description: "Your Connect user ID, if present.",
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

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts?platform=instagram" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: accounts } = await client.accounts.list({
  platform: "instagram",
});

console.log(accounts);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

accounts = client.accounts.list(platform="instagram")
print(accounts["data"])`,
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

  accounts, err := client.Accounts.List(context.Background(), &unipost.ListAccountsParams{
    Platform: "instagram",
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = accounts
}`,
  },
];

const RESPONSE_200 = `{
  "data": [
    {
      "id": "sa_instagram_123",
      "platform": "instagram",
      "account_name": "studio.alex",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "byo",
      "connected_at": "2026-04-02T10:00:00Z",
      "external_user_id": null
    }
  ]
}`;

const RESPONSE_401 = `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
}`;

const RESPONSE_TABS = [
  { code: "200", body: RESPONSE_200, fields: RESPONSE_200_FIELDS },
  { code: "401", body: RESPONSE_401, fields: RESPONSE_401_FIELDS },
];

const RESPONSE_SNIPPETS = RESPONSE_TABS.map((tab) => ({
  lang: "json",
  label: tab.code,
  code: tab.body,
}));

function TryItSection({
  title,
  defaultOpen = false,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  return (
    <details open={defaultOpen} className="accounts-try-section">
      <summary
        style={{
          listStyle: "none",
          cursor: "pointer",
          padding: "16px 18px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          fontSize: 15,
          fontWeight: 700,
          color: "var(--docs-text)",
        }}
      >
        <span>{title}</span>
        <ChevronDown className="accounts-try-chevron" style={{ width: 18, height: 18, color: "var(--docs-text-muted)" }} />
      </summary>
      <div style={{ padding: "0 18px 18px" }}>{children}</div>
    </details>
  );
}

function TryItField({
  label,
  type,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  type: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <div style={{ display: "grid", gap: 8 }}>
      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", gap: 12 }}>
        <label style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "var(--docs-text)" }}>{label}</label>
        <span style={{ fontFamily: "var(--docs-mono)", fontSize: 13, color: "var(--docs-text-faint)" }}>{type}</span>
      </div>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        style={{
          width: "100%",
          border: "1px solid var(--docs-border)",
          borderRadius: 12,
          background: "var(--docs-bg-muted)",
          color: "var(--docs-text)",
          fontSize: 15,
          lineHeight: 1.4,
          padding: "14px 16px",
          outline: "none",
        }}
      />
    </div>
  );
}

export function ListAccountsContent() {
  const [apiKey, setApiKey] = useState("");
  const [platform, setPlatform] = useState("");
  const [externalUserId, setExternalUserId] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [liveStatus, setLiveStatus] = useState<number | null>(null);
  const [liveResponse, setLiveResponse] = useState<string>(RESPONSE_200);
  const [liveError, setLiveError] = useState<string>("");

  const requestPath = useMemo(() => {
    const params = new URLSearchParams();
    if (platform.trim()) params.set("platform", platform.trim());
    if (externalUserId.trim()) params.set("external_user_id", externalUserId.trim());
    const query = params.toString();
    return query ? `/v1/social-accounts?${query}` : "/v1/social-accounts";
  }, [externalUserId, platform]);

  const authHeader = useMemo(() => {
    const trimmed = apiKey.trim();
    if (!trimmed) return "";
    return /^Bearer\s+/i.test(trimmed) ? trimmed : `Bearer ${trimmed}`;
  }, [apiKey]);

  async function handleSend() {
    setIsSending(true);
    setLiveError("");

    try {
      const response = await fetch(`https://api.unipost.dev${requestPath}`, {
        headers: authHeader
          ? {
              Authorization: authHeader,
            }
          : {},
      });

      const text = await response.text();
      setLiveStatus(response.status);

      try {
        const parsed = JSON.parse(text);
        setLiveResponse(JSON.stringify(parsed, null, 2));
      } catch {
        setLiveResponse(text || "{}");
      }
    } catch (error) {
      setLiveStatus(null);
      setLiveError(error instanceof Error ? error.message : "Request failed");
      setLiveResponse(`{\n  "error": {\n    "message": "Request failed."\n  }\n}`);
    } finally {
      setIsSending(false);
    }
  }

  return (
    <>
      <style
        dangerouslySetInnerHTML={{
          __html: `
            .accounts-try-section > summary::-webkit-details-marker{display:none}
            .accounts-try-section + .accounts-try-section{border-top:1px solid var(--docs-border)}
            .accounts-try-section .accounts-try-chevron{transition:transform .18s ease}
            .accounts-try-section[open] .accounts-try-chevron{transform:rotate(180deg)}
          `,
        }}
      />
      <style dangerouslySetInnerHTML={{ __html: codeBlockStyles() }} />
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify({
            "@context": "https://schema.org",
            "@type": "TechArticle",
            name: "UniPost API — GET /v1/social-accounts",
            description: "List connected social media accounts",
            url: "https://unipost.dev/docs/api/accounts/list",
            author: { "@type": "Organization", name: "UniPost" },
            dateModified: "2026-04-22",
          }),
        }}
      />

      <ApiReferencePage
        section="accounts"
        title="List accounts"
        description={<>Returns connected social accounts in the current workspace. Use it to discover publishable <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>account_id</code> values.</>}
      >
        <ApiReferenceGrid
          left={
            <>
              <div style={{ display: "grid", gap: 14 }}>
                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <div style={{ padding: "16px 18px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14 }}>
                    <div style={{ minWidth: 0 }}>
                      <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>GET</span>
                      <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{requestPath}</code>
                    </div>
                    <button
                      type="button"
                      onClick={handleSend}
                      disabled={isSending}
                      style={{
                        border: 0,
                        borderRadius: 12,
                        background: isSending ? "#fb6c45" : "#f04d23",
                        color: "#fff",
                        fontSize: 15,
                        fontWeight: 700,
                        padding: "13px 22px",
                        cursor: isSending ? "default" : "pointer",
                        display: "inline-flex",
                        alignItems: "center",
                        gap: 8,
                        flexShrink: 0,
                      }}
                    >
                      {isSending ? <Loader2 style={{ width: 16, height: 16 }} className="animate-spin" /> : null}
                      {isSending ? "Sending..." : "Send"}
                    </button>
                  </div>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <TryItSection title="Authorization" defaultOpen>
                    <TryItField
                      label="Authorization (header)"
                      type="string"
                      value={apiKey}
                      onChange={setApiKey}
                      placeholder="Bearer up_live_xxxx"
                    />
                  </TryItSection>
                  <TryItSection title="Query Params">
                    <div style={{ display: "grid", gap: 16 }}>
                      <TryItField
                        label="platform?"
                        type="string"
                        value={platform}
                        onChange={setPlatform}
                        placeholder="instagram"
                      />
                      <TryItField
                        label="external_user_id?"
                        type="string"
                        value={externalUserId}
                        onChange={setExternalUserId}
                        placeholder="user_123"
                      />
                    </div>
                  </TryItSection>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <div style={{ padding: "18px" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
                    {liveStatus !== null || liveError ? (
                      <div style={{ marginBottom: 18 }}>
                        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
                          <span style={{ fontFamily: "var(--docs-mono)", fontSize: 12, fontWeight: 700, color: "var(--docs-text-faint)", letterSpacing: ".08em", textTransform: "uppercase" }}>
                            Live Response
                          </span>
                          {liveStatus !== null ? (
                            <span style={{ fontFamily: "var(--docs-mono)", fontSize: 12, fontWeight: 700, color: liveStatus < 300 ? "#10b981" : "#f04d23" }}>
                              {liveStatus}
                            </span>
                          ) : null}
                        </div>
                        <CodeBlock code={liveResponse} language="json" compact />
                        {liveError ? (
                          <div style={{ fontSize: 13, color: "#f04d23", marginTop: 10 }}>{liveError}</div>
                        ) : null}
                      </div>
                    ) : (
                      <div style={{ fontSize: 14, lineHeight: 1.6, color: "var(--docs-text-muted)", marginBottom: 18 }}>
                        Send a request to see a live response here.
                      </div>
                    )}
                  </div>
                  <ApiAccordion title="200">
                    <ApiFieldList items={RESPONSE_200_FIELDS} />
                  </ApiAccordion>
                  <ApiAccordion title="401">
                    <ApiFieldList items={RESPONSE_401_FIELDS} />
                  </ApiAccordion>
                </ApiEndpointCard>
              </div>
            </>
          }
          right={
            <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
              <CodeTabs snippets={SNIPPETS} />
              <CodeTabs snippets={RESPONSE_SNIPPETS} />
            </div>
          }
        />
      </ApiReferencePage>
    </>
  );
}
