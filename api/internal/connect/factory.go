package connect

import "strings"

// NewManagedConnector builds a Connect OAuth connector for the given
// platform using the supplied client credentials. Callers use this
// when a workspace has uploaded white-label credentials and the
// managed Connect flow should run against that customer-owned app
// instead of UniPost's global env-var credentials.
func NewManagedConnector(platform, clientID, clientSecret, callbackBaseURL string) Connector {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "twitter":
		return NewTwitterConnector(clientID, clientSecret, callbackBaseURL)
	case "linkedin":
		return NewLinkedInConnector(clientID, clientSecret, callbackBaseURL)
	case "youtube":
		return NewYouTubeConnector(clientID, clientSecret, callbackBaseURL)
	case "instagram":
		return NewInstagramConnector(clientID, clientSecret, callbackBaseURL)
	case "tiktok":
		return NewTikTokConnector(clientID, clientSecret, callbackBaseURL)
	case "threads":
		return NewThreadsConnector(clientID, clientSecret, callbackBaseURL)
	default:
		return nil
	}
}
