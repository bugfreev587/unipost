package ws

import "github.com/xiaoboyu/unipost-api/internal/db"

func LogEnvelope(row db.IntegrationLog) map[string]any {
	log := map[string]any{
		"id":           row.ID,
		"workspace_id": row.WorkspaceID,
		"ts":           row.Ts.Time,
		"level":        row.Level,
		"status":       row.Status,
		"category":     row.Category,
		"action":       row.Action,
		"source":       row.Source,
		"message":      row.Message,
	}
	if row.RequestID.Valid {
		log["request_id"] = row.RequestID.String
	}
	if row.TraceID.Valid {
		log["trace_id"] = row.TraceID.String
	}
	if row.ActorUserID.Valid {
		log["actor_user_id"] = row.ActorUserID.String
	}
	if row.ActorApiKeyID.Valid {
		log["actor_api_key_id"] = row.ActorApiKeyID.String
	}
	if row.ProfileID.Valid {
		log["profile_id"] = row.ProfileID.String
	}
	if row.SocialAccountID.Valid {
		log["social_account_id"] = row.SocialAccountID.String
	}
	if row.PostID.Valid {
		log["post_id"] = row.PostID.String
	}
	if row.PlatformPostID.Valid {
		log["platform_post_id"] = row.PlatformPostID.String
	}
	if row.Platform.Valid {
		log["platform"] = row.Platform.String
	}
	if row.Endpoint.Valid {
		log["endpoint"] = row.Endpoint.String
	}
	if row.Method.Valid {
		log["method"] = row.Method.String
	}
	if row.HTTPStatusCode.Valid {
		log["http_status_code"] = row.HTTPStatusCode.Int32
	}
	if row.RemoteStatusCode.Valid {
		log["remote_status_code"] = row.RemoteStatusCode.Int32
	}
	if row.DurationMs.Valid {
		log["duration_ms"] = row.DurationMs.Int32
	}
	if row.ErrorCode.Valid {
		log["error_code"] = row.ErrorCode.String
	}

	return map[string]any{
		"type":         "logs.new",
		"workspace_id": row.WorkspaceID,
		"log":          log,
	}
}
