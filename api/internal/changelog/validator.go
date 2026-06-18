package changelog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

func ValidateCandidate(payload CandidatePayload) error {
	if !payload.HasCandidate {
		return fmt.Errorf("%w: hasCandidate must be true", ErrCandidateInvalid)
	}
	c := payload.Candidate
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrCandidateInvalid)
	}
	if _, err := time.Parse("2006-01-02", c.Date); err != nil {
		return fmt.Errorf("%w: date must be YYYY-MM-DD", ErrCandidateInvalid)
	}
	for name, value := range map[string]string{
		"title":          c.Title,
		"summary":        c.Summary,
		"whyUserVisible": c.WhyUserVisible,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s is required", ErrCandidateInvalid, name)
		}
	}
	if !validCategory(c.Category) {
		return fmt.Errorf("%w: unsupported category %q", ErrCandidateInvalid, c.Category)
	}
	if !validImpact(c.Impact) {
		return fmt.Errorf("%w: unsupported impact %q", ErrCandidateInvalid, c.Impact)
	}
	if len(c.SourceLinks) == 0 {
		return fmt.Errorf("%w: sourceLinks are required", ErrCandidateInvalid)
	}
	for _, link := range append(append([]Link{}, c.Links...), c.SourceLinks...) {
		if err := validateLink(link); err != nil {
			return err
		}
	}
	for _, sdk := range c.SDKVersions {
		if err := validateSDKVersion(sdk); err != nil {
			return err
		}
	}
	return nil
}

func validCategory(category Category) bool {
	switch category {
	case CategoryAPI, CategorySDK, CategoryDashboard, CategoryPlatform, CategoryDX, CategoryReliability:
		return true
	default:
		return false
	}
}

func validImpact(impact Impact) bool {
	switch impact {
	case ImpactNew, ImpactImproved, ImpactChanged, ImpactFixed:
		return true
	default:
		return false
	}
}

func validateLink(link Link) error {
	if strings.TrimSpace(link.Label) == "" || strings.TrimSpace(link.Href) == "" {
		return fmt.Errorf("%w: links require label and href", ErrCandidateInvalid)
	}
	href := strings.TrimSpace(link.Href)
	if strings.HasPrefix(href, "/") {
		return nil
	}
	parsed, err := url.Parse(href)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return fmt.Errorf("%w: invalid link %q", ErrCandidateInvalid, href)
	}
	return nil
}

func validateSDKVersion(version SDKVersion) error {
	if strings.EqualFold(strings.TrimSpace(version.PackageName), "@unipost/sdk-js") {
		return fmt.Errorf("%w: JavaScript SDK package is @unipost/sdk, not @unipost/sdk-js", ErrCandidateInvalid)
	}
	if strings.EqualFold(strings.TrimSpace(version.PackageName), "@unipost/agentpost") {
		return fmt.Errorf("%w: AgentPost is not an SDK release", ErrCandidateInvalid)
	}
	if !validEcosystem(version.Ecosystem) {
		return fmt.Errorf("%w: unsupported SDK ecosystem %q", ErrCandidateInvalid, version.Ecosystem)
	}
	if strings.TrimSpace(version.PackageName) == "" || strings.TrimSpace(version.Version) == "" || strings.TrimSpace(version.Href) == "" {
		return fmt.Errorf("%w: SDK versions require packageName, version, and href", ErrCandidateInvalid)
	}
	return validateLink(Link{Label: string(version.Ecosystem), Href: version.Href})
}

func validEcosystem(ecosystem Ecosystem) bool {
	switch ecosystem {
	case EcosystemNPM, EcosystemPip, EcosystemGo, EcosystemMaven:
		return true
	default:
		return false
	}
}

func NormalizeSourceHash(parts []string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	sort.Strings(cleaned)
	sum := sha256.Sum256([]byte(strings.Join(cleaned, "\n")))
	return hex.EncodeToString(sum[:])
}
