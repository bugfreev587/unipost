import { DocsCode, DocsPage } from "../_components/docs-shell";

export default function McpPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="MCP"
      lead="UniPost exposes a Model Context Protocol server so agents can draft, validate, publish, and inspect analytics without custom glue code for each platform."
    >
      <h2 id="what-it-is">What it is</h2>
      <p>Use MCP when your product or workflow already runs AI agents. Instead of teaching the model seven different social APIs, you give it one tool layer with consistent publishing semantics.</p>

      <h2 id="recommended-flow">Recommended flow</h2>
      <ul className="docs-list">
        <li>Generate a candidate draft with your model.</li>
        <li>Call UniPost Validate to catch platform-specific issues.</li>
        <li>Create a preview link if a human should review the content.</li>
        <li>Publish through UniPost once the draft is approved.</li>
      </ul>

      <h2 id="mcp-config">MCP config</h2>
      <DocsCode
        code={`{
  "mcpServers": {
    "unipost": {
      "command": "npx",
      "args": ["-y", "@unipost/mcp"],
      "env": {
        "UNIPOST_API_KEY": "up_live_xxxx"
      }
    }
  }
}`}
      />
    </DocsPage>
  );
}
