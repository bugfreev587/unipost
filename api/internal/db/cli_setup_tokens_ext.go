package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type CLISetupToken struct {
	ID          string             `json:"id"`
	WorkspaceID string             `json:"workspace_id"`
	UserID      string             `json:"user_id"`
	TokenHash   string             `json:"token_hash"`
	Client      string             `json:"client"`
	KeyName     string             `json:"key_name"`
	ExpiresAt   pgtype.Timestamptz `json:"expires_at"`
	UsedAt      pgtype.Timestamptz `json:"used_at"`
	RevokedAt   pgtype.Timestamptz `json:"revoked_at"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

type CreateCLISetupTokenParams struct {
	ID          string             `json:"id"`
	WorkspaceID string             `json:"workspace_id"`
	UserID      string             `json:"user_id"`
	TokenHash   string             `json:"token_hash"`
	Client      string             `json:"client"`
	KeyName     string             `json:"key_name"`
	ExpiresAt   pgtype.Timestamptz `json:"expires_at"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

type MarkCLISetupTokenUsedParams struct {
	ID     string             `json:"id"`
	UsedAt pgtype.Timestamptz `json:"used_at"`
}

const createCLISetupToken = `
INSERT INTO cli_setup_tokens (id, workspace_id, user_id, token_hash, client, key_name, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workspace_id, user_id, token_hash, client, key_name, expires_at, used_at, revoked_at, created_at
`

func (q *Queries) CreateCLISetupToken(ctx context.Context, arg CreateCLISetupTokenParams) (CLISetupToken, error) {
	row := q.db.QueryRow(ctx, createCLISetupToken,
		arg.ID,
		arg.WorkspaceID,
		arg.UserID,
		arg.TokenHash,
		arg.Client,
		arg.KeyName,
		arg.ExpiresAt,
		arg.CreatedAt,
	)
	return scanCLISetupToken(row)
}

const getCLISetupTokenByHash = `
SELECT id, workspace_id, user_id, token_hash, client, key_name, expires_at, used_at, revoked_at, created_at
FROM cli_setup_tokens
WHERE token_hash = $1
`

func (q *Queries) GetCLISetupTokenByHash(ctx context.Context, tokenHash string) (CLISetupToken, error) {
	row := q.db.QueryRow(ctx, getCLISetupTokenByHash, tokenHash)
	return scanCLISetupToken(row)
}

const markCLISetupTokenUsed = `
UPDATE cli_setup_tokens
SET used_at = $2
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL
RETURNING id, workspace_id, user_id, token_hash, client, key_name, expires_at, used_at, revoked_at, created_at
`

func (q *Queries) MarkCLISetupTokenUsed(ctx context.Context, arg MarkCLISetupTokenUsedParams) (CLISetupToken, error) {
	row := q.db.QueryRow(ctx, markCLISetupTokenUsed, arg.ID, arg.UsedAt)
	return scanCLISetupToken(row)
}

type cliSetupTokenRow interface {
	Scan(dest ...any) error
}

func scanCLISetupToken(row cliSetupTokenRow) (CLISetupToken, error) {
	var token CLISetupToken
	err := row.Scan(
		&token.ID,
		&token.WorkspaceID,
		&token.UserID,
		&token.TokenHash,
		&token.Client,
		&token.KeyName,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	return token, err
}
