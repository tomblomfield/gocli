package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVercelNewRequiresToken(t *testing.T) {
	_, err := NewVercel(ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestVercelName(t *testing.T) {
	v, _ := NewVercel(ProviderConfig{Token: "tok"})
	if v.Name() != "vercel" {
		t.Errorf("Name() = %q, want %q", v.Name(), "vercel")
	}
}

func TestVercelGetDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v13/deployments/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := vercelDeployment{
			UID:        "dpl_abc123",
			Name:       "my-app",
			URL:        "my-app-abc123.vercel.app",
			ReadyState: "READY",
			Target:     "production",
			Created:    1704067200000,
			Ready:      1704067500000,
			GitSource: &struct {
				SHA     string `json:"sha"`
				Message string `json:"message"`
				Ref     string `json:"ref"`
			}{
				SHA:     "deadbeef",
				Message: "deploy fix",
				Ref:     "main",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	v, _ := NewVercel(ProviderConfig{Token: "tok"})
	v.client = srv.Client()
	v.client.Transport = rewriteTransport{url: srv.URL, orig: vercelAPI}

	d, err := v.GetDeployment(context.Background(), "dpl_abc123")
	if err != nil {
		t.Fatalf("GetDeployment() error: %v", err)
	}

	if d.ID != "dpl_abc123" {
		t.Errorf("ID = %q, want %q", d.ID, "dpl_abc123")
	}
	if d.Status != StatusSucceeded {
		t.Errorf("Status = %s, want SUCCEEDED", d.Status)
	}
	if d.URL != "https://my-app-abc123.vercel.app" {
		t.Errorf("URL = %q, want %q", d.URL, "https://my-app-abc123.vercel.app")
	}
	if d.CommitSHA != "deadbeef" {
		t.Errorf("CommitSHA = %q, want %q", d.CommitSHA, "deadbeef")
	}
	if d.Environment != "production" {
		t.Errorf("Environment = %q, want %q", d.Environment, "production")
	}
}

func TestVercelLatestDeployment(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.HasPrefix(r.URL.Path, "/v6/deployments") {
			resp := map[string]any{
				"deployments": []map[string]any{
					{"uid": "dpl_latest", "name": "app", "readyState": "BUILDING", "created": 1704067200000},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Detail fetch
		resp := vercelDeployment{
			UID:        "dpl_latest",
			Name:       "app",
			ReadyState: "BUILDING",
			Created:    1704067200000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	v, _ := NewVercel(ProviderConfig{Token: "tok", Project: "proj"})
	v.client = srv.Client()
	v.client.Transport = rewriteTransport{url: srv.URL, orig: vercelAPI}

	d, err := v.LatestDeployment(context.Background())
	if err != nil {
		t.Fatalf("LatestDeployment() error: %v", err)
	}

	if d.ID != "dpl_latest" {
		t.Errorf("ID = %q, want %q", d.ID, "dpl_latest")
	}
	if d.Status != StatusBuilding {
		t.Errorf("Status = %s, want BUILDING", d.Status)
	}
}

func TestVercelNoDeployments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"deployments": []any{}})
	}))
	defer srv.Close()

	v, _ := NewVercel(ProviderConfig{Token: "tok"})
	v.client = srv.Client()
	v.client.Transport = rewriteTransport{url: srv.URL, orig: vercelAPI}

	_, err := v.LatestDeployment(context.Background())
	if err != ErrNoDeployments {
		t.Errorf("error = %v, want ErrNoDeployments", err)
	}
}

func TestVercelStatusMapping(t *testing.T) {
	tests := []struct {
		raw  string
		want Status
	}{
		{"QUEUED", StatusPending},
		{"INITIALIZING", StatusPending},
		{"BUILDING", StatusBuilding},
		{"DEPLOYING", StatusDeploying},
		{"READY", StatusSucceeded},
		{"ERROR", StatusFailed},
		{"CANCELED", StatusCancelled},
		{"OTHER", StatusUnknown},
	}

	for _, tt := range tests {
		if got := vercelStatus(tt.raw); got != tt.want {
			t.Errorf("vercelStatus(%q) = %s, want %s", tt.raw, got, tt.want)
		}
	}
}

func TestVercelTeamHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		teamID := r.URL.Query().Get("teamId")
		if teamID != "team_123" {
			t.Errorf("teamId = %q, want %q", teamID, "team_123")
		}
		json.NewEncoder(w).Encode(vercelDeployment{
			UID:        "dpl_1",
			ReadyState: "READY",
		})
	}))
	defer srv.Close()

	v, _ := NewVercel(ProviderConfig{Token: "tok", Team: "team_123"})
	v.client = srv.Client()
	v.client.Transport = rewriteTransport{url: srv.URL, orig: vercelAPI}

	v.GetDeployment(context.Background(), "dpl_1")
}
