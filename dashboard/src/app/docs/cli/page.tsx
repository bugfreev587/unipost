import { DocsPage } from "../_components/docs-shell";

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Overview"
      title="CLI"
      lead="A first-party UniPost CLI is planned so developers can script common workflows like auth checks, account inspection, publish operations, and analytics queries from the terminal."
    >
      <div className="docs-callout">
        <strong>Coming soon:</strong> the CLI page will document installation, authentication, and common commands once the first public release is ready.
      </div>
    </DocsPage>
  );
}
