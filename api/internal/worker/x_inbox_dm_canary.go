package worker

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func ParseXInboxDMCanary(raw string) (map[string]struct{}, error) {
	canary := make(map[string]struct{})
	if strings.TrimSpace(raw) == "" {
		return canary, nil
	}

	for index, member := range strings.Split(raw, ",") {
		member = strings.TrimSpace(member)
		if member == "" {
			return map[string]struct{}{}, fmt.Errorf("X DM canary member %d is empty", index+1)
		}

		id, err := uuid.Parse(member)
		if err != nil {
			return map[string]struct{}{}, fmt.Errorf("parse X DM canary member %d: %w", index+1, err)
		}
		canary[id.String()] = struct{}{}
	}

	return canary, nil
}
