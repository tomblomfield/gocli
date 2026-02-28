package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const railwayAPI = "https://backboard.railway.app/graphql/v2"

// Railway implements the Provider interface for Railway deployments.
type Railway struct {
	token       string
	projectID   string
	serviceID   string
	environment string
	client      *http.Client
}

// NewRailway creates a new Railway provider.
func NewRailway(cfg ProviderConfig) (*Railway, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("%w: RAILWAY_TOKEN is required", ErrNotConfigured)
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf("%w: project ID is required for Railway", ErrNotConfigured)
	}
	return &Railway{
		token:       cfg.Token,
		projectID:   cfg.Project,
		serviceID:   cfg.Service,
		environment: cfg.Environment,
		client:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (r *Railway) Name() string { return "railway" }

type railwayGQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type railwayDeploymentNode struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"staticUrl"`
	Meta   *struct {
		CommitHash    string `json:"commitHash"`
		CommitMessage string `json:"commitMessage"`
	} `json:"meta"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Service   *struct {
		Name string `json:"name"`
	} `json:"service"`
	Environment *struct {
		Name string `json:"name"`
	} `json:"environment"`
}

func (r *Railway) doGraphQL(ctx context.Context, gql railwayGQLRequest, target any) error {
	body, err := json.Marshal(gql)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, railwayAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("railway API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("railway API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("railway GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return json.Unmarshal(gqlResp.Data, target)
}

func (r *Railway) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	query := `query($id: String!) {
		deployment(id: $id) {
			id status staticUrl createdAt updatedAt
			meta { commitHash commitMessage }
			service { name }
			environment { name }
		}
	}`

	var result struct {
		Deployment railwayDeploymentNode `json:"deployment"`
	}
	err := r.doGraphQL(ctx, railwayGQLRequest{
		Query:     query,
		Variables: map[string]any{"id": id},
	}, &result)
	if err != nil {
		return nil, err
	}

	return r.nodeToDeployment(&result.Deployment), nil
}

func (r *Railway) LatestDeployment(ctx context.Context) (*Deployment, error) {
	query := `query($input: DeploymentListInput!) {
		deployments(input: $input) {
			edges { node {
				id status staticUrl createdAt updatedAt
				meta { commitHash commitMessage }
				service { name }
				environment { name }
			}}
		}
	}`

	input := map[string]any{
		"projectId": r.projectID,
	}
	if r.serviceID != "" {
		input["serviceId"] = r.serviceID
	}
	if r.environment != "" {
		input["environmentId"] = r.environment
	}

	var result struct {
		Deployments struct {
			Edges []struct {
				Node railwayDeploymentNode `json:"node"`
			} `json:"edges"`
		} `json:"deployments"`
	}
	err := r.doGraphQL(ctx, railwayGQLRequest{
		Query:     query,
		Variables: map[string]any{"input": input},
	}, &result)
	if err != nil {
		return nil, err
	}

	if len(result.Deployments.Edges) == 0 {
		return nil, ErrNoDeployments
	}

	return r.nodeToDeployment(&result.Deployments.Edges[0].Node), nil
}

func (r *Railway) nodeToDeployment(node *railwayDeploymentNode) *Deployment {
	d := &Deployment{
		ID:        node.ID,
		Status:    railwayStatus(node.Status),
		Provider:  "railway",
		Project:   r.projectID,
		URL:       node.URL,
		RawStatus: node.Status,
	}
	if node.Meta != nil {
		d.CommitSHA = node.Meta.CommitHash
		d.CommitMsg = node.Meta.CommitMessage
	}
	if node.Service != nil {
		d.Project = node.Service.Name
	}
	if node.Environment != nil {
		d.Environment = node.Environment.Name
	}
	if t, err := time.Parse(time.RFC3339, node.CreatedAt); err == nil {
		d.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, node.UpdatedAt); err == nil {
		d.UpdatedAt = t
	}
	return d
}

func railwayStatus(s string) Status {
	switch s {
	case "INITIALIZING", "QUEUED", "WAITING":
		return StatusPending
	case "BUILDING":
		return StatusBuilding
	case "DEPLOYING":
		return StatusDeploying
	case "SUCCESS", "READY":
		return StatusSucceeded
	case "FAILED", "ERROR":
		return StatusFailed
	case "CANCELLED", "CANCELED":
		return StatusCancelled
	case "CRASHED":
		return StatusCrashed
	default:
		return StatusUnknown
	}
}
