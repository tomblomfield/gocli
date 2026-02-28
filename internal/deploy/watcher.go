package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// WatchConfig configures the deployment watcher.
type WatchConfig struct {
	Interval     time.Duration
	Timeout      time.Duration
	DeploymentID string // If empty, watches latest deployment
	JSONOutput   bool
	Writer       io.Writer // Status updates go here (typically stderr)
}

// WatchResult is the final result of a watch operation.
type WatchResult struct {
	Deployment *Deployment   `json:"deployment"`
	Duration   time.Duration `json:"duration"`
	Polls      int           `json:"polls"`
}

// Watch polls a provider for deployment status until a terminal state is reached.
// It writes progress updates to cfg.Writer and returns the final result.
func Watch(ctx context.Context, provider Provider, cfg WatchConfig) (*WatchResult, error) {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}
	if cfg.Writer == nil {
		cfg.Writer = io.Discard
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	start := time.Now()
	polls := 0
	var lastStatus Status

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	// Do an immediate first poll
	fetch := func() (*Deployment, error) {
		polls++
		if cfg.DeploymentID != "" {
			return provider.GetDeployment(ctx, cfg.DeploymentID)
		}
		return provider.LatestDeployment(ctx)
	}

	for {
		d, err := fetch()
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("timed out after %s waiting for deployment", cfg.Timeout)
			}
			// Log transient errors but keep polling
			writeStatus(cfg.Writer, cfg.JSONOutput, "error", provider.Name(), "", err.Error())
			goto wait
		}

		if d.Status != lastStatus {
			elapsed := time.Since(start).Truncate(time.Second)
			writeStatus(cfg.Writer, cfg.JSONOutput, d.Status.String(), provider.Name(), d.ID, statusMessage(d, elapsed))
			lastStatus = d.Status
		}

		if d.Status.Terminal() {
			return &WatchResult{
				Deployment: d,
				Duration:   time.Since(start),
				Polls:      polls,
			}, nil
		}

	wait:
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for deployment", cfg.Timeout)
		case <-ticker.C:
		}
	}
}

func statusMessage(d *Deployment, elapsed time.Duration) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("status=%s", d.Status))
	if d.ID != "" {
		parts = append(parts, fmt.Sprintf("id=%s", truncate(d.ID, 12)))
	}
	if d.Environment != "" {
		parts = append(parts, fmt.Sprintf("env=%s", d.Environment))
	}
	parts = append(parts, fmt.Sprintf("elapsed=%s", elapsed))
	if d.Status.Terminal() && d.URL != "" {
		parts = append(parts, fmt.Sprintf("url=%s", d.URL))
	}
	return strings.Join(parts, " ")
}

type statusUpdate struct {
	Time     string `json:"time"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
	ID       string `json:"id,omitempty"`
	Message  string `json:"message"`
}

func writeStatus(w io.Writer, asJSON bool, status, provider, id, message string) {
	if asJSON {
		u := statusUpdate{
			Time:     time.Now().UTC().Format(time.RFC3339),
			Provider: provider,
			Status:   status,
			ID:       id,
			Message:  message,
		}
		data, _ := json.Marshal(u)
		fmt.Fprintln(w, string(data))
	} else {
		symbol := statusSymbol(status)
		fmt.Fprintf(w, "%s [%s] %s %s\n",
			time.Now().Format("15:04:05"),
			provider,
			symbol,
			message,
		)
	}
}

func statusSymbol(status string) string {
	switch status {
	case "PENDING":
		return "..."
	case "BUILDING":
		return ">>>"
	case "DEPLOYING":
		return ">>>"
	case "SUCCEEDED":
		return "[OK]"
	case "FAILED", "CRASHED":
		return "[FAIL]"
	case "CANCELLED":
		return "[CANCEL]"
	case "error":
		return "[ERR]"
	default:
		return "[?]"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
