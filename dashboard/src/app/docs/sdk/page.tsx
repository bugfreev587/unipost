import Link from "next/link";
import { DocsPage } from "../_components/docs-shell";

const SDKS = [
  {
    name: "JavaScript",
    language: "TypeScript + Node.js",
    repo: "github.com/unipost-dev/sdk-js",
    href: "https://github.com/unipost-dev/sdk-js",
    install: "npm install @unipost/sdk",
  },
  {
    name: "Python",
    language: "Python 3.9+",
    repo: "github.com/unipost-dev/sdk-python",
    href: "https://github.com/unipost-dev/sdk-python",
    install: "pip install unipost",
  },
  {
    name: "Go",
    language: "Go 1.21+",
    repo: "github.com/unipost-dev/sdk-go",
    href: "https://github.com/unipost-dev/sdk-go",
    install: "go get github.com/unipost-dev/sdk-go",
  },
] as const;

function GithubGlyph() {
  return (
    <svg
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M12 .5C5.73.5.75 5.48.75 11.75c0 4.97 3.22 9.18 7.69 10.67.56.1.77-.24.77-.54 0-.27-.01-.98-.02-1.93-3.13.68-3.79-1.51-3.79-1.51-.51-1.29-1.24-1.64-1.24-1.64-1.01-.69.08-.68.08-.68 1.12.08 1.71 1.15 1.71 1.15 1 1.71 2.63 1.22 3.27.93.1-.72.39-1.22.71-1.5-2.5-.28-5.13-1.25-5.13-5.57 0-1.23.44-2.24 1.16-3.03-.12-.28-.5-1.42.11-2.96 0 0 .94-.3 3.08 1.16.89-.25 1.85-.37 2.8-.38.95.01 1.91.13 2.8.38 2.14-1.46 3.08-1.16 3.08-1.16.61 1.54.23 2.68.11 2.96.72.79 1.16 1.8 1.16 3.03 0 4.33-2.64 5.29-5.15 5.56.4.35.76 1.03.76 2.08 0 1.5-.01 2.71-.01 3.08 0 .3.2.65.78.54 4.47-1.49 7.68-5.7 7.68-10.67C23.25 5.48 18.27.5 12 .5z" />
    </svg>
  );
}

export default function SdkPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="SDKs"
      title="SDKs"
      lead="Official UniPost client libraries for JavaScript, Python, and Go. Publish to every connected platform with one typed interface."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <h2 id="official-sdks">Official SDKs</h2>
      <div className="sdk-card-grid">
        {SDKS.map((sdk) => (
          <Link
            href={sdk.href}
            target="_blank"
            rel="noreferrer noopener"
            className="sdk-card"
            key={sdk.name}
          >
            <div className="sdk-card-head">
              <span className="sdk-card-icon">
                <GithubGlyph />
              </span>
              <span className="sdk-card-arrow" aria-hidden="true">
                ↗
              </span>
            </div>
            <div className="sdk-card-body">
              <div className="sdk-card-title">{sdk.name}</div>
              <div className="sdk-card-sub">{sdk.language}</div>
              <div className="sdk-card-repo">{sdk.repo}</div>
            </div>
            <div className="sdk-card-install">
              <code>{sdk.install}</code>
            </div>
          </Link>
        ))}
      </div>

      <div className="docs-callout">
        <strong>Need a language we don&rsquo;t ship yet?</strong> The REST API is the source of truth — any HTTP client can talk to it. See the <Link href="/docs/api">API reference</Link> to get started, or <Link href="mailto:support@unipost.dev">let us know</Link> which runtime to prioritize next.
      </div>
    </DocsPage>
  );
}

const styles = `
.sdk-card-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin:22px 0 28px}
.sdk-card{position:relative;display:flex;flex-direction:column;gap:18px;padding:22px 22px 20px;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);box-shadow:0 1px 0 rgba(255,255,255,.02);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.sdk-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-2px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.sdk-card-head{display:flex;align-items:center;justify-content:space-between}
.sdk-card-icon{display:inline-flex;align-items:center;justify-content:center;width:38px;height:38px;border-radius:12px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text)}
.sdk-card-arrow{font-size:15px;line-height:1;color:var(--docs-text-faint);opacity:.55;transition:opacity .15s ease,transform .15s ease}
.sdk-card:hover .sdk-card-arrow{opacity:1;color:var(--docs-link);transform:translate(2px,-2px)}
.sdk-card-body{display:flex;flex-direction:column;gap:4px;min-width:0}
.sdk-card-title{font-size:17px;font-weight:700;letter-spacing:-.02em;color:var(--docs-text)}
.sdk-card-sub{font-size:12.5px;font-weight:600;letter-spacing:.02em;color:var(--docs-text-faint)}
.sdk-card-repo{font-family:var(--docs-mono);font-size:13px;color:var(--docs-text-soft);line-height:1.5;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.sdk-card-install{padding:10px 12px;border-radius:10px;background:var(--docs-inline-code-bg);border:1px solid var(--docs-border);overflow:hidden}
.sdk-card-install code{font-family:var(--docs-mono);font-size:12.5px;color:var(--docs-text);background:transparent;border:none;padding:0;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
@media (max-width:960px){.sdk-card-grid{grid-template-columns:1fr}}
`;
