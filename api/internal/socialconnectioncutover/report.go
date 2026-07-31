package socialconnectioncutover

import (
	"encoding/json"
	"sort"
	"time"
)

type Counts struct {
	Accounts           int64 `json:"accounts"`
	Connections        int64 `json:"connections"`
	PublishableNull    int64 `json:"publishable_null_bindings"`
	MissingIGIdentity  int64 `json:"missing_instagram_identity"`
	ActiveDeliveryJobs int64 `json:"active_delivery_jobs"`
	ConflictGroups     int64 `json:"conflict_groups"`
	AliasWarnings      int64 `json:"alias_warnings"`
}

type Conflict struct {
	WorkspaceID      string         `json:"workspace_id"`
	Platform         string         `json:"platform"`
	ProviderIdentity string         `json:"provider_identity,omitempty"`
	Reason           string         `json:"reason"`
	AccountIDs       []string       `json:"account_ids"`
	ProfileIDs       []string       `json:"profile_ids,omitempty"`
	ExternalUserIDs  []string       `json:"external_user_ids,omitempty"`
	ConnectionTypes  []string       `json:"connection_types,omitempty"`
	Classification   map[string]int `json:"classification_counts,omitempty"`
}

type AliasWarning struct {
	WorkspaceID        string   `json:"workspace_id"`
	Platform           string   `json:"platform"`
	ConnectionIDs      []string `json:"connection_ids,omitempty"`
	ProviderIdentities []string `json:"provider_identities"`
	SharedIdentifiers  []string `json:"shared_secondary_identifiers"`
}

type RelationSize struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	Rows  int64  `json:"estimated_rows"`
}

type Blocker struct {
	Code    string `json:"code"`
	Count   int64  `json:"count"`
	Message string `json:"message"`
}

type Report struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	Mode          string         `json:"mode"`
	Counts        Counts         `json:"counts"`
	Conflicts     []Conflict     `json:"conflicts"`
	AliasWarnings []AliasWarning `json:"alias_warnings"`
	Relations     []RelationSize `json:"relations"`
	Blockers      []Blocker      `json:"blockers"`
}

func (r Report) HasBlockers() bool {
	return len(r.Blockers) != 0
}

func (r Report) MarshalJSON() ([]byte, error) {
	for index := range r.Conflicts {
		sort.Strings(r.Conflicts[index].AccountIDs)
		sort.Strings(r.Conflicts[index].ProfileIDs)
		sort.Strings(r.Conflicts[index].ExternalUserIDs)
		sort.Strings(r.Conflicts[index].ConnectionTypes)
	}
	sort.Slice(r.Conflicts, func(i, j int) bool {
		left, right := r.Conflicts[i], r.Conflicts[j]
		if left.WorkspaceID != right.WorkspaceID {
			return left.WorkspaceID < right.WorkspaceID
		}
		if left.Platform != right.Platform {
			return left.Platform < right.Platform
		}
		if left.ProviderIdentity != right.ProviderIdentity {
			return left.ProviderIdentity < right.ProviderIdentity
		}
		return left.Reason < right.Reason
	})
	for index := range r.AliasWarnings {
		sort.Strings(r.AliasWarnings[index].ConnectionIDs)
		sort.Strings(r.AliasWarnings[index].ProviderIdentities)
		sort.Strings(r.AliasWarnings[index].SharedIdentifiers)
	}
	sort.Slice(r.AliasWarnings, func(i, j int) bool {
		left, right := r.AliasWarnings[i], r.AliasWarnings[j]
		if left.WorkspaceID != right.WorkspaceID {
			return left.WorkspaceID < right.WorkspaceID
		}
		return left.Platform < right.Platform
	})
	sort.Slice(r.Relations, func(i, j int) bool { return r.Relations[i].Name < r.Relations[j].Name })
	sort.Slice(r.Blockers, func(i, j int) bool { return r.Blockers[i].Code < r.Blockers[j].Code })
	type reportAlias Report
	return json.Marshal(reportAlias(r))
}
