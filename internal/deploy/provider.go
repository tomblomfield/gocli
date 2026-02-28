// Package deploy provides deployment status monitoring across cloud providers.
package deploy

import (
	"context"
	"fmt"
	"time"
)

// Status represents the current state of a deployment.
type Status int

const (
	StatusUnknown    Status = iota
	StatusPending           // Queued / waiting to start
	StatusBuilding          // Build in progress
	StatusDeploying         // Deploying (post-build)
	StatusSucceeded         // Deployment completed successfully
	StatusFailed            // Deployment failed
	StatusCancelled         // Deployment was cancelled
	StatusCrashed           // Deployed but crashed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusBuilding:
		return "BUILDING"
	case StatusDeploying:
		return "DEPLOYING"
	case StatusSucceeded:
		return "SUCCEEDED"
	case StatusFailed:
		return "FAILED"
	case StatusCancelled:
		return "CANCELLED"
	case StatusCrashed:
		return "CRASHED"
	default:
		return "UNKNOWN"
	}
}

// Terminal returns true if this status represents a final state.
func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusCrashed:
		return true
	default:
		return false
	}
}

// Success returns true if this status represents a successful outcome.
func (s Status) Success() bool {
	return s == StatusSucceeded
}

// Deployment holds information about a single deployment.
type Deployment struct {
	ID          string
	Status      Status
	Provider    string
	Project     string
	Environment string
	URL         string
	CommitSHA   string
	CommitMsg   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RawStatus   string // Provider-specific status string
}

// Provider is the interface that deployment providers must implement.
type Provider interface {
	// Name returns the provider name (e.g. "railway", "vercel").
	Name() string

	// GetDeployment fetches a specific deployment by ID.
	GetDeployment(ctx context.Context, id string) (*Deployment, error)

	// LatestDeployment fetches the most recent deployment for the configured project.
	LatestDeployment(ctx context.Context) (*Deployment, error)
}

// ProviderConfig holds common configuration for providers.
type ProviderConfig struct {
	Token       string
	Project     string
	Service     string // Railway-specific
	Environment string // Railway-specific
	Team        string // Vercel-specific
}

// ErrNoDeployments is returned when no deployments are found.
var ErrNoDeployments = fmt.Errorf("no deployments found")

// ErrNotConfigured is returned when a required configuration is missing.
var ErrNotConfigured = fmt.Errorf("provider not configured")
