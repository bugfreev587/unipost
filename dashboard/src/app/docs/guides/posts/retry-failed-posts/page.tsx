import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const RETRY_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `# Read the latest result and retry_policy snapshot.
curl -fSs "https://api.unipost.dev/v1/posts/post_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# After the blocker is resolved, retry the failed result once.
curl -fSs -X POST \\
  "https://api.unipost.dev/v1/posts/post_abc123/results/spr_failed_456/retry" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# Poll the parent post to follow the queued result and aggregate status.
curl -fSs "https://api.unipost.dev/v1/posts/post_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

export default function RetryFailedPostsGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Retry failed posts"
      lead="Decide whether UniPost will retry a failed destination automatically, whether a manual retry is available, and when to wait or repair the original publish inputs first."
      className="docs-page-wide"
    >
      <h2 id="automatic-vs-manual">Automatic and manual retry are different</h2>
      <p>
        Read <code>results[].retry_policy</code> instead of inferring retry behavior from an error message.
        Automatic retry means UniPost already has another attempt scheduled or running. Manual retry is a caller action
        that is available only after the result has failed and no delivery attempt remains active.
      </p>
      <DocsTable
        columns={["Signal", "Meaning", "Your action"]}
        rows={[
          ["retry_policy.will_retry = true", "An automatic retry is scheduled or running.", "Wait and poll the parent post. Do not call manual retry."],
          ["retry_policy.manual_retry_allowed = true", "The failed result can be submitted for a manual retry now, subject to a fresh policy and queue check.", "Resolve the original blocker, then submit one manual retry."],
          ["Both are false", "No automatic retry is active and manual retry is currently unavailable.", "Use retry_policy.reason and the latest result fields to decide what must change."],
        ]}
      />

      <h2 id="eligibility">When a failed result can be retried</h2>
      <p>
        Treat <code>retry_policy.manual_retry_allowed</code> as a best-effort snapshot. UniPost rechecks every condition
        when it receives the retry request, so the endpoint response is authoritative.
      </p>
      <DocsTable
        columns={["Check", "Retry can proceed", "Retry cannot proceed yet"]}
        rows={[
          ["Result state", "results[].status is failed.", "The result is not failed, the post already succeeded, or publishing is still processing."],
          ["Delivery job", "No active job exists.", "A pending, running, or retrying job already exists."],
          ["Social account", "The original account is still available.", "The account is disconnected or reconnect_required."],
          ["Media", "The original media is still uploaded and retained.", "The media was cleaned up or can no longer be resolved."],
          ["Plan and platform policy", "The current plan and platform policy allow publishing.", "The Free Plan TikTok restriction is active or publishing policy is unavailable."],
          ["Queue admission", "The request and one new queue job are admitted.", "Rate or queue-depth admission rejects the request; wait before trying again."],
        ]}
      />

      <div className="docs-callout docs-callout-tip">
        A result classified as non-retriable can still become manually retryable after an external condition changes.
        Check the current <code>manual_retry_allowed</code> value and fix the condition before submitting the request.
      </div>

      <h2 id="tiktok-capacity">TikTok reached_active_user_cap</h2>
      <p>
        In Production, <code>reached_active_user_cap</code> is currently classified as <code>is_retriable=false</code>, so
        UniPost does not automatically retry it. This avoids repeated requests while TikTok's active-user capacity is
        still full.
      </p>
      <p>
        On a Paid Plan, wait for the TikTok capacity window to recover, then call manual retry once. On the Free Plan,
        while the current TikTok restriction is active, UniPost returns 402 PLAN_PLATFORM_PUBLISHING_RESTRICTED and
        does not request TikTok. After the restriction is lifted or the workspace upgrades, a manual retry can proceed
        if the original media is still available.
      </p>
      <p>
        If TikTok returns the cap again after that manual attempt, the result fails again. UniPost does not continue
        with another automatic retry.
      </p>

      <h2 id="recommended-flow">Recommended retry flow</h2>
      <ol>
        <li><code>{"GET /v1/posts/{id}"}</code> and find the destination in <code>results[]</code>.</li>
        <li>
          Confirm <code>results[].status</code> is <code>failed</code> and inspect
          <code> retry_policy.manual_retry_allowed</code> and <code>retry_policy.reason</code>.
        </li>
        <li>
          Wait for the external condition to recover or reconnect the account. If the retained media is unavailable,
          upload it again and create a new publish request; newly uploaded media cannot be attached to the old result.
        </li>
        <li>
          If the original media is still retained and manual retry is allowed, POST the result retry endpoint once.
          There is no idempotency key, so do not repeat a successful request.
        </li>
        <li>Poll <code>{"GET /v1/posts/{id}"}</code> or your queue view until the destination reaches a terminal status.</li>
      </ol>
      <DocsCodeTabs snippets={RETRY_SNIPPETS} />

      <h2 id="responses">Key responses</h2>
      <DocsTable
        columns={["Status", "Code", "Meaning"]}
        rows={[
          ["200", "queued", "The retry was accepted. Poll the parent post; do not submit it again."],
          ["402", "PLAN_PLATFORM_PUBLISHING_RESTRICTED", "The current plan/platform publishing policy blocks this retry."],
          ["409", "RESULT_NOT_RETRYABLE", "The result is not currently failed."],
          ["409", "QUEUE_JOB_ACTIVE", "A pending, running, or retrying job already exists."],
          ["409", "SOCIAL_ACCOUNT_NOT_AVAILABLE", "The original connected account cannot publish right now."],
          ["409", "MEDIA_REUPLOAD_REQUIRED", "The retained media is no longer available."],
          ["429", "RATE_LIMITED", "Honor Retry-After and wait for request, enqueue, or queue-depth admission to recover."],
          ["503", "POLICY_UNAVAILABLE", "UniPost cannot safely evaluate the current publishing policy."],
        ]}
      />
    </DocsPage>
  );
}
