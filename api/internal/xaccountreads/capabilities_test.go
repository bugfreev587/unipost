package xaccountreads

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

func TestAccountReadCapabilitiesRequireOfflineAccess(t *testing.T) {
	capabilities := EvaluateCapabilities(CapabilityInput{
		AccountStatus: "active", Scopes: []string{"users.read", "tweet.read"},
		RefreshAvailable: true, AppMode: xinbox.AppModeUniPostManaged,
	})
	if capabilities.ProfileRead.Authorized || capabilities.OwnPostHistoryRead.Authorized {
		t.Fatalf("capabilities=%+v", capabilities)
	}
}

func TestWorkspaceAccountReadCapabilitiesRequireOAuthClientCredentials(t *testing.T) {
	input := CapabilityInput{
		AccountStatus: "active", Scopes: []string{"users.read", "tweet.read", "offline.access"},
		RefreshAvailable: true, AppMode: xinbox.AppModeWorkspace,
	}
	withoutCredentials := EvaluateCapabilities(input)
	if withoutCredentials.ProfileRead.Authorized || withoutCredentials.OwnPostHistoryRead.Authorized {
		t.Fatalf("without credentials=%+v", withoutCredentials)
	}
	input.AppCredentialsAvailable = true
	withCredentials := EvaluateCapabilities(input)
	if !withCredentials.ProfileRead.Authorized || !withCredentials.OwnPostHistoryRead.Authorized {
		t.Fatalf("with credentials=%+v", withCredentials)
	}
}
