import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";

export default function DocsPricingPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="Pricing"
      lead="Pricing belongs in the docs because it affects architecture decisions: which environments you use, whether you need white-label Connect, and how far you want to push usage-based automation."
    >
      <h2 id="when-to-read">When to read this page</h2>
      <p>If you are deciding whether to build with BYO accounts, managed Connect sessions, or white-label onboarding, read pricing early. Those decisions change both product scope and operational cost.</p>

      <h2 id="plan-shape">Plan shape</h2>
      <DocsTable
        columns={["Need", "Docs path", "Pricing path"]}
        rows={[
          ["Ship your own content", "Quickstart + Posts API", "Standard publish usage"],
          ["Onboard customer accounts", "Connect Sessions + Managed Users", "Managed account usage"],
          ["Brand the onboarding flow", "Connect + Branding", "White-label plan"],
        ]}
      />

      <h2 id="full-pricing">Full pricing</h2>
      <p>The detailed pricing table still lives on the product marketing page so you can compare plans side by side.</p>
      <p><Link href="/pricing">Open full pricing</Link></p>
    </DocsPage>
  );
}
