"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { Bot, Copy, Check, Terminal, ExternalLink } from "lucide-react";

const CODE_SNIPPETS = {
  claudeDesktop: `{
  "mcpServers": {
    "unipost": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "https://mcp.unipost.dev/mcp",
        "--header",
        "Authorization:Bearer YOUR_API_KEY",
        "--transport",
        "http-only"
      ]
    }
  }
}`,
  claudeCode: `claude mcp add unipost \\
  -t http \\
  --header "Authorization:Bearer YOUR_API_KEY" \\
  -- "https://mcp.unipost.dev/mcp"`,
  cursor: `{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_KEY"
      }
    }
  }
}`,
  curl: `curl -X POST https://mcp.unipost.dev/mcp \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json, text/event-stream" \\
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": { "name": "test", "version": "0.1.0" }
    }
  }'`,
};

type Client = keyof typeof CODE_SNIPPETS;

const CLIENTS: { key: Client; label: string; desc: string }[] = [
  { key: "claudeDesktop", label: "Claude Desktop", desc: "Add to claude_desktop_config.json" },
  { key: "claudeCode", label: "Claude Code", desc: "Run in your terminal" },
  { key: "cursor", label: "Cursor / Windsurf", desc: "Add to MCP settings JSON" },
  { key: "curl", label: "cURL", desc: "Test the connection manually" },
];

const TOOLS = [
  { name: "unipost_list_accounts", desc: "List all connected social media accounts" },
  { name: "unipost_create_post", desc: "Create and publish a post to one or more accounts" },
  { name: "unipost_get_post", desc: "Get the status and details of a published post" },
  { name: "unipost_get_analytics", desc: "Get engagement metrics for a published post" },
  { name: "unipost_list_posts", desc: "List recent posts filtered by status" },
];

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); }}
      style={{
        background: "none", border: "none", color: copied ? "var(--daccent)" : "var(--dmuted)",
        cursor: "pointer", padding: 4, display: "flex", alignItems: "center", gap: 4, fontSize: 11,
      }}
    >
      {copied ? <><Check style={{ width: 13, height: 13 }} /> Copied</> : <><Copy style={{ width: 13, height: 13 }} /> Copy</>}
    </button>
  );
}

