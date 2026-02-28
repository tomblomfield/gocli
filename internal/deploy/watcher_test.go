package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockProvider is a test helper that returns a sequence of deployments.
type mockProvider struct {
	name        string
	deployments []*Deployment
	callCount   int
	err         error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) GetDeployment(_ context.Context, id string) (*Deployment, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callCount >= len(m.deployments) {
		return m.deployments[len(m.deployments)-1], nil
	}
	d := m.deployments[m.callCount]
	m.callCount++
	return d, nil
}

func (m *mockProvider) LatestDeployment(_ context.Context) (*Deployment, error) {
	return m.GetDeployment(nil, "")
}

func TestWatchSuccess(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusPending, Provider: "test"},
			{ID: "d1", Status: StatusBuilding, Provider: "test"},
			{ID: "d1", Status: StatusDeploying, Provider: "test"},
			{ID: "d1", Status: StatusSucceeded, Provider: "test", URL: "https://app.test.dev"},
		},
	}

	var buf bytes.Buffer
	result, err := Watch(context.Background(), mock, WatchConfig{
		Interval: 1 * time.Millisecond,
		Timeout:  5 * time.Second,
		Writer:   &buf,
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	if result.Deployment.Status != StatusSucceeded {
		t.Errorf("status = %s, want SUCCEEDED", result.Deployment.Status)
	}
	if result.Polls != 4 {
		t.Errorf("polls = %d, want 4", result.Polls)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}

	// Verify status updates were written
	output := buf.String()
	if !strings.Contains(output, "PENDING") {
		t.Error("output should contain PENDING status")
	}
	if !strings.Contains(output, "SUCCEEDED") {
		t.Error("output should contain SUCCEEDED status")
	}
}

func TestWatchFailure(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusBuilding, Provider: "test"},
			{ID: "d1", Status: StatusFailed, Provider: "test"},
		},
	}

	var buf bytes.Buffer
	result, err := Watch(context.Background(), mock, WatchConfig{
		Interval: 1 * time.Millisecond,
		Timeout:  5 * time.Second,
		Writer:   &buf,
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	if result.Deployment.Status != StatusFailed {
		t.Errorf("status = %s, want FAILED", result.Deployment.Status)
	}
	if result.Deployment.Status.Success() {
		t.Error("failed deployment should not be Success()")
	}
}

func TestWatchSpecificDeployment(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "specific-123", Status: StatusSucceeded, Provider: "test"},
		},
	}

	result, err := Watch(context.Background(), mock, WatchConfig{
		Interval:     1 * time.Millisecond,
		Timeout:      5 * time.Second,
		DeploymentID: "specific-123",
		Writer:       &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	if result.Deployment.ID != "specific-123" {
		t.Errorf("deployment ID = %s, want specific-123", result.Deployment.ID)
	}
}

func TestWatchTimeout(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusBuilding, Provider: "test"},
		},
	}

	_, err := Watch(context.Background(), mock, WatchConfig{
		Interval: 1 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
		Writer:   &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("Watch() should error on timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want timeout error", err)
	}
}

func TestWatchContextCancelled(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusBuilding, Provider: "test"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := Watch(ctx, mock, WatchConfig{
		Interval: 1 * time.Millisecond,
		Timeout:  5 * time.Second,
		Writer:   &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("Watch() should error on cancelled context")
	}
}

func TestWatchTransientErrors(t *testing.T) {
	// Provider returns an error then succeeds
	callCount := 0
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusSucceeded, Provider: "test"},
		},
	}
	// Override to return an error on first call
	origLatest := mock.deployments
	mock.deployments = nil
	mock.err = fmt.Errorf("network error")

	var buf bytes.Buffer
	go func() {
		time.Sleep(10 * time.Millisecond)
		// Fix the mock after first error
		mock.err = nil
		mock.deployments = origLatest
		_ = callCount
	}()

	result, err := Watch(context.Background(), mock, WatchConfig{
		Interval: 5 * time.Millisecond,
		Timeout:  2 * time.Second,
		Writer:   &buf,
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	if result.Deployment.Status != StatusSucceeded {
		t.Errorf("status = %s, want SUCCEEDED", result.Deployment.Status)
	}

	// Should have logged the error
	if !strings.Contains(buf.String(), "ERR") {
		t.Error("output should contain error indication")
	}
}

func TestWatchJSONOutput(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusPending, Provider: "test"},
			{ID: "d1", Status: StatusSucceeded, Provider: "test", URL: "https://app.test.dev"},
		},
	}

	var buf bytes.Buffer
	_, err := Watch(context.Background(), mock, WatchConfig{
		Interval:   1 * time.Millisecond,
		Timeout:    5 * time.Second,
		JSONOutput: true,
		Writer:     &buf,
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	// Each line should be valid JSON
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %q", i, line)
		}
		if _, ok := obj["status"]; !ok {
			t.Errorf("line %d missing 'status' field", i)
		}
		if _, ok := obj["provider"]; !ok {
			t.Errorf("line %d missing 'provider' field", i)
		}
	}
}

func TestWatchCancelled(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		deployments: []*Deployment{
			{ID: "d1", Status: StatusPending, Provider: "test"},
			{ID: "d1", Status: StatusCancelled, Provider: "test"},
		},
	}

	result, err := Watch(context.Background(), mock, WatchConfig{
		Interval: 1 * time.Millisecond,
		Timeout:  5 * time.Second,
		Writer:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	if result.Deployment.Status != StatusCancelled {
		t.Errorf("status = %s, want CANCELLED", result.Deployment.Status)
	}
	if result.Deployment.Status.Success() {
		t.Error("cancelled deployment should not be Success()")
	}
}

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"PENDING", "..."},
		{"BUILDING", ">>>"},
		{"DEPLOYING", ">>>"},
		{"SUCCEEDED", "[OK]"},
		{"FAILED", "[FAIL]"},
		{"CRASHED", "[FAIL]"},
		{"CANCELLED", "[CANCEL]"},
		{"error", "[ERR]"},
		{"SOMETHING", "[?]"},
	}

	for _, tt := range tests {
		if got := statusSymbol(tt.status); got != tt.want {
			t.Errorf("statusSymbol(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		if got := truncate(tt.s, tt.n); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
