import Link from "next/link";
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
      className="docs-page-wide"
      eyebrow="Get Started"
      title="MCP"
      lead="Hosted Model Context Protocol server. Give your agent one tool interface for every connected platform instead of seven separate APIs."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="mcp-badges">
        <span className="mcp-badge">Hosted</span>
        <span className="mcp-badge">Streamable HTTP</span>
        <span className="mcp-badge">Bearer auth</span>
        <span className="mcp-badge">8 tools</span>
        <span className="mcp-badge">4 clients supported</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who is it for", "Agents and LLM workflows that need to post, analyze, and manage social accounts"],
          ["How it runs", "Hosted by UniPost — no server to deploy"],
          ["Auth", "Bearer API key in the `Authorization` header"],
          ["Transport", "Streamable HTTP — falls back to SSE for legacy clients"],
          ["Setup time", "~2 minutes per client"],
        ]}
      />

      <h2 id="when-to-use">MCP vs SDK vs raw API</h2>
      <p className="mcp-note">The first question worth answering: should your integration even use MCP?</p>
      <DocsTable
        columns={["Use case", "Best interface", "Why"]}
        rows={[
          ["LLM-driven operator or agent workflow", "MCP", "Native tool surface for accounts, posts, and analytics"],
          ["Typed application integration", "SDK", "Better ergonomics and language-native patterns"],
          ["Low-level debugging or custom client", "Raw API", "Direct control over raw requests and responses"],
          ["Large local video upload before publish", "Media API + MCP", "Upload to UniPost storage first, then publish with `media_ids`"],
        ]}
      />

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
          ["unipost_upload_media", "Upload media into UniPost's media library and return a `media_id`"],
          ["unipost_get_media", "Check whether a media upload is hydrated and ready to publish"],
          ["unipost_create_post", "Create and publish a post to one or more accounts"],
          ["unipost_create_youtube_video_post", "Upload + publish in one YouTube-oriented video workflow"],
          ["unipost_get_post", "Get the status and details of a post"],
          ["unipost_get_analytics", "Read engagement metrics for a post"],
          ["unipost_list_posts", "List recent posts filtered by status"],
        ]}
      />
      <p className="mcp-note">The MCP surface is strongest for account lookup, text publishing, analytics, and media that is already reachable by URL or already uploaded into UniPost. Large local video files are not the ideal path today.</p>

      <h2 id="recommended-flow">Recommended flow</h2>
      <p className="mcp-note">&ldquo;Generate text, then publish immediately&rdquo; is not the safest pattern. Prefer generate → validate → preview → publish.</p>
      <div className="mcp-flow">
        <div className="mcp-flow-step">
          <div className="mcp-flow-num">1</div>
          <div className="mcp-flow-body">
            <div className="mcp-flow-title">List accounts</div>
            <div className="mcp-flow-sub">Ground the agent in real destination accounts.</div>
          </div>
        </div>
        <div className="mcp-flow-step">
          <div className="mcp-flow-num">2</div>
          <div className="mcp-flow-body">
            <div className="mcp-flow-title">Draft candidate copy</div>
            <div className="mcp-flow-sub">Let the model propose platform-aware captions.</div>
          </div>
        </div>
        <div className="mcp-flow-step">
          <div className="mcp-flow-num">3</div>
          <div className="mcp-flow-body">
            <div className="mcp-flow-title">Validate</div>
            <div className="mcp-flow-sub">Catch caption, media, and support issues before publish.</div>
          </div>
        </div>
        <div className="mcp-flow-step">
          <div className="mcp-flow-num">4</div>
          <div className="mcp-flow-body">
            <div className="mcp-flow-title">Preview</div>
            <div className="mcp-flow-sub">Send a human-readable link when review is required.</div>
          </div>
        </div>
        <div className="mcp-flow-step">
          <div className="mcp-flow-num">5</div>
          <div className="mcp-flow-body">
            <div className="mcp-flow-title">Publish</div>
            <div className="mcp-flow-sub">Commit once the draft is approved.</div>
          </div>
        </div>
      </div>

      <h2 id="youtube-video-workflow">YouTube video workflow</h2>
      <p className="mcp-note">For YouTube, the most reliable flow today is: upload the video into UniPost&rsquo;s media library first, then create the post with a <code>media_id</code>. That matches the dashboard flow and avoids pushing a large local file through the MCP request path.</p>
      <DocsTable
        columns={["Step", "What to do", "Why"]}
        rows={[
          ["1", "Create a media upload in UniPost", "Reserve a media row and get a presigned upload URL"],
          ["2", "Upload the local video directly to storage", "Keep large file transfer out of the MCP request path"],
          ["3", "Confirm the media row is uploaded", "Make sure UniPost can resolve the object before publish"],
          ["4", "Call `unipost_create_post` with `media_ids`", "Publish to YouTube using the same workflow as the dashboard"],
        ]}
      />
      <p className="mcp-note">UniPost exposes <code>unipost_create_youtube_video_post</code> as a higher-level wrapper, but for very large local files the most reliable path is still a hosted <code>video_url</code> or a reusable <code>media_id</code>.</p>

      <h2 id="client-config">Client configuration</h2>
      <p className="mcp-note">Each client expects this config in a different place. Drop the matching snippet in the file below, then restart the client.</p>
      <DocsTable
        columns={["Client", "Config location", "Notes"]}
        rows={[
          ["Claude Desktop", "`~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\\Claude\\claude_desktop_config.json` (Windows)", "Merge the `unipost` entry into the existing `mcpServers` block"],
          ["Cursor", "`.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global)", "Project config is easiest when you want the same MCPs checked into a repo"],
          ["Windsurf", "`~/.codeium/windsurf/mcp_config.json`", "Or open MCP settings and edit the raw config there"],
          ["Claude Code", "No file needed — run the `claude mcp add` command", "For a checked-in config, use `.mcp.json` at the project root with `claude mcp add --scope project`"],
        ]}
      />
      <DocsCodeTabs snippets={MCP_SNIPPETS} />

      <h2 id="testing">Test the server directly</h2>
      <p className="mcp-note">If you want to confirm auth and transport outside an MCP client, initialize the server directly with cURL.</p>
      <DocsCodeTabs snippets={CURL_SNIPPET} />

      <h2 id="limitations">Limitations</h2>
      <DocsTable
        columns={["Limitation", "Reason"]}
        rows={[
          ["Large local video uploads are awkward", "MCP request path is not built for big binary transfer — use the Media API first, then publish with `media_ids`"],
          ["Hosted only, no self-host today", "The MCP surface runs on UniPost infrastructure — agents authenticate with their API key"],
          ["Tool inventory is curated", "UniPost intentionally exposes a small, stable tool set — new tools ship under the same naming convention"],
          ["Rate limits follow your UniPost plan", "The same per-key limits as the REST API apply"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="mcp-next">
        <Link href="/docs/quickstart" className="mcp-next-card">
          <div className="mcp-next-kicker">Start here</div>
          <div className="mcp-next-title">Quickstart</div>
          <div className="mcp-next-body">Get an API key and connect your first account so the agent has something to publish to.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="mcp-next-card">
          <div className="mcp-next-kicker">API reference</div>
          <div className="mcp-next-title">Create post</div>
          <div className="mcp-next-body">Full request shape behind the <code>unipost_create_post</code> MCP tool.</div>
        </Link>
        <Link href="/docs/api/media" className="mcp-next-card">
          <div className="mcp-next-kicker">API reference</div>
          <div className="mcp-next-title">Media API</div>
          <div className="mcp-next-body">Reserve and upload media from local files before publish.</div>
        </Link>
        <Link href="/docs/platforms" className="mcp-next-card">
          <div className="mcp-next-kicker">Per platform</div>
          <div className="mcp-next-title">Platform guides</div>
          <div className="mcp-next-body">Caption limits, media rules, and what each platform actually supports.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.mcp-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 24px}
.mcp-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.mcp-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.mcp-note code{font-family:var(--docs-mono);font-size:12.5px}
.mcp-flow{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin:14px 0 6px}
.mcp-flow-step{display:grid;grid-template-columns:36px 1fr;gap:14px;align-items:start;padding:14px 16px;border:1px solid var(--docs-border);border-radius:14px;background:var(--docs-bg-elevated)}
.mcp-flow-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:13px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border))}
.mcp-flow-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:3px}
.mcp-flow-sub{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.mcp-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.mcp-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.mcp-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.mcp-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.mcp-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.mcp-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.mcp-next-body code{font-family:var(--docs-mono);font-size:12px}
@media (max-width:960px){
  .mcp-flow{grid-template-columns:1fr}
  .mcp-next{grid-template-columns:1fr}
}
`;
