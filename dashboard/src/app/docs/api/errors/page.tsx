import { DocsPage, DocsTable } from "../../_components/docs-shell";

export default function ErrorsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Errors"
      lead="UniPost uses structured error responses so clients can distinguish validation issues, authorization failures, conflicts, and transient server errors without guessing from freeform text."
    >
      <h2 id="shape">Error shape</h2>
      <p>Top-level API errors are returned in an <code>error</code> envelope and now include a <code>request_id</code> field for tracing. During the migration to a stricter public contract, UniPost also returns a lowercase <code>error.normalized_code</code> alias alongside the historical <code>error.code</code>. Validation-heavy endpoints may also include an <code>issues[]</code> array with field-specific detail.</p>

      <h2 id="common-codes">Common codes</h2>
      <DocsTable
        columns={["HTTP", "Historical code", "Normalized code", "Meaning"]}
        rows={[
          ["400", "BAD_REQUEST / VALIDATION_ERROR", "bad_request / validation_error", "Malformed JSON or structurally invalid request"],
          ["401", "UNAUTHORIZED", "unauthorized", "Missing or invalid API key"],
          ["404", "NOT_FOUND", "not_found", "The requested resource is not in this workspace"],
          ["409", "CONFLICT", "conflict", "The request conflicts with current state, often around idempotency or draft promotion"],
          ["422", "VALIDATION_ERROR", "validation_error", "The body is structurally valid but violates business rules"],
          ["500", "INTERNAL_ERROR", "internal_error", "Unexpected server-side failure"],
        ]}
      />

      <h2 id="migration">Migration guidance</h2>
      <p>Public clients should branch on <code>error.normalized_code</code> going forward. The uppercase <code>error.code</code> field remains for backward compatibility while UniPost finishes normalizing older endpoints and SDK surfaces.</p>

      <h2 id="platform-errors">Platform-side errors</h2>
      <p>Per-platform publish failures usually appear in <code>results[].error_message</code> on a post response rather than as top-level API errors. That allows UniPost to represent partial success across platforms.</p>
    </DocsPage>
  );
}
