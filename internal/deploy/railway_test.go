package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRailwayNewRequiresToken(t *testing.T) {
	_, err := NewRailway(ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestRailwayNewRequiresProject(t *testing.T) {
	_, err := NewRailway(ProviderConfig{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestRailwayName(t *testing.T) {
	r, _ := NewRailway(ProviderConfig{Token: "tok", Project: "proj"})
	if r.Name() != "railway" {
		t.Errorf("Name() = %q, want %q", r.Name(), "railway")
	}
}

func TestRailwayGetDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{
			"data": map[string]any{
				"deployment": map[string]any{
					"id":        "dep-123",
					"status":    "SUCCESS",
					"staticUrl": "https://app.up.railway.app",
					"createdAt": "2025-01-01T00:00:00Z",
					"updatedAt": "2025-01-01T00:05:00Z",
					"meta": map[string]any{
						"commitHash":    "abc1234",
						"commitMessage": "fix: deploy",
					},
					"service":     map[string]any{"name": "web"},
					"environment": map[string]any{"name": "production"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r, _ := NewRailway(ProviderConfig{Token: "test-token", Project: "proj-1"})
	r.client = srv.Client()
	// Override the API URL by using a custom transport
	origURL := railwayAPI
	r.client.Transport = rewriteTransport{url: srv.URL, orig: origURL}

	d, err := r.GetDeployment(context.Background(), "dep-123")
	if err != nil {
		t.Fatalf("GetDeployment() error: %v", err)
	}

	if d.ID != "dep-123" {
		t.Errorf("ID = %q, want %q", d.ID, "dep-123")
	}
	if d.Status != StatusSucceeded {
		t.Errorf("Status = %s, want SUCCEEDED", d.Status)
	}
	if d.URL != "https://app.up.railway.app" {
		t.Errorf("URL = %q, want %q", d.URL, "https://app.up.railway.app")
	}
	if d.CommitSHA != "abc1234" {
		t.Errorf("CommitSHA = %q, want %q", d.CommitSHA, "abc1234")
	}
	if d.Environment != "production" {
		t.Errorf("Environment = %q, want %q", d.Environment, "production")
	}
}

func TestRailwayLatestDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"deployments": map[string]any{
					"edges": []map[string]any{
						{
							"node": map[string]any{
								"id":        "latest-1",
								"status":    "BUILDING",
								"staticUrl": "",
								"createdAt": "2025-01-01T00:00:00Z",
								"updatedAt": "2025-01-01T00:01:00Z",
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r, _ := NewRailway(ProviderConfig{Token: "tok", Project: "proj"})
	r.client = srv.Client()
	r.client.Transport = rewriteTransport{url: srv.URL, orig: railwayAPI}

	d, err := r.LatestDeployment(context.Background())
	if err != nil {
		t.Fatalf("LatestDeployment() error: %v", err)
	}

	if d.ID != "latest-1" {
		t.Errorf("ID = %q, want %q", d.ID, "latest-1")
	}
	if d.Status != StatusBuilding {
		t.Errorf("Status = %s, want BUILDING", d.Status)
	}
}

func TestRailwayNoDeployments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"deployments": map[string]any{
					"edges": []any{},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r, _ := NewRailway(ProviderConfig{Token: "tok", Project: "proj"})
	r.client = srv.Client()
	r.client.Transport = rewriteTransport{url: srv.URL, orig: railwayAPI}

	_, err := r.LatestDeployment(context.Background())
	if err != ErrNoDeployments {
		t.Errorf("error = %v, want ErrNoDeployments", err)
	}
}

func TestRailwayStatusMapping(t *testing.T) {
	tests := []struct {
		raw  string
		want Status
	}{
		{"INITIALIZING", StatusPending},
		{"QUEUED", StatusPending},
		{"WAITING", StatusPending},
		{"BUILDING", StatusBuilding},
		{"DEPLOYING", StatusDeploying},
		{"SUCCESS", StatusSucceeded},
		{"READY", StatusSucceeded},
		{"FAILED", StatusFailed},
		{"ERROR", StatusFailed},
		{"CANCELLED", StatusCancelled},
		{"CANCELED", StatusCancelled},
		{"CRASHED", StatusCrashed},
		{"SOMETHING_ELSE", StatusUnknown},
	}

	for _, tt := range tests {
		if got := railwayStatus(tt.raw); got != tt.want {
			t.Errorf("railwayStatus(%q) = %s, want %s", tt.raw, got, tt.want)
		}
	}
}

// rewriteTransport rewrites requests to point at the test server.
type rewriteTransport struct {
	url  string
	orig string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.url[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}
