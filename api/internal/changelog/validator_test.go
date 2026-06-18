package changelog

import "testing"

func validPayload() CandidatePayload {
	return CandidatePayload{
		HasCandidate: true,
		Candidate: Candidate{
			ID:             "developer-logs-api",
			Date:           "2026-06-18",
			DisplayDate:    "June 18, 2026",
			Title:          "Developer Logs API",
			Summary:        "Workspace-scoped developer logs are available over REST and SSE.",
			Category:       CategoryReliability,
			Impact:         ImpactNew,
			IsBreaking:     false,
			WhyUserVisible: "Developers can inspect delivery and API logs without support help.",
			Links: []Link{
				{Label: "Logs docs", Href: "/docs/api/logs"},
			},
			SourceLinks: []Link{
				{Label: "Release PR", Href: "https://github.com/bugfreev587/unipost/pull/67"},
			},
			Confidence: "high",
		},
	}
}

func TestValidateCandidateRequiresObjectiveEvidence(t *testing.T) {
	payload := validPayload()
	payload.Candidate.SourceLinks = nil

	err := ValidateCandidate(payload)
	if err == nil {
		t.Fatal("ValidateCandidate returned nil, want missing sourceLinks error")
	}
}

func TestValidateCandidateRejectsSDKJSAndAgentPostAsSDK(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version SDKVersion
	}{
		{
			name: "sdk-js package",
			version: SDKVersion{
				Ecosystem:      EcosystemNPM,
				PackageName:    "@unipost/sdk-js",
				Version:        "0.4.1",
				Href:           "https://www.npmjs.com/package/@unipost/sdk-js",
				InstallCommand: "npm install @unipost/sdk-js@0.4.1",
			},
		},
		{
			name: "agentpost package",
			version: SDKVersion{
				Ecosystem:      EcosystemNPM,
				PackageName:    "@unipost/agentpost",
				Version:        "0.2.0",
				Href:           "https://www.npmjs.com/package/@unipost/agentpost",
				InstallCommand: "npm install @unipost/agentpost@0.2.0",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := validPayload()
			payload.Candidate.Category = CategorySDK
			payload.Candidate.SDKVersions = []SDKVersion{tc.version}

			err := ValidateCandidate(payload)
			if err == nil {
				t.Fatal("ValidateCandidate returned nil, want SDK guardrail error")
			}
		})
	}
}

func TestValidateCandidateAllowsOptionalInstallCommand(t *testing.T) {
	payload := validPayload()
	payload.Candidate.Category = CategorySDK
	payload.Candidate.SDKVersions = []SDKVersion{
		{
			Ecosystem:   EcosystemNPM,
			PackageName: "@unipost/sdk",
			Version:     "0.4.1",
			Href:        "https://www.npmjs.com/package/@unipost/sdk/v/0.4.1",
		},
	}

	if err := ValidateCandidate(payload); err != nil {
		t.Fatalf("ValidateCandidate returned %v, want nil", err)
	}
}

func TestNormalizeSourceHashIsStableAndOrderIndependent(t *testing.T) {
	first := NormalizeSourceHash([]string{"pr:72", "commit:abc", "commit:def"})
	second := NormalizeSourceHash([]string{"commit:def", "pr:72", "commit:abc"})

	if first == "" {
		t.Fatal("NormalizeSourceHash returned empty hash")
	}
	if first != second {
		t.Fatalf("NormalizeSourceHash order mismatch: %q != %q", first, second)
	}
}
