package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const herokuAPI = "https://api.heroku.com"

// Heroku implements the Provider interface for Heroku deployments.
type Heroku struct {
	token  string
	app    string
	client *http.Client
}

// NewHeroku creates a new Heroku provider.
func NewHeroku(cfg ProviderConfig) (*Heroku, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("%w: HEROKU_API_KEY is required", ErrNotConfigured)
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf("%w: app name is required for Heroku", ErrNotConfigured)
	}
	return &Heroku{
		token:  cfg.Token,
		app:    cfg.Project,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (h *Heroku) Name() string { return "heroku" }

type herokuBuild struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	App       struct {
		Name string `json:"name"`
	} `json:"app"`
	SourceBlob struct {
		Commit        string `json:"commit"`
		CommitMessage string `json:"commit_message"`
		Version       string `json:"version"`
	} `json:"source_blob"`
	Release *struct {
		ID string `json:"id"`
	} `json:"release"`
	Slug *struct {
		ID string `json:"id"`
	} `json:"slug"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (h *Heroku) doRequest(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, herokuAPI+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Accept", "application/vnd.heroku+json; version=3")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("heroku API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heroku API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (h *Heroku) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	body, err := h.doRequest(ctx, "/apps/"+h.app+"/builds/"+id)
	if err != nil {
		return nil, err
	}

	var build herokuBuild
	if err := json.Unmarshal(body, &build); err != nil {
		return nil, fmt.Errorf("parse build: %w", err)
	}

	return h.toDeployment(&build), nil
}

func (h *Heroku) LatestDeployment(ctx context.Context) (*Deployment, error) {
	body, err := h.doRequest(ctx, "/apps/"+h.app+"/builds")
	if err != nil {
		return nil, err
	}

	var builds []herokuBuild
	if err := json.Unmarshal(body, &builds); err != nil {
		return nil, fmt.Errorf("parse builds: %w", err)
	}

	if len(builds) == 0 {
		return nil, ErrNoDeployments
	}

	// Heroku returns builds in chronological order; take the last one
	latest := builds[len(builds)-1]
	return h.toDeployment(&latest), nil
}

func (h *Heroku) toDeployment(b *herokuBuild) *Deployment {
	d := &Deployment{
		ID:        b.ID,
		Status:    herokuStatus(b.Status),
		Provider:  "heroku",
		Project:   b.App.Name,
		CommitSHA: b.SourceBlob.Commit,
		CommitMsg: b.SourceBlob.CommitMessage,
		RawStatus: b.Status,
	}
	if b.App.Name != "" {
		d.URL = "https://" + b.App.Name + ".herokuapp.com"
	}
	if t, err := time.Parse(time.RFC3339, b.CreatedAt); err == nil {
		d.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, b.UpdatedAt); err == nil {
		d.UpdatedAt = t
	}
	return d
}

func herokuStatus(s string) Status {
	switch s {
	case "pending":
		return StatusPending
	case "building":
		return StatusBuilding
	case "succeeded":
		return StatusSucceeded
	case "failed":
		return StatusFailed
	default:
		return StatusUnknown
	}
}
