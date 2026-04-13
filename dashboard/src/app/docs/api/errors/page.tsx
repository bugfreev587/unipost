import { DocsPage, DocsTable } from "../../_components/docs-shell";

export default function ErrorsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Errors"
      lead="UniPost uses structured error responses so clients can distinguish validation issues, authorization failures, conflicts, and transient server errors without guessing from freeform text."
    >
      <h2 id="shape">Error shape</h2>
      <p>Top-level API errors are returned in an <code>error</code> envelope. Validation-heavy endpoints may also include an <code>issues[]</code> array with field-specific detail.</p>

      <h2 id="common-codes">Common codes</h2>
      <DocsTable
        columns={["HTTP", "Code", "Meaning"]}
        rows={[
          ["400", "VALIDATION_ERROR", "Malformed JSON or structurally invalid request"],
          ["401", "UNAUTHORIZED", "Missing or invalid API key"],
          ["404", "NOT_FOUND", "The requested resource is not in this workspace"],
          ["409", "CONFLICT", "The request conflicts with current state, often around idempotency or draft promotion"],
          ["422", "VALIDATION_ERROR", "The body is structurally valid but violates business rules"],
          ["500", "INTERNAL_ERROR", "Unexpected server-side failure"],
        ]}
      />

      <h2 id="platform-errors">Platform-side errors</h2>
      <p>Per-platform publish failures usually appear in <code>results[].error_message</code> on a post response rather than as top-level API errors. That allows UniPost to represent partial success across platforms.</p>
    </DocsPage>
  );
}
