package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHerokuNewRequiresToken(t *testing.T) {
	_, err := NewHeroku(ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestHerokuNewRequiresApp(t *testing.T) {
	_, err := NewHeroku(ProviderConfig{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for missing app")
	}
}

func TestHerokuName(t *testing.T) {
	h, _ := NewHeroku(ProviderConfig{Token: "tok", Project: "app"})
	if h.Name() != "heroku" {
		t.Errorf("Name() = %q, want %q", h.Name(), "heroku")
	}
}

func TestHerokuGetDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/builds/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.heroku+json; version=3" {
			t.Error("missing Heroku Accept header")
		}

		resp := herokuBuild{
			ID:     "build-456",
			Status: "succeeded",
			App:    struct{ Name string `json:"name"` }{Name: "my-heroku-app"},
			SourceBlob: struct {
				Commit        string `json:"commit"`
				CommitMessage string `json:"commit_message"`
				Version       string `json:"version"`
			}{
				Commit:        "abc123def",
				CommitMessage: "deploy: fix the bug",
			},
			CreatedAt: "2025-01-01T00:00:00Z",
			UpdatedAt: "2025-01-01T00:03:00Z",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	h, _ := NewHeroku(ProviderConfig{Token: "tok", Project: "my-heroku-app"})
	h.client = srv.Client()
	h.client.Transport = rewriteTransport{url: srv.URL, orig: herokuAPI}

	d, err := h.GetDeployment(context.Background(), "build-456")
	if err != nil {
		t.Fatalf("GetDeployment() error: %v", err)
	}

	if d.ID != "build-456" {
		t.Errorf("ID = %q, want %q", d.ID, "build-456")
	}
	if d.Status != StatusSucceeded {
		t.Errorf("Status = %s, want SUCCEEDED", d.Status)
	}
	if d.CommitSHA != "abc123def" {
		t.Errorf("CommitSHA = %q, want %q", d.CommitSHA, "abc123def")
	}
	if d.URL != "https://my-heroku-app.herokuapp.com" {
		t.Errorf("URL = %q, want %q", d.URL, "https://my-heroku-app.herokuapp.com")
	}
}

func TestHerokuLatestDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		builds := []herokuBuild{
			{ID: "old-build", Status: "succeeded", App: struct{ Name string `json:"name"` }{Name: "app"}},
			{ID: "latest-build", Status: "building", App: struct{ Name string `json:"name"` }{Name: "app"}},
		}
		json.NewEncoder(w).Encode(builds)
	}))
	defer srv.Close()

	h, _ := NewHeroku(ProviderConfig{Token: "tok", Project: "app"})
	h.client = srv.Client()
	h.client.Transport = rewriteTransport{url: srv.URL, orig: herokuAPI}

	d, err := h.LatestDeployment(context.Background())
	if err != nil {
		t.Fatalf("LatestDeployment() error: %v", err)
	}

	if d.ID != "latest-build" {
		t.Errorf("ID = %q, want %q", d.ID, "latest-build")
	}
	if d.Status != StatusBuilding {
		t.Errorf("Status = %s, want BUILDING", d.Status)
	}
}

func TestHerokuNoDeployments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]herokuBuild{})
	}))
	defer srv.Close()

	h, _ := NewHeroku(ProviderConfig{Token: "tok", Project: "app"})
	h.client = srv.Client()
	h.client.Transport = rewriteTransport{url: srv.URL, orig: herokuAPI}

	_, err := h.LatestDeployment(context.Background())
	if err != ErrNoDeployments {
		t.Errorf("error = %v, want ErrNoDeployments", err)
	}
}

func TestHerokuStatusMapping(t *testing.T) {
	tests := []struct {
		raw  string
		want Status
	}{
		{"pending", StatusPending},
		{"building", StatusBuilding},
		{"succeeded", StatusSucceeded},
		{"failed", StatusFailed},
		{"other", StatusUnknown},
	}

	for _, tt := range tests {
		if got := herokuStatus(tt.raw); got != tt.want {
			t.Errorf("herokuStatus(%q) = %s, want %s", tt.raw, got, tt.want)
		}
	}
}
