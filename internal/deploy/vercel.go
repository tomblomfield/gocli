package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const vercelAPI = "https://api.vercel.com"

// Vercel implements the Provider interface for Vercel deployments.
type Vercel struct {
	token   string
	project string
	team    string
	client  *http.Client
}

// NewVercel creates a new Vercel provider.
func NewVercel(cfg ProviderConfig) (*Vercel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("%w: VERCEL_TOKEN is required", ErrNotConfigured)
	}
	return &Vercel{
		token:   cfg.Token,
		project: cfg.Project,
		team:    cfg.Team,
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (v *Vercel) Name() string { return "vercel" }

type vercelDeployment struct {
	UID            string `json:"uid"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	State          string `json:"state"`
	ReadyState     string `json:"readyState"`
	Created        int64  `json:"created"`
	BuildingAt     int64  `json:"buildingAt"`
	Ready          int64  `json:"ready"`
	Target         string `json:"target"`
	GitSource      *struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Ref     string `json:"ref"`
	} `json:"gitSource"`
}

func (v *Vercel) doRequest(ctx context.Context, path string) ([]byte, error) {
	u := vercelAPI + path
	if v.team != "" {
		parsed, _ := url.Parse(u)
		q := parsed.Query()
		q.Set("teamId", v.team)
		parsed.RawQuery = q.Encode()
		u = parsed.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.token)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vercel API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vercel API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (v *Vercel) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	body, err := v.doRequest(ctx, "/v13/deployments/"+id)
	if err != nil {
		return nil, err
	}

	var vd vercelDeployment
	if err := json.Unmarshal(body, &vd); err != nil {
		return nil, fmt.Errorf("parse deployment: %w", err)
	}

	return v.toDeployment(&vd), nil
}

func (v *Vercel) LatestDeployment(ctx context.Context) (*Deployment, error) {
	path := "/v6/deployments?limit=1&sort=created"
	if v.project != "" {
		path += "&projectId=" + url.QueryEscape(v.project)
	}

	body, err := v.doRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Deployments []vercelDeployment `json:"deployments"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse deployments: %w", err)
	}

	if len(result.Deployments) == 0 {
		return nil, ErrNoDeployments
	}

	// Fetch full deployment details for the latest
	return v.GetDeployment(ctx, result.Deployments[0].UID)
}

func (v *Vercel) toDeployment(vd *vercelDeployment) *Deployment {
	d := &Deployment{
		ID:        vd.UID,
		Status:    vercelStatus(vd.ReadyState),
		Provider:  "vercel",
		Project:   vd.Name,
		RawStatus: vd.ReadyState,
	}
	if vd.URL != "" {
		d.URL = "https://" + vd.URL
	}
	d.Environment = vd.Target
	if vd.GitSource != nil {
		d.CommitSHA = vd.GitSource.SHA
		d.CommitMsg = vd.GitSource.Message
	}
	if vd.Created > 0 {
		d.CreatedAt = time.UnixMilli(vd.Created)
	}
	if vd.Ready > 0 {
		d.UpdatedAt = time.UnixMilli(vd.Ready)
	} else if vd.BuildingAt > 0 {
		d.UpdatedAt = time.UnixMilli(vd.BuildingAt)
	}
	return d
}

func vercelStatus(s string) Status {
	switch s {
	case "QUEUED", "INITIALIZING":
		return StatusPending
	case "BUILDING":
		return StatusBuilding
	case "DEPLOYING":
		return StatusDeploying
	case "READY":
		return StatusSucceeded
	case "ERROR":
		return StatusFailed
	case "CANCELED":
		return StatusCancelled
	default:
		return StatusUnknown
	}
}
