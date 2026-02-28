package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFlyNewRequiresToken(t *testing.T) {
	_, err := NewFly(ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestFlyNewRequiresApp(t *testing.T) {
	_, err := NewFly(ProviderConfig{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for missing app")
	}
}

func TestFlyName(t *testing.T) {
	f, _ := NewFly(ProviderConfig{Token: "tok", Project: "app"})
	if f.Name() != "fly" {
		t.Errorf("Name() = %q, want %q", f.Name(), "fly")
	}
}

func TestFlyGetDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/machines/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := flyMachine{
			ID:    "machine-789",
			Name:  "web-1",
			State: "started",
			ImageRef: struct {
				Repository string `json:"repository"`
				Tag        string `json:"tag"`
				Digest     string `json:"digest"`
			}{
				Repository: "registry.fly.io/my-app",
				Tag:        "deployment-123",
				Digest:     "sha256:abcdef",
			},
			CreatedAt: "2025-01-01T00:00:00Z",
			UpdatedAt: "2025-01-01T00:02:00Z",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	f, _ := NewFly(ProviderConfig{Token: "tok", Project: "my-app"})
	f.client = srv.Client()
	f.client.Transport = rewriteTransport{url: srv.URL, orig: flyMachinesAPI}

	d, err := f.GetDeployment(context.Background(), "machine-789")
	if err != nil {
		t.Fatalf("GetDeployment() error: %v", err)
	}

	if d.ID != "machine-789" {
		t.Errorf("ID = %q, want %q", d.ID, "machine-789")
	}
	if d.Status != StatusSucceeded {
		t.Errorf("Status = %s, want SUCCEEDED (started)", d.Status)
	}
	if d.URL != "https://my-app.fly.dev" {
		t.Errorf("URL = %q, want %q", d.URL, "https://my-app.fly.dev")
	}
	if d.CommitSHA != "sha256:abcdef" {
		t.Errorf("CommitSHA = %q, want %q", d.CommitSHA, "sha256:abcdef")
	}
}

func TestFlyLatestDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		machines := []flyMachine{
			{ID: "m1", State: "started", UpdatedAt: "2025-01-01T00:00:00Z"},
			{ID: "m2", State: "starting", UpdatedAt: "2025-01-01T00:05:00Z"},
			{ID: "m3", State: "started", UpdatedAt: "2025-01-01T00:02:00Z"},
		}
		json.NewEncoder(w).Encode(machines)
	}))
	defer srv.Close()

	f, _ := NewFly(ProviderConfig{Token: "tok", Project: "app"})
	f.client = srv.Client()
	f.client.Transport = rewriteTransport{url: srv.URL, orig: flyMachinesAPI}

	d, err := f.LatestDeployment(context.Background())
	if err != nil {
		t.Fatalf("LatestDeployment() error: %v", err)
	}

	// Should pick the most recently updated machine (m2)
	if d.ID != "m2" {
		t.Errorf("ID = %q, want %q (most recently updated)", d.ID, "m2")
	}
	if d.Status != StatusDeploying {
		t.Errorf("Status = %s, want DEPLOYING (starting)", d.Status)
	}
}

func TestFlyNoMachines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]flyMachine{})
	}))
	defer srv.Close()

	f, _ := NewFly(ProviderConfig{Token: "tok", Project: "app"})
	f.client = srv.Client()
	f.client.Transport = rewriteTransport{url: srv.URL, orig: flyMachinesAPI}

	_, err := f.LatestDeployment(context.Background())
	if err != ErrNoDeployments {
		t.Errorf("error = %v, want ErrNoDeployments", err)
	}
}

func TestFlyStatusMapping(t *testing.T) {
	tests := []struct {
		raw  string
		want Status
	}{
		{"created", StatusPending},
		{"preparing", StatusPending},
		{"starting", StatusDeploying},
		{"started", StatusSucceeded},
		{"running", StatusSucceeded},
		{"stopping", StatusFailed},
		{"stopped", StatusFailed},
		{"destroying", StatusFailed},
		{"destroyed", StatusFailed},
		{"failed", StatusCrashed},
		{"other", StatusUnknown},
	}

	for _, tt := range tests {
		if got := flyStatus(tt.raw); got != tt.want {
			t.Errorf("flyStatus(%q) = %s, want %s", tt.raw, got, tt.want)
		}
	}
}
