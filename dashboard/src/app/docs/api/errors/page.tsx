"use client";

import { ApiInlineLink, ApiReferencePage, CodeTabs, DocSection, ErrorTable, ParamTable, type ParamRow } from "../_components/doc-components";

const ERROR_FIELDS: ParamRow[] = [
  { name: "error.code", type: "string", required: true, description: "Stable API error code. Existing CLI output normalizes this value for backward compatibility." },
  { name: "error.normalized_code", type: "string", required: true, description: "Lowercase machine-readable alias for routing and support automation." },
  { name: "error.message", type: "string", required: true, description: "Customer-safe summary of what failed." },
  { name: "error.hint", type: "string", required: false, description: "Short remediation guidance when UniPost can safely suggest one." },
  { name: "error.next_action", type: "string", required: false, description: "Stable action enum for client UI and automation. Present only on actionable errors." },
  { name: "error.is_retriable", type: "boolean", required: false, description: "Whether retrying the same request later is expected to help." },
  { name: "error.error_source", type: "string", required: false, description: "Where the failure originated: unipost, platform, worker, or unknown." },
  { name: "error.error_temporality", type: "string", required: false, description: "Whether the condition is temporary, permanent, or unknown." },
  { name: "error.provider_error", type: "object", required: false, description: "Sanitized provider fields such as provider, http_status, code, subcode, type, reason, domain, quota_limit, quota_location, and is_transient." },
  { name: "error.retry_policy", type: "object", required: false, description: "Best-effort retry snapshot. Use will_retry for automatic retry state; use is_retriable for retry eligibility." },
  { name: "error.docs_url", type: "string", required: false, description: "Relevant documentation URL for the failed request." },
  { name: "error.issues", type: "array", required: false, description: "Field-level validation issues. Preserve this array in clients instead of parsing message text." },
  { name: "request_id", type: "string", required: true, description: "Request identifier to include when contacting support." },
];

const ISSUE_FIELDS: ParamRow[] = [
  { name: "platform_post_index", type: "integer", required: false, description: "0-based index from platform_posts[] when the issue belongs to one destination." },
  { name: "account_id", type: "string", required: false, description: "Connected account associated with the issue." },
  { name: "platform", type: "string", required: false, description: "Resolved platform, such as tiktok, instagram, youtube, or linkedin." },
  { name: "field", type: "string", required: false, description: "Request field that needs a change, for example caption or platform_options.youtube.title." },
  { name: "code", type: "string", required: true, description: "Machine-readable issue code, such as exceeds_max_length or missing_required_field." },
  { name: "message", type: "string", required: true, description: "Customer-facing explanation for this one issue." },
  { name: "hint", type: "string", required: false, description: "Specific fix guidance for this issue." },
  { name: "next_action", type: "string", required: false, description: "Action enum for blocking validation errors." },
  { name: "actual", type: "any", required: false, description: "Actual submitted value or count when useful for debugging." },
  { name: "limit", type: "any", required: false, description: "Platform limit that was exceeded or missed." },
  { name: "severity", type: "string", required: true, description: "error for blocking issues, warning for non-blocking issues." },
];

const NEXT_ACTION_FIELDS: ParamRow[] = [
  { name: "fix_request", type: "validation", required: false, description: "Change the request payload, then validate again." },
  { name: "shorten_caption", type: "validation", required: false, description: "Shorten the caption or title to the platform limit." },
  { name: "review_platform_options", type: "publish", required: false, description: "Check privacy, disclosure, content, or destination options for the platform." },
  { name: "fix_media", type: "publish", required: false, description: "Replace or re-encode the media to meet platform requirements." },
  { name: "retry_later", type: "publish", required: false, description: "The platform or worker path is temporarily unavailable." },
  { name: "wait_and_retry", type: "rate limit", required: false, description: "Respect Retry-After or wait before sending the same request again." },
  { name: "review_quota", type: "quota", required: false, description: "Reduce usage or upgrade capacity before retrying." },
  { name: "reconnect_account", type: "auth", required: false, description: "Reconnect the affected social account." },
  { name: "reconnect_or_update_permissions", type: "auth", required: false, description: "Reconnect with the required scopes or permissions." },
  { name: "select_valid_target", type: "publish", required: false, description: "Choose a valid account, page, board, post, or other platform target." },
  { name: "contact_support", type: "support", required: false, description: "Send the request_id to UniPost support. Do not parse provider payloads yourself." },
];

const ERROR_EXAMPLE = `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "request failed pre-publish validation",
    "hint": "Fix the listed validation issues and retry.",
    "next_action": "fix_request",
    "is_retriable": false,
    "error_source": "unipost",
    "error_temporality": "permanent",
    "docs_url": "https://unipost.dev/docs/api/posts/validate",
    "issues": [
      {
        "platform_post_index": 0,
        "account_id": "sa_tiktok_1",
        "platform": "tiktok",
        "field": "caption",
        "code": "exceeds_max_length",
        "message": "TikTok photo title must be 90 characters or fewer. UniPost uses the caption as the TikTok photo title for photo posts.",
        "hint": "Shorten this TikTok photo caption/title to 90 characters or fewer before publishing.",
        "next_action": "shorten_caption",
        "actual": 91,
        "limit": 90,
        "severity": "error"
      }
    ]
  },
  "request_id": "req_123"
}`;

