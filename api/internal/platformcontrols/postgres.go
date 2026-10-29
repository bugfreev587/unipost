package platformcontrols

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const controlColumns = `id, platform, state, reason_code, user_message, version, state_changed_at, updated_by_user_id, created_at, updated_at`

func scanControl(row pgx.Row) (Control, error) {
	var control Control
	var state string
	var actorID *string
	err := row.Scan(
		&control.ID,
		&control.Platform,
		&state,
		&control.ReasonCode,
		&control.UserMessage,
		&control.Version,
		&control.StateChangedAt,
		&actorID,
		&control.CreatedAt,
		&control.UpdatedAt,
	)
	if actorID != nil {
		control.UpdatedByUserID = *actorID
	}
	control.State = State(state)
	return control, err
}

func (s *PostgresStore) ControlForPlatform(ctx context.Context, platform string) (Control, error) {
	return scanControl(s.pool.QueryRow(ctx, `SELECT `+controlColumns+` FROM platform_operational_controls WHERE platform=$1`, platform))
}

func (s *PostgresStore) List(ctx context.Context) ([]Control, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+controlColumns+` FROM platform_operational_controls ORDER BY platform`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	controls := make([]Control, 0, len(SupportedPlatforms))
	for rows.Next() {
		control, err := scanControl(rows)
		if err != nil {
			return nil, err
		}
		controls = append(controls, control)
	}
	return controls, rows.Err()
}

func (s *PostgresStore) SetState(ctx context.Context, request TransitionRequest) (TransitionResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TransitionResult{}, err
	}
	defer tx.Rollback(ctx)

	current, err := scanControl(tx.QueryRow(ctx, `SELECT `+controlColumns+` FROM platform_operational_controls WHERE platform=$1 FOR UPDATE`, request.Platform))
	if err != nil {
		return TransitionResult{}, err
	}
	if current.Version != request.ExpectedVersion {
		return TransitionResult{}, &VersionConflictError{Current: current}
	}
	reasonCode := request.ReasonCode
	if reasonCode == "" {
		reasonCode = current.ReasonCode
	}
	userMessage := request.UserMessage
	if userMessage == "" {
		userMessage = current.UserMessage
	}
	if current.State == request.State && current.ReasonCode == reasonCode && current.UserMessage == userMessage {
		if err := tx.Commit(ctx); err != nil {
			return TransitionResult{}, err
		}
		return TransitionResult{Control: current, Changed: false}, nil
	}

	beforeJSON, err := json.Marshal(current)
	if err != nil {
		return TransitionResult{}, err
	}
	now := time.Now().UTC()
	updated, err := scanControl(tx.QueryRow(ctx, `
		UPDATE platform_operational_controls
		SET state=$2, reason_code=$3, user_message=$4, version=version+1,
		    state_changed_at=$5, updated_by_user_id=NULLIF($6,''), updated_at=$5
		WHERE platform=$1
		RETURNING `+controlColumns,
		request.Platform, request.State, reasonCode, userMessage, now, request.ActorUserID))
	if err != nil {
		return TransitionResult{}, err
	}
	afterJSON, err := json.Marshal(updated)
	if err != nil {
		return TransitionResult{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO platform_operational_control_events (
			control_id, platform, event_type, actor_user_id,
			expected_version, resulting_version, before_state, after_state,
			request_id, actor_ip, actor_user_agent
		) VALUES ($1,$2,'state_changed',NULLIF($3,''),$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''))
	`, updated.ID, updated.Platform, request.ActorUserID, request.ExpectedVersion, updated.Version,
		beforeJSON, afterJSON, request.RequestID, request.ActorIP, request.ActorUserAgent)
	if err != nil {
		return TransitionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Control: updated, Changed: true}, nil
}

func IsMissingControl(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func SafeUserMessage(control Control) string {
	message := strings.TrimSpace(control.UserMessage)
	if message == "" {
		return "This platform is temporarily unavailable."
	}
	return message
}
