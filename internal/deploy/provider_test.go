package deploy

import "testing"

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusUnknown, "UNKNOWN"},
		{StatusPending, "PENDING"},
		{StatusBuilding, "BUILDING"},
		{StatusDeploying, "DEPLOYING"},
		{StatusSucceeded, "SUCCEEDED"},
		{StatusFailed, "FAILED"},
		{StatusCancelled, "CANCELLED"},
		{StatusCrashed, "CRASHED"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusTerminal(t *testing.T) {
	terminal := []Status{StatusSucceeded, StatusFailed, StatusCancelled, StatusCrashed}
	nonTerminal := []Status{StatusUnknown, StatusPending, StatusBuilding, StatusDeploying}

	for _, s := range terminal {
		if !s.Terminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.Terminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestStatusSuccess(t *testing.T) {
	if !StatusSucceeded.Success() {
		t.Error("StatusSucceeded.Success() should be true")
	}
	for _, s := range []Status{StatusFailed, StatusCancelled, StatusCrashed, StatusPending, StatusBuilding} {
		if s.Success() {
			t.Errorf("%s.Success() should be false", s)
		}
	}
}
