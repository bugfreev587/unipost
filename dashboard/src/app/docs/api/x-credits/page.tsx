import Link from "next/link";
import {
  ApiReferencePage,
  CodeTabs,
  DocSection,
  EndpointHeader,
  ErrorTable,
  ParamTable,
  type ParamRow,
} from "../_components/doc-components";
import { X_CREDIT_OPERATIONS, X_CREDITS_CATALOG_VERSION } from "@/data/x-credits-catalog.generated";
import { requirePublicDocsFeature } from "@/lib/public-feature-flags-server";

const RESPONSE_FIELDS: ParamRow[] = [
  { name: "mode", type: "string", required: true, description: 'Always "monthly_allowance" in the bounded-usage phase.' },
  { name: "plan_id", type: "string", required: true, description: "Workspace plan used to select the included allowance." },
  { name: "monthly_allowance", type: "integer | null", required: true, description: "Included X Credits for the billing period. Enterprise returns null for contract-defined capacity." },
  { name: "monthly_used", type: "integer", required: true, description: "Finalized and provisional weighted usage in the current billing period." },
  { name: "monthly_remaining", type: "integer | null", required: true, description: "Remaining included X Credits. Enterprise returns null." },
  { name: "billing_period_start", type: "string", required: true, description: "ISO-8601 start of the current allowance period." },
  { name: "billing_period_end", type: "string", required: true, description: "ISO-8601 reset boundary for the current allowance period." },
  { name: "catalog_version", type: "string", required: true, description: `Operation catalog version. Current value: ${X_CREDITS_CATALOG_VERSION}.` },
  { name: "inbound_daily_usage", type: "integer", required: true, description: "Weighted inbound X usage accepted today in UTC." },
  { name: "inbound_daily_limit", type: "integer | null", required: true, description: "Daily inbound safety limit. Enterprise returns null for contract-defined capacity." },
  { name: "connection_mode_note", type: "string", required: true, description: "Explains that managed X connections consume UniPost X Credits while BYO connections do not." },
];

const RESPONSE_EXAMPLE = `{
  "data": {
    "mode": "monthly_allowance",
    "plan_id": "basic",
    "monthly_allowance": 4000,
    "monthly_used": 215,
    "monthly_remaining": 3785,
    "billing_period_start": "2026-07-01T00:00:00Z",
    "billing_period_end": "2026-08-01T00:00:00Z",
    "catalog_version": "${X_CREDITS_CATALOG_VERSION}",
    "inbound_daily_usage": 25,
    "inbound_daily_limit": 400,
    "connection_mode_note": "Managed X connections consume this allowance. Bring-your-own X API connections do not consume UniPost X Credits."
  },
  "request_id": "req_123"
}`;

export default async function XCreditsReferencePage() {
  const publicFeatureFlags = await requirePublicDocsFeature("x_credits_billing_v1");

  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "X Credits" }]}
      section="api"
      title="X Credits"
      description="Inspect the workspace's included managed-X allowance after X Credits billing is enabled. The endpoint is unavailable during the controlled rollout for regular workspaces."
    >
      <EndpointHeader
        method="GET"
        path="/v1/billing/x-credits"
        description="Returns the authenticated workspace's bounded monthly X Credits allowance."
        badges={["Bearer token", "Workspace scoped"]}
      />

      <div style={{ display: "grid", gap: 34 }}>
        <DocSection id="request" title="Request">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            X Credits billing is controlled by <code>x_credits_billing_v1</code>. Until it is enabled for the workspace,
            managed X calls do not count against or block on the customer monthly balance and this endpoint returns
            <code> FEATURE_NOT_AVAILABLE</code>. The 20 X publishes/account/day limit and internal inbound safety cap
            remain active.
          </p>
          <CodeTabs
            snippets={[
              {
                lang: "curl",
                label: "cURL",
                code: `curl "https://api.unipost.dev/v1/billing/x-credits" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
              },
            ]}
          />
        </DocSection>

        <DocSection id="response" title="Response">
          <ParamTable params={RESPONSE_FIELDS} />
          <div style={{ marginTop: 18 }}>
            <CodeTabs snippets={[{ lang: "json", label: "200", code: RESPONSE_EXAMPLE }]} />
          </div>
        </DocSection>

        <DocSection id="operation-catalog" title="Operation catalog">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            The public catalog is versioned. X Credits are weighted units, not dollars, and are separate from the
            workspace&apos;s posts/month allowance. The table includes the shipped X Inbox read, inbound, reply, and legacy
            DM operations used by the list, reply, and sync workflows.
          </p>
          <div style={{ overflowX: "auto" }}>
            <table className="docs-table">
              <thead>
                <tr><th>Operation key</th><th>Description</th><th>Credits</th></tr>
              </thead>
              <tbody>
                {X_CREDIT_OPERATIONS.filter((operation) => operation.phase === "mvp").map((operation) => (
                  <tr key={operation.key}>
                    <td><code>{operation.key}</code></td>
                    <td>{operation.label}</td>
                    <td>{operation.credits}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </DocSection>

        <DocSection id="connection-modes" title="Managed versus BYO X connections">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Managed X connections use UniPost&apos;s X developer app and consume this allowance. Bring-your-own X API
            connections use the customer&apos;s developer credentials and do not consume UniPost X Credits. Platform-owned
            limits, abuse controls, and the independent 20-post daily X safety cap still apply to both modes.
          </p>
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            Account capabilities expose these identities as <code>unipost_managed_app</code> and
            <code> workspace_x_app</code>. X Inbox requires the Basic plan or higher. Managed inbound delivery also stops
            at <code>x_inbound_daily_cap_exceeded</code> independently of the monthly allowance.
          </p>
        </DocSection>

        <DocSection id="errors" title="Errors">
          <ErrorTable
            errors={[
              { code: "unauthorized", http: 401, description: "The request is missing valid workspace authentication." },
              { code: "feature_not_available", http: 403, description: "X Credits billing is not enabled for this workspace yet." },
              { code: "x_monthly_usage_limit_exceeded", http: 402, description: "The managed-X hard limit has been reached for this billing period. Wait for reset or upgrade/contact UniPost." },
              { code: "internal_error", http: 500, description: "The allowance snapshot could not be loaded. Retry and include request_id if contacting support." },
            ]}
          />
        </DocSection>

        <DocSection id="next" title="Next steps">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Use the <Link href="/docs/guides/x/credits">X Credits guide</Link> to estimate operations and handle
            exhaustion. Continue with <Link href="/docs/guides/x/comments">X comments</Link>
            {publicFeatureFlags.x_dms_v1 ? <> or <Link href="/docs/guides/x/direct-messages">X direct messages</Link></> : null}.
            Compare included plan capacity in{" "}
            <Link href="/docs/pricing">Plans and limits</Link>.
          </p>
        </DocSection>
      </div>
    </ApiReferencePage>
  );
}
