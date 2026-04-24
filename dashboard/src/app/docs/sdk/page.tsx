import Link from "next/link";
import { DocsPage } from "../_components/docs-shell";

const SDKS = [
  {
    name: "JavaScript",
    repo: "github.com/unipost-dev/sdk-js",
    href: "https://github.com/unipost-dev/sdk-js",
  },
  {
    name: "Python",
    repo: "github.com/unipost-dev/sdk-python",
    href: "https://github.com/unipost-dev/sdk-python",
  },
  {
    name: "Go",
    repo: "github.com/unipost-dev/sdk-go",
    href: "https://github.com/unipost-dev/sdk-go",
  },
] as const;

export default function SdkPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="SDKs"
      title="SDKs"
      lead="Official UniPost client libraries for JavaScript, Python, and Go."
    >
      <div className="docs-grid">
        {SDKS.map((sdk) => (
          <div className="docs-card" key={sdk.name}>
            <div className="docs-card-title">{sdk.name}</div>
            <p>
              <Link href={sdk.href} target="_blank" rel="noreferrer noopener">
                {sdk.repo}
              </Link>
            </p>
          </div>
        ))}
      </div>
    </DocsPage>
  );
}
