package xinbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// TokenRefreshResolver resolves the exact OAuth client that originally
// authorized a persisted X account. It intentionally never infers app identity
// from connection_type or from the current presence of workspace credentials.
type TokenRefreshResolver struct {
	queries            *db.Queries
	encryptor          *crypto.AESEncryptor
	globalClientID     string
	globalClientSecret string
	callbackBaseURL    string
}

type TokenRefresher interface {
	Refresh(context.Context, db.SocialAccount, string) (*connect.TokenSet, error)
}

func NewTokenRefreshResolver(queries *db.Queries, encryptor *crypto.AESEncryptor, globalClientID, globalClientSecret, callbackBaseURL string) *TokenRefreshResolver {
	return &TokenRefreshResolver{
		queries:            queries,
		encryptor:          encryptor,
		globalClientID:     strings.TrimSpace(globalClientID),
		globalClientSecret: strings.TrimSpace(globalClientSecret),
		callbackBaseURL:    callbackBaseURL,
	}
}

func (r *TokenRefreshResolver) Resolve(ctx context.Context, account db.SocialAccount) (connect.Connector, error) {
	if r == nil {
		return nil, fmt.Errorf("X token refresh resolver is not configured")
	}
	if !strings.EqualFold(strings.TrimSpace(account.Platform), "twitter") {
		return nil, fmt.Errorf("X token refresh resolver cannot resolve platform %q", account.Platform)
	}
	mode, err := ParseAppMode(account.XAppMode.String)
	if err != nil || !account.XAppMode.Valid {
		if err == nil {
			err = fmt.Errorf("missing persisted X app mode")
		}
		return nil, err
	}
	switch mode {
	case AppModeUniPostManaged:
		connector := connect.NewTwitterConnector(r.globalClientID, r.globalClientSecret, r.callbackBaseURL)
		if connector == nil {
			return nil, fmt.Errorf("global X OAuth credentials are not configured")
		}
		return connector, nil
	case AppModeWorkspace:
		if r.queries == nil || r.encryptor == nil {
			return nil, fmt.Errorf("workspace X OAuth credential resolver is not configured")
		}
		profile, err := r.queries.GetProfile(ctx, account.ProfileID)
		if err != nil {
			return nil, fmt.Errorf("load X account profile: %w", err)
		}
		credential, err := r.queries.GetPlatformCredential(ctx, db.GetPlatformCredentialParams{
			WorkspaceID: profile.WorkspaceID,
			Platform:    "twitter",
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("workspace X OAuth credentials are missing")
			}
			return nil, fmt.Errorf("load workspace X OAuth credentials: %w", err)
		}
		clientSecret, err := r.encryptor.Decrypt(credential.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt workspace X OAuth client secret: %w", err)
		}
		connector := connect.NewTwitterConnector(credential.ClientID, clientSecret, r.callbackBaseURL)
		if connector == nil {
			return nil, fmt.Errorf("workspace X OAuth credentials are incomplete")
		}
		return connector, nil
	case AppModeLegacyUnknown:
		return nil, fmt.Errorf("legacy X app identity is ambiguous; reconnect required")
	default:
		return nil, fmt.Errorf("unsupported persisted X app mode %q", mode)
	}
}

func (r *TokenRefreshResolver) Refresh(ctx context.Context, account db.SocialAccount, refreshToken string) (*connect.TokenSet, error) {
	connector, err := r.Resolve(ctx, account)
	if err != nil {
		return nil, err
	}
	return connector.Refresh(ctx, refreshToken)
}