const FAILED_RESULT_EXAMPLE = `{
  "id": "spr_tiktok_1",
  "social_account_id": "sa_tiktok_1",
  "platform": "tiktok",
  "status": "failed",
  "error_message": "TikTok rejected the photo publish request: TikTok reported invalid_params. Common fixes: confirm the TikTok privacy/content options are supported. For photo posts, keep photo captions/titles to 90 characters or fewer. If this TikTok app is still in sandbox/unaudited mode, use SELF_ONLY privacy until app review is complete.",
  "error_code": "platform_request_invalid",
  "failure_stage": "publish",
  "platform_error_code": "invalid_params",
  "is_retriable": false,
  "next_action": "review_platform_options",
  "error_source": "platform",
  "error_temporality": "permanent",
  "provider_error": {
    "provider": "tiktok",
    "http_status": 400,
    "code": "invalid_params"
  },
  "retry_policy": {
    "is_retriable": false,
    "will_retry": false,
    "retry_state": "not_retriable",
    "next_run_at": null,
    "attempts_made": null,
    "max_attempts": null,
    "attempts_remaining": null,
    "manual_retry_allowed": true,
    "reason": "classification_not_retriable"
  }
}`;

export default function ApiErrorsPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "Errors" }]}
      section="api"
      title="API errors"
      description="UniPost errors are designed to be actionable without parsing provider payloads. Preserve the structured fields, show request_id in support surfaces, and use /v1/posts as the canonical publishing route family."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <DocSection id="envelope" title="Error envelope">
          <ParamTable params={ERROR_FIELDS} />
          <div style={{ marginTop: 18 }}>
            <CodeTabs snippets={[{ lang: "json", label: "Validation error", code: ERROR_EXAMPLE }]} />
          </div>
        </DocSection>

        <DocSection id="validation-issues" title="Validation issues">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Validation details appear in <code>error.issues</code> for failed create requests and in <code>data.errors</code> or <code>data.warnings</code> from <ApiInlineLink endpoint="POST /v1/posts/validate" />. Client UIs should render these fields directly instead of matching strings in <code>error.message</code>.
          </p>
          <ParamTable params={ISSUE_FIELDS} />
        </DocSection>

        <DocSection id="publish-failures" title="Publish failure fields">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Failed per-account results returned by <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> include structured failure fields. Use <code>error_source</code> to distinguish UniPost, worker, and official platform failures. Use <code>error_temporality</code> to distinguish temporary, permanent, and unknown conditions. Use <code>retry_policy.will_retry</code> to know whether UniPost has actually scheduled automatic retry; <code>is_retriable</code> only means retrying may help. Do not parse <code>error_message</code> for branching.
          </p>
          <CodeTabs snippets={[{ lang: "json", label: "Failed result", code: FAILED_RESULT_EXAMPLE }]} />
          <div style={{ marginTop: 18 }}>
            <ErrorTable
              errors={[
                { code: "validation_error", http: 400, description: "The request or post payload is invalid. Inspect issues and fix the listed fields." },
                { code: "platform_request_invalid", http: 400, description: "The provider rejected platform options or metadata. Review privacy, title, disclosure, or content options." },
                { code: "media_error", http: 400, description: "The provider rejected media format, dimensions, duration, URL, or processing state." },
                { code: "temporary_platform_error", http: 503, description: "The platform or async worker path failed transiently. Check retry_policy.will_retry to know whether UniPost scheduled retry." },
                { code: "rate_limit", http: 429, description: "Wait before retrying. Respect Retry-After when present." },
                { code: "x_monthly_usage_limit_exceeded", http: 402, description: "The workspace has reached its managed-X Credits hard limit for the current billing period. Show the reset date or an upgrade/contact path; do not retry-loop." },
                { code: "account_reconnect_required", http: 409, description: "Reconnect the affected social account before retrying." },
                { code: "missing_permission", http: 403, description: "Reconnect or update platform permissions and scopes." },
                { code: "target_not_found", http: 404, description: "The selected platform target no longer exists or is not visible to the account." },
                { code: "platform_error", http: 502, description: "A provider failure that UniPost cannot classify more specifically." },
                { code: "unknown_error", http: 500, description: "The failure is unknown. Contact support with request_id." },
              ]}
            />
          </div>
        </DocSection>

        <DocSection id="next-actions" title="next_action values">
          <ParamTable params={NEXT_ACTION_FIELDS} />
        </DocSection>

        <DocSection id="route-naming" title="Route naming">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            The canonical public publishing routes are <code>/v1/posts</code>, <code>/v1/posts/validate</code>, and <code>/v1/posts/:post_id</code>. Older client references to <code>/v1/social-posts</code> should be migrated to <code>/v1/posts</code>; new docs, CLI, MCP, and SDK examples use the canonical route names.
          </p>
        </DocSection>
      </div>
    </ApiReferencePage>
  );
}
