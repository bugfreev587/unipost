package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var terminalPostEventSinks = [...]string{"webhook", "notification", "loops"}

// RecordPostStatusTransition persists the parent projection version while the
// caller holds the post advisory lock. Terminal transitions also create one
// independently retryable delivery ledger row per sink.
func RecordPostStatusTransition(
	ctx context.Context,
	queries *Queries,
	postID string,
	workspaceID string,
	parentStatus string,
	parentVersion string,
	eventType string,
	payload any,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		return err
	}
	body["parent_version"] = parentVersion
	if eventType == "post.published" {
		_, electionErr := queries.ElectWorkspaceFirstPostPublished(ctx, ElectWorkspaceFirstPostPublishedParams{
			WorkspaceID: workspaceID,
			PostID:      postID,
		})
		firstPost := true
		if errors.Is(electionErr, pgx.ErrNoRows) {
			firstPost = false
		} else if electionErr != nil {
			return electionErr
		}
		body["first_post_published"] = firstPost
	}
	encoded, err = json.Marshal(body)
	if err != nil {
		return err
	}
	event, err := queries.CreatePostStatusTransitionEvent(ctx, CreatePostStatusTransitionEventParams{
		PostID:        postID,
		WorkspaceID:   workspaceID,
		ParentStatus:  parentStatus,
		ParentVersion: parentVersion,
		EventType:     eventType,
		Payload:       encoded,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if eventType == "" {
		return nil
	}
	for _, sink := range terminalPostEventSinks {
		if err := queries.CreateTerminalPostEventDelivery(ctx, CreateTerminalPostEventDeliveryParams{
			EventID: event.ID,
			Sink:    sink,
		}); err != nil {
			return err
		}
	}
	return nil
}