export default function AIAgentsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const [activeClient, setActiveClient] = useState<Client>("claudeDesktop");

  return (
    <>
      {/* Header */}
      <div style={{ marginBottom: 32 }}>
        <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>AI Agents</div>
        <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>
          Connect AI agents to UniPost via the Model Context Protocol (MCP)
        </div>
      </div>

      {/* Server endpoint */}
      <div
        style={{
          background: "var(--surface)", border: "1px solid var(--dborder2)",
          borderRadius: 10, padding: "16px 20px", marginBottom: 24,
          display: "flex", alignItems: "center", justifyContent: "space-between",
        }}
      >
        <div>
          <div style={{ fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: 0.8, color: "var(--dmuted)", marginBottom: 6 }}>
            MCP Server Endpoint
          </div>
          <code style={{ fontSize: 13.5, color: "var(--daccent)", fontFamily: "var(--font-mono, monospace)" }}>
            https://mcp.unipost.dev/mcp
          </code>
        </div>
        <CopyButton text="https://mcp.unipost.dev/mcp" />
      </div>

      {/* Setup instructions */}
      <div style={{ marginBottom: 32 }}>
        <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 16 }}>Quick Setup</div>

        {/* Client tabs */}
        <div style={{ display: "flex", gap: 6, marginBottom: 16, flexWrap: "wrap" }}>
          {CLIENTS.map((c) => (
            <button
              key={c.key}
              onClick={() => setActiveClient(c.key)}
              className={activeClient === c.key ? "dbtn dbtn-primary" : "dbtn dbtn-ghost"}
              style={{ padding: "6px 14px", fontSize: 12.5 }}
            >
              {c.label}
            </button>
          ))}
        </div>

        {/* Instruction */}
        <div style={{ fontSize: 13, color: "var(--dmuted)", marginBottom: 12 }}>
          {CLIENTS.find((c) => c.key === activeClient)?.desc}
          {activeClient === "claudeDesktop" && (
            <span style={{ display: "block", marginTop: 4, fontSize: 12, color: "#888" }}>
              Requires <code style={{ fontSize: 11.5, color: "#aaa" }}>mcp-remote</code> package (auto-installed via npx).
              If <code style={{ fontSize: 11.5, color: "#aaa" }}>npx</code> is not found, replace with the full path (e.g. <code style={{ fontSize: 11.5, color: "#aaa" }}>/opt/homebrew/bin/npx</code>).
            </span>
          )}
        </div>

        {/* Code block */}
        <div
          style={{
            background: "#0d0d0d", border: "1px solid var(--dborder2)",
            borderRadius: 10, overflow: "hidden",
          }}
        >
          <div style={{
            display: "flex", alignItems: "center", justifyContent: "space-between",
            padding: "8px 14px", borderBottom: "1px solid var(--dborder2)", background: "#111",
          }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--dmuted)", fontSize: 11.5 }}>
              <Terminal style={{ width: 12, height: 12 }} />
              {activeClient === "claudeCode" ? "Terminal" : "JSON"}
            </div>
            <CopyButton text={CODE_SNIPPETS[activeClient].replace(/YOUR_API_KEY/g, "YOUR_API_KEY")} />
          </div>
          <pre style={{
            padding: "16px 20px", margin: 0, overflow: "auto",
            fontSize: 12.5, lineHeight: 1.65, color: "#e0e0e0",
            fontFamily: "var(--font-mono, monospace)",
          }}>
            {CODE_SNIPPETS[activeClient]}
          </pre>
        </div>

        <div style={{
          marginTop: 12, padding: "10px 14px", borderRadius: 8,
          background: "rgba(255,180,50,0.06)", border: "1px solid rgba(255,180,50,0.15)",
          fontSize: 12.5, color: "#cca040", lineHeight: 1.5,
        }}>
          Replace <code style={{ fontSize: 11.5, background: "rgba(255,180,50,0.1)", padding: "1px 5px", borderRadius: 3 }}>YOUR_API_KEY</code> with
          a production API key from the <a href={`/projects/${projectId}/api-keys`} style={{ color: "var(--daccent)", textDecoration: "underline" }}>API Keys</a> page.
        </div>
      </div>

      {/* Available tools */}
      <div style={{ marginBottom: 32 }}>
        <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 16 }}>Available Tools</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {TOOLS.map((tool) => (
            <div
              key={tool.name}
              style={{
                display: "flex", alignItems: "center", gap: 14,
                padding: "12px 16px", borderRadius: 8,
                background: "var(--surface)", border: "1px solid var(--dborder2)",
              }}
            >
              <code style={{
                fontSize: 12.5, color: "var(--daccent)", fontFamily: "var(--font-mono, monospace)",
                whiteSpace: "nowrap",
              }}>
                {tool.name}
              </code>
              <span style={{ fontSize: 13, color: "var(--dmuted)" }}>{tool.desc}</span>
            </div>
          ))}
        </div>
      </div>

      {/* How it works */}
      <div style={{ marginBottom: 32 }}>
        <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 16 }}>How It Works</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {[
            { step: "1", title: "Connect your social accounts", desc: "Link platforms like Bluesky, LinkedIn, Instagram, and more from the Accounts page." },
            { step: "2", title: "Generate an API key", desc: "Create a production API key to authenticate MCP requests." },
            { step: "3", title: "Configure your AI client", desc: "Add UniPost as an MCP server using the setup instructions above." },
            { step: "4", title: "Start posting", desc: "Ask your AI agent to create posts, check analytics, or manage your social presence." },
          ].map((item) => (
            <div key={item.step} style={{ display: "flex", gap: 14, alignItems: "flex-start" }}>
              <div style={{
                width: 26, height: 26, borderRadius: "50%", flexShrink: 0,
                background: "rgba(var(--accent-rgb, 99,102,241), 0.12)",
                color: "var(--daccent)", fontSize: 12, fontWeight: 700,
                display: "flex", alignItems: "center", justifyContent: "center",
              }}>
                {item.step}
              </div>
              <div>
                <div style={{ fontSize: 13.5, fontWeight: 600, color: "var(--dtext)", marginBottom: 2 }}>{item.title}</div>
                <div style={{ fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.5 }}>{item.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Example prompts */}
      <div>
        <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 16 }}>Example Prompts</div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 10 }}>
          {[
            "List my connected social accounts",
            "Post 'Hello from AI!' to all my Bluesky accounts",
            "Show me analytics for my latest post",
            "Schedule a post for tomorrow at 9am",
            "Which platforms had the most engagement this week?",
            "Create a post about our new product launch",
          ].map((prompt) => (
            <div
              key={prompt}
              style={{
                padding: "12px 14px", borderRadius: 8,
                background: "var(--surface)", border: "1px solid var(--dborder2)",
                fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.5,
                fontStyle: "italic",
              }}
            >
              &ldquo;{prompt}&rdquo;
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
