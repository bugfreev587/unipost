"use client";

import { DocsTable } from "../../../_components/docs-shell";
import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Parent UniPost post ID.",
  },
  {
    name: "resultID",
    type: "string",
    description: "Failed per-destination result ID from the parent post's results array.",
  },
];

const SUCCESS_FIELDS: ApiFieldItem[] = [
  { name: "data.id", type: "string", description: "The existing result ID." },
  { name: "data.social_account_id", type: "string", description: "Destination social account ID." },
  { name: "data.platform", type: "string", description: "Destination platform." },
  { name: "data.status", type: "string", description: '"queued" after the retry job is accepted.' },
  { name: "data.retry_policy", type: "object", description: "Current best-effort queue and retry snapshot." },
  { name: "data.retry_policy.retry_state", type: "string", description: "Usually scheduled after a successful enqueue." },
  { name: "data.retry_policy.will_retry", type: "boolean", description: "Whether UniPost has an active retry attempt." },
  { name: "data.retry_policy.manual_retry_allowed", type: "boolean", description: "False while the accepted retry job is active." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Stable uppercase error code." },
  { name: "error.normalized_code", type: "string", description: "Lowercase error-code alias." },
  { name: "error.message", type: "string", description: "Human-readable reason the retry was rejected." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const REQUEST_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST \\
  "https://api.unipost.dev/v1/posts/post_abc123/results/spr_failed_456/retry" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "spr_failed_456",
    "social_account_id": "sa_tiktok_789",
    "platform": "tiktok",
    "status": "queued",
    "retry_policy": {
      "is_retriable": false,
      "will_retry": true,
      "retry_state": "scheduled",
      "manual_retry_allowed": false
    }
  }
}`,
  },
  {
    lang: "json",
    label: "402",
    code: `{
  "error": {
    "code": "PLAN_PLATFORM_PUBLISHING_RESTRICTED",
    "normalized_code": "plan_platform_publishing_restricted",
    "message": "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.",
    "next_action": "upgrade_or_wait_then_retry",
    "is_retriable": false,
    "error_source": "unipost",
    "error_temporality": "temporary",
    "details": {
      "platform": "tiktok",
      "plan_id": "free"
    }
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "429",
    code: `{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "queue_depth_exceeded",
    "message": "This workspace already has too many queued deliveries. Wait for the queue to drain before creating more posts.",
    "hint": "Wait for the Retry-After window to pass, then retry the request.",
    "next_action": "wait_and_retry",
    "is_retriable": true
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "QUEUE_JOB_ACTIVE",
    "normalized_code": "queue_job_active",
    "message": "An active delivery job already exists for this result."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "503",
    code: `{
  "error": {
    "code": "POLICY_UNAVAILABLE",
    "normalized_code": "policy_unavailable",
    "message": "Publishing policy is temporarily unavailable."
  },
  "request_id": "req_123"
}`,
  },
];

export default function RetryPostReferencePage() {
  return (
    <SingleEndpointReferencePage
      section="Posts"
      title="Retry Post"
      description="Queue one new delivery attempt for a failed per-destination result."
      method="POST"
      path="/v1/posts/{id}/results/{resultID}/retry"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Parameters", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: SUCCESS_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "429", fields: ERROR_FIELDS },
        { code: "503", fields: ERROR_FIELDS },
      ]}
      snippets={REQUEST_SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
      guideLinks={[
        { label: "Retry failed posts", href: "/docs/guides/posts/retry-failed-posts" },
      ]}
    >
      <section>
        <h2>No request body</h2>
        <p>
          Send the Authorization header and the two path parameters only. Do not send JSON or an idempotency header.
        </p>
      </section>

      <section>
        <h2>Behavior</h2>
        <p>
          A retry reuses the same result record and does not create a new result row. After UniPost accepts the
          job, this endpoint returns the result with status queued. Fetch the parent with{" "}
          <code>{"GET /v1/posts/{id}"}</code> to follow the result and the aggregate post status.
        </p>
        <p>
          This endpoint has no idempotency key. After a successful response, do not call it again for the same
          attempt. If a pending, running, or retrying job already exists, UniPost returns 409 QUEUE_JOB_ACTIVE.
        </p>
      </section>

      <section>
        <h2>Error responses</h2>
        <DocsTable
          columns={["Status", "Code", "What to do"]}
          rows={[
            ["402", "PLAN_PLATFORM_PUBLISHING_RESTRICTED", "Wait until the current plan/platform restriction is lifted, or use an eligible plan."],
            ["409", "RESULT_NOT_RETRYABLE", "Only a result whose current status is failed can be retried."],
            ["409", "QUEUE_JOB_ACTIVE", "Do not retry again. Poll the parent post while the active job runs."],
            ["409", "SOCIAL_ACCOUNT_NOT_AVAILABLE", "Reconnect the destination account before publishing again."],
            ["409", "MEDIA_REUPLOAD_REQUIRED", "Upload the original media again, then create a new publish request."],
            ["429", "RATE_LIMITED", "Honor Retry-After, then retry after the request, enqueue, or queue-depth window recovers."],
            ["503", "POLICY_UNAVAILABLE", "Retry later after UniPost can evaluate the current publishing policy."],
          ]}
        />
      </section>
    </SingleEndpointReferencePage>
  );
}
