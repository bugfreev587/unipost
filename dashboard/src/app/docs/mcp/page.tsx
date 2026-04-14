import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const MCP_SNIPPETS = [
  {
    label: "Claude Desktop",
    code: `{
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
  },
  {
    label: "Cursor",
    code: `{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_KEY"
      }
    }
  }
}`,
  },
  {
    label: "Windsurf",
    code: `{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_KEY"
      }
    }
  }
}`,
  },
  {
    label: "Claude Code",
    code: `claude mcp add unipost \\
  -t http \\
  --header "Authorization:Bearer YOUR_API_KEY" \\
  -- "https://mcp.unipost.dev/mcp"`,
  },
];

const CURL_SNIPPET = [
  {
    label: "cURL",
    code: `curl -X POST https://mcp.unipost.dev/mcp \\
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
  },
];

export default function McpPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="MCP"
      lead="UniPost exposes a hosted Model Context Protocol server so agents can list accounts, validate content, publish posts, and inspect analytics through one tool layer instead of seven separate platform APIs."
    >
      <h2 id="what-it-is">What MCP is for</h2>
      <p>Use MCP when your product or workflow already runs AI agents. Instead of teaching a model the exact request shapes, limits, and quirks of every social network, you give it one consistent interface for accounts, posts, validation, and analytics.</p>

      <h2 id="transport">Transport and endpoint</h2>
      <DocsTable
        columns={["Property", "Value"]}
        rows={[
          ["Endpoint", "https://mcp.unipost.dev/mcp"],
          ["Transport", "Streamable HTTP"],
          ["Auth", "Bearer YOUR_API_KEY"],
          ["Legacy fallback", "https://mcp.unipost.dev/sse"],
        ]}
      />

      <h2 id="tools">Available tools</h2>
      <DocsTable
        columns={["Tool", "What it does"]}
        rows={[
          ["unipost_list_accounts", "List connected social media accounts"],
          ["unipost_create_post", "Create and publish a post to one or more accounts"],
          ["unipost_get_post", "Get the status and details of a post"],
          ["unipost_get_analytics", "Read engagement metrics for a post"],
          ["unipost_list_posts", "List recent posts filtered by status"],
        ]}
      />

      <h2 id="recommended-flow">Recommended flow</h2>
      <p>The best MCP workflow is not &ldquo;generate text, then publish immediately.&rdquo; The safer pattern is generate, validate, preview if needed, then publish.</p>
      <DocsTable
        columns={["Step", "Why it exists"]}
        rows={[
          ["List accounts", "Ground the agent in real destination accounts"],
          ["Draft candidate copy", "Let the model propose platform-aware captions"],
          ["Validate", "Catch caption, media, and support issues before publish"],
          ["Preview", "Send a human-readable link when review is required"],
          ["Publish", "Commit once the draft is approved"],
        ]}
      />

      <h2 id="client-config">Client configuration</h2>
      <p>Here is the part that was missing: each client expects this config in a different place. Copy the matching snippet into the file below, then restart the client.</p>
      <DocsTable
        columns={["Client", "Put the config here", "Notes"]}
        rows={[
          ["Claude Desktop", "~/Library/Application Support/Claude/claude_desktop_config.json (macOS) or %APPDATA%\\Claude\\claude_desktop_config.json (Windows)", "Replace the whole `mcpServers` block or merge the `unipost` entry into your existing file"],
          ["Cursor", ".cursor/mcp.json in your project or ~/.cursor/mcp.json for a global setup", "Project config is easiest when you want the same MCPs checked into a repo"],
          ["Windsurf", "~/.codeium/windsurf/mcp_config.json", "You can also open MCP settings in Windsurf and edit the raw config there"],
          ["Claude Code", "No file needed for the command below. Run it in your terminal instead.", "If you want a checked-in config, use `.mcp.json` at the project root with `claude mcp add --scope project`"],
        ]}
      />
      <DocsCodeTabs snippets={MCP_SNIPPETS} />

      <h2 id="testing">Test the server directly</h2>
      <p>If you want to confirm auth and transport outside an MCP client, initialize the server directly with cURL.</p>
      <DocsCodeTabs snippets={CURL_SNIPPET} />

      <h2 id="when-to-use">When to use MCP vs SDK vs raw API</h2>
      <DocsTable
        columns={["Use case", "Best interface", "Why"]}
        rows={[
          ["LLM-driven operator or agent workflow", "MCP", "Native tool interface for accounts, posts, and analytics"],
          ["Typed application integration", "SDK", "Better ergonomics and stronger language-native patterns"],
          ["Low-level debugging or custom client", "API Reference", "Direct control over raw requests and responses"],
        ]}
      />
    </DocsPage>
  );
}
