package socialconnectioncutover

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

type deploymentInventoryStub struct {
	deployments []railwaybackup.Deployment
	err         error
}

func (s deploymentInventoryStub) ActiveDeployments(context.Context, railwaybackup.DeploymentInventoryRequest) ([]railwaybackup.Deployment, error) {
	return s.deployments, s.err
}

type sessionReaderStub struct {
	sessions []RuntimeSession
	err      error
}

func (s sessionReaderStub) RuntimeSessions(context.Context) ([]RuntimeSession, error) {
	return s.sessions, s.err
}

func TestDeploymentProofRequiresExactSuccessfulServicesAndSessions(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	proof := DeploymentProof{
		Inventory: deploymentInventoryStub{deployments: []railwaybackup.Deployment{
			{ID: "api-deployment", ServiceID: "api-service", Status: "SUCCESS", CommitSHA: sha},
			{ID: "worker-deployment", ServiceID: "worker-service", Status: "SUCCESS", CommitSHA: sha},
		}},
		Sessions: sessionReaderStub{sessions: []RuntimeSession{
			{PID: 10, ApplicationName: "unipost:api:api-service:api-deployment:" + sha},
			{PID: 11, ApplicationName: "unipost:post-delivery-worker:worker-service:worker-deployment:" + sha},
		}},
		ProjectID: "project", EnvironmentID: "environment", CommitSHA: sha,
		RequiredServiceIDs: []string{"api-service", "worker-service"},
	}
	if err := proof.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDeploymentProofRejectsOldOrUnhealthyDeployment(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	proof := DeploymentProof{
		Inventory: deploymentInventoryStub{deployments: []railwaybackup.Deployment{
			{ID: "old", ServiceID: "api-service", Status: "SUCCESS", CommitSHA: "old-sha"},
			{ID: "building", ServiceID: "worker-service", Status: "BUILDING", CommitSHA: sha},
		}},
		Sessions: sessionReaderStub{}, ProjectID: "project", EnvironmentID: "environment", CommitSHA: sha,
		RequiredServiceIDs: []string{"api-service", "worker-service"},
	}
	err := proof.Verify(context.Background())
	if err == nil || !strings.Contains(err.Error(), "deployment") {
		t.Fatalf("Verify error = %v", err)
	}
}

func TestDeploymentProofRejectsUnknownOrOldDatabaseSession(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	for name, applicationName := range map[string]string{
		"old sha": "unipost:api:api-service:deployment:old",
		"unknown": "unipost:unknown:api-service:deployment:" + sha,
		"invalid": "unipost:not-enough-parts",
	} {
		t.Run(name, func(t *testing.T) {
			proof := DeploymentProof{
				Inventory: deploymentInventoryStub{deployments: []railwaybackup.Deployment{
					{ID: "deployment", ServiceID: "api-service", Status: "SUCCESS", CommitSHA: sha},
				}},
				Sessions:  sessionReaderStub{sessions: []RuntimeSession{{PID: 12, ApplicationName: applicationName}}},
				ProjectID: "project", EnvironmentID: "environment", CommitSHA: sha,
				RequiredServiceIDs: []string{"api-service"},
			}
			if err := proof.Verify(context.Background()); err == nil || !strings.Contains(err.Error(), "session") {
				t.Fatalf("Verify error = %v", err)
			}
		})
	}
}

func TestDeploymentProofPropagatesInventoryFailure(t *testing.T) {
	want := errors.New("Railway unavailable")
	proof := DeploymentProof{
		Inventory: deploymentInventoryStub{err: want}, Sessions: sessionReaderStub{},
		ProjectID: "project", EnvironmentID: "environment", CommitSHA: "sha",
		RequiredServiceIDs: []string{"api-service"},
	}
	if err := proof.Verify(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Verify error = %v, want %v", err, want)
	}
}
