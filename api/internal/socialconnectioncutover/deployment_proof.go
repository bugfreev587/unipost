package socialconnectioncutover

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

type RuntimeSession struct {
	PID             int32
	ApplicationName string
}

type RuntimeSessionReader interface {
	RuntimeSessions(context.Context) ([]RuntimeSession, error)
}

type DeploymentProof struct {
	Inventory          railwaybackup.DeploymentInventoryClient
	Sessions           RuntimeSessionReader
	ProjectID          string
	EnvironmentID      string
	CommitSHA          string
	RequiredServiceIDs []string
}

func (p DeploymentProof) Verify(ctx context.Context) error {
	if p.Inventory == nil || p.Sessions == nil {
		return errors.New("Railway deployment inventory and database session reader are required")
	}
	if strings.TrimSpace(p.ProjectID) == "" || strings.TrimSpace(p.EnvironmentID) == "" || strings.TrimSpace(p.CommitSHA) == "" {
		return errors.New("Railway project, environment, and commit identity are required")
	}
	required := make(map[string]bool, len(p.RequiredServiceIDs))
	for _, serviceID := range p.RequiredServiceIDs {
		serviceID = strings.TrimSpace(serviceID)
		if serviceID == "" {
			return errors.New("required Railway service identity is empty")
		}
		required[serviceID] = false
	}
	if len(required) == 0 {
		return errors.New("at least one Railway runtime service is required")
	}

	deployments, err := p.Inventory.ActiveDeployments(ctx, railwaybackup.DeploymentInventoryRequest{
		ProjectID: p.ProjectID, EnvironmentID: p.EnvironmentID,
	})
	if err != nil {
		return fmt.Errorf("verify Railway deployment inventory: %w", err)
	}
	activeDeploymentIDs := make(map[string]map[string]bool)
	for _, deployment := range deployments {
		if _, relevant := required[deployment.ServiceID]; !relevant {
			continue
		}
		if deployment.Status != "SUCCESS" {
			return fmt.Errorf("Railway deployment %s for service %s is %s, not SUCCESS", deployment.ID, deployment.ServiceID, deployment.Status)
		}
		if deployment.CommitSHA != p.CommitSHA {
			return fmt.Errorf("Railway deployment %s for service %s does not match requested commit", deployment.ID, deployment.ServiceID)
		}
		required[deployment.ServiceID] = true
		if activeDeploymentIDs[deployment.ServiceID] == nil {
			activeDeploymentIDs[deployment.ServiceID] = make(map[string]bool)
		}
		activeDeploymentIDs[deployment.ServiceID][deployment.ID] = true
	}
	missing := make([]string, 0)
	for serviceID, found := range required {
		if !found {
			missing = append(missing, serviceID)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Railway deployment inventory is missing successful services: %s", strings.Join(missing, ", "))
	}

	sessions, err := p.Sessions.RuntimeSessions(ctx)
	if err != nil {
		return fmt.Errorf("verify database runtime sessions: %w", err)
	}
	allowedModes := map[string]bool{
		"api": true, "post-delivery-worker": true, "media-worker": true,
	}
	for _, session := range sessions {
		if !strings.HasPrefix(session.ApplicationName, "unipost:") {
			continue
		}
		parts := strings.Split(session.ApplicationName, ":")
		if len(parts) != 5 || !allowedModes[parts[1]] {
			return fmt.Errorf("database session %d has an unrecognized UniPost application identity", session.PID)
		}
		serviceID, deploymentID, commitSHA := parts[2], parts[3], parts[4]
		if commitSHA != p.CommitSHA {
			return fmt.Errorf("database session %d is running a different commit", session.PID)
		}
		if !activeDeploymentIDs[serviceID][deploymentID] {
			return fmt.Errorf("database session %d does not belong to a verified active deployment", session.PID)
		}
	}
	return nil
}

type PostgresRuntimeSessionReader struct {
	db Queryer
}

func NewPostgresRuntimeSessionReader(db Queryer) *PostgresRuntimeSessionReader {
	return &PostgresRuntimeSessionReader{db: db}
}

func (r *PostgresRuntimeSessionReader) RuntimeSessions(ctx context.Context) ([]RuntimeSession, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pid::INTEGER, application_name
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		  AND application_name LIKE 'unipost:%'
		ORDER BY pid
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make([]RuntimeSession, 0)
	for rows.Next() {
		var session RuntimeSession
		if err := rows.Scan(&session.PID, &session.ApplicationName); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}
