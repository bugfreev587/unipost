package integrationlogs

import "testing"

func TestIsInternalRequestPath(t *testing.T) {
	cases := []struct {
		path     string
		internal bool
	}{
		// Internal — dashboard / admin surfaces.
		{"/v1/me", true},
		{"/v1/me/bootstrap", true},
		{"/v1/me/tutorials/welcome/complete", true},
		{"/v1/admin/stats", true},
		{"/v1/admin/logs/42", true},
		{"/v1/audit-log", true},
		{"/v1/api-metrics/summary", true},
		{"/v1/billing", true},
		{"/v1/billing/checkout", true},
		{"/v1/plans", true},
		{"/v1/members", true},
		{"/v1/members/u_1/role", true},
		{"/v1/invites/abc/accept", true},
		{"/v1/inbox/ws", true},
		{"/v1/logs", true},
		{"/v1/logs/9", true},
		{"/v1/logs/ws", true},
		{"/v1/post-delivery-jobs", true},
		{"/v1/post-delivery-jobs/j_1/retry", true},
		{"/v1/posts/post_1/queue", true},
		{"/v1/posts/post_1/archive", true},
		{"/v1/posts/post_1/restore", true},
		{"/v1/posts/post_1/cancel", true},
		{"/v1/posts/post_1/preview-link", true},
		{"/v1/posts/summaries", true},
		{"/v1/oauth/callback/twitter", true},
		{"/v1/connect/callback/linkedin", true},
		{"/v1/public/drafts/abc", true},
		{"/v1/meta/data-deletion", true},
		{"/v1/ai/post-assist", true},
		{"/v1/platforms/capabilities", true},
		{"/v1/usage", true},
		{"/v1/limits", true},
		{"/v1/platform-credentials", true},
		{"/v1/profiles/p_1/users", true},
		{"/v1/profiles/p_1/users/u_1", true},
		{"/v1/profiles/p_1/accounts", true},
		{"/v1/profiles/p_1/accounts/sa_1/metrics", true},
		{"/v1/profiles/p_1/oauth/connect/twitter", true},

		// Public — must remain visible.
		{"/v1/posts", false},
		{"/v1/posts/post_1", false},
		{"/v1/posts/validate", false},
		{"/v1/posts/bulk", false},
		{"/v1/posts/post_1/publish", false},
		{"/v1/posts/post_1/analytics", false},
		{"/v1/posts/post_1/results/r_1/retry", false},
		{"/v1/accounts", false},
		{"/v1/accounts/sa_1", false},
		{"/v1/accounts/connect", false},
		{"/v1/accounts/sa_1/capabilities", false},
		{"/v1/accounts/sa_1/health", false},
		{"/v1/accounts/sa_1/metrics", false},
		{"/v1/accounts/sa_1/tiktok/creator-info", false},
		{"/v1/profiles", false},
		{"/v1/profiles/p_1", false},
		{"/v1/api-keys", false},
		{"/v1/api-keys/k_1", false},
		{"/v1/webhooks", false},
		{"/v1/webhooks/w_1/rotate", false},
		{"/v1/connect/sessions", false},
		{"/v1/connect/sessions/cs_1", false},
		{"/v1/media", false},
		{"/v1/media/m_1", false},
		{"/v1/users", false},
		{"/v1/users/external_user_1", false},
		{"/v1/workspace", false},
		{"/v1/analytics/summary", false},
		{"/v1/analytics/rollup", false},
	}

	for _, tc := range cases {
		got := IsInternalRequestPath(tc.path)
		if got != tc.internal {
			t.Errorf("IsInternalRequestPath(%q) = %v, want %v", tc.path, got, tc.internal)
		}
	}
}
