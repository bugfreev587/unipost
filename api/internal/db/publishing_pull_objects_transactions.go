package db

import "context"

// ClaimPublishingPullObjectsDue coordinates cleanup with reservations through
// the publishing object row lock. The active-usage check intentionally runs as
// a later statement in the same Read Committed transaction, after the lock is
// held, so it sees usages committed by a reservation that won the lock race.
func (q *Queries) ClaimPublishingPullObjectsDue(ctx context.Context, batchSize int32) ([]PublishingPullObject, error) {
	claimed := make([]PublishingPullObject, 0)
	err := q.WithTransaction(ctx, func(txQueries *Queries) error {
		candidates, err := txQueries.LockPublishingPullObjectCandidates(ctx, batchSize)
		if err != nil {
			return err
		}
		claimed = make([]PublishingPullObject, 0, len(candidates))
		for _, candidate := range candidates {
			hasActiveUsage, err := txQueries.PublishingPullObjectHasActiveUsage(ctx, candidate.ObjectKey)
			if err != nil {
				return err
			}
			if hasActiveUsage {
				continue
			}
			row, err := txQueries.MarkPublishingPullObjectDeleting(ctx, candidate.ObjectKey)
			if err != nil {
				return err
			}
			claimed = append(claimed, row)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}
