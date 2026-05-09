package integrationlogs

import "strings"

var internalPathPrefixes = []string{
	"/v1/me",
	"/v1/admin",
	"/v1/audit-log",
	"/v1/api-metrics",
	"/v1/billing",
	"/v1/plans",
	"/v1/members",
	"/v1/invites",
	"/v1/inbox",
	"/v1/logs",
	"/v1/post-delivery-jobs",
	"/v1/oauth",
	"/v1/connect/callback",
	"/v1/public",
	"/v1/meta",
	"/v1/ai",
	"/v1/platforms",
	"/v1/usage",
	"/v1/limits",
	"/v1/platform-credentials",
}

var internalPostsCommandSuffixes = []string{
	"/queue",
	"/preview-link",
	"/archive",
	"/restore",
	"/cancel",
}

// IsInternalRequestPath reports whether an HTTP request path belongs to a
// dashboard- or admin-only surface that customers should not see in the
// workspace Logs view. Public REST endpoints documented under /docs/api
// return false.
func IsInternalRequestPath(p string) bool {
	if p == "/v1/posts/summaries" {
		return true
	}
	for _, prefix := range internalPathPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	if strings.HasPrefix(p, "/v1/posts/") {
		for _, suffix := range internalPostsCommandSuffixes {
			if strings.HasSuffix(p, suffix) {
				return true
			}
		}
	}
	if strings.HasPrefix(p, "/v1/profiles/") {
		rest := strings.TrimPrefix(p, "/v1/profiles/")
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) >= 2 {
			switch parts[1] {
			case "users", "accounts", "oauth":
				return true
			}
		}
	}
	return false
}
