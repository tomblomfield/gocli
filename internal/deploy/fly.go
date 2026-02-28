package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const flyMachinesAPI = "https://api.machines.dev"

// Fly implements the Provider interface for Fly.io deployments.
// It uses the Machines API to track deployment status.
type Fly struct {
	token  string
	app    string
	client *http.Client
}

// NewFly creates a new Fly.io provider.
func NewFly(cfg ProviderConfig) (*Fly, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("%w: FLY_API_TOKEN is required", ErrNotConfigured)
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf("%w: app name is required for Fly", ErrNotConfigured)
	}
	return &Fly{
		token:  cfg.Token,
		app:    cfg.Project,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (f *Fly) Name() string { return "fly" }

type flyMachine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Region    string `json:"region"`
	ImageRef  struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Digest     string `json:"digest"`
	} `json:"image_ref"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Events    []struct {
		Type      string `json:"type"`
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	} `json:"events"`
}

type flyRelease struct {
	ID          string `json:"id"`
	Version     int    `json:"version"`
	Status      string `json:"status"`
	Description string `json:"description"`
	ImageRef    string `json:"image_ref"`
	CreatedAt   string `json:"created_at"`
}

func (f *Fly) doRequest(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, flyMachinesAPI+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fly API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fly API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (f *Fly) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	body, err := f.doRequest(ctx, "/v1/apps/"+f.app+"/machines/"+id)
	if err != nil {
		return nil, err
	}

	var machine flyMachine
	if err := json.Unmarshal(body, &machine); err != nil {
		return nil, fmt.Errorf("parse machine: %w", err)
	}

	return f.machineToDeployment(&machine), nil
}

func (f *Fly) LatestDeployment(ctx context.Context) (*Deployment, error) {
	body, err := f.doRequest(ctx, "/v1/apps/"+f.app+"/machines")
	if err != nil {
		return nil, err
	}

	var machines []flyMachine
	if err := json.Unmarshal(body, &machines); err != nil {
		return nil, fmt.Errorf("parse machines: %w", err)
	}

	if len(machines) == 0 {
		return nil, ErrNoDeployments
	}

	// Find the most recently updated machine
	latest := &machines[0]
	for i := range machines {
		if machines[i].UpdatedAt > latest.UpdatedAt {
			latest = &machines[i]
		}
	}

	return f.machineToDeployment(latest), nil
}

func (f *Fly) machineToDeployment(m *flyMachine) *Deployment {
	d := &Deployment{
		ID:        m.ID,
		Status:    flyStatus(m.State),
		Provider:  "fly",
		Project:   f.app,
		URL:       "https://" + f.app + ".fly.dev",
		RawStatus: m.State,
	}
	if m.ImageRef.Digest != "" {
		d.CommitSHA = m.ImageRef.Digest
	}
	if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
		d.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, m.UpdatedAt); err == nil {
		d.UpdatedAt = t
	}
	return d
}

func flyStatus(s string) Status {
	switch s {
	case "created", "preparing":
		return StatusPending
	case "starting":
		return StatusDeploying
	case "started", "running":
		return StatusSucceeded
	case "stopping", "stopped", "destroying", "destroyed":
		return StatusFailed
	case "failed":
		return StatusCrashed
	default:
		return StatusUnknown
	}
}
