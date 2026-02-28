package special

import (
	"context"
	"strings"
	"testing"

	"github.com/tomblomfield/gocli/internal/format"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry should not return nil")
	}

	// Should have common commands registered
	cmds := r.Commands()
	if len(cmds) == 0 {
		t.Error("registry should have commands registered")
	}
}

func TestIsSpecial(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		input    string
		expected bool
	}{
		{`\q`, true},
		{`\x`, true},
		{`\timing`, true},
		{`\?`, true},
		{"quit", true},
		{"exit", true},
		{`\e`, true},
		{`\!`, true},
		{`\f`, true},
		{`\fs`, true},
		{`\fd`, true},
		{"SELECT * FROM users", false},
		{"", false},
		{"INSERT INTO foo VALUES (1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := r.IsSpecial(tt.input)
			if result != tt.expected {
				t.Errorf("IsSpecial(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExecute_Quit(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\q`)
	if err != ErrQuit {
		t.Errorf("\\q should return ErrQuit, got %v", err)
	}

	_, err = r.Execute(context.Background(), nil, "quit")
	if err != ErrQuit {
		t.Errorf("quit should return ErrQuit, got %v", err)
	}

	_, err = r.Execute(context.Background(), nil, "exit")
	if err != ErrQuit {
		t.Errorf("exit should return ErrQuit, got %v", err)
	}
}

func TestExecute_ExpandedToggle(t *testing.T) {
	r := NewRegistry()

	if r.Expanded {
		t.Error("expanded should start as false")
	}

	results, err := r.Execute(context.Background(), nil, `\x`)
	if err != nil {
		t.Fatalf("\\x should not error: %v", err)
	}

	if !r.Expanded {
		t.Error("expanded should be true after first \\x")
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "on") {
		t.Error("should report 'on'")
	}

	results, err = r.Execute(context.Background(), nil, `\x`)
	if err != nil {
		t.Fatalf("\\x should not error: %v", err)
	}

	if r.Expanded {
		t.Error("expanded should be false after second \\x")
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "off") {
		t.Error("should report 'off'")
	}
}

func TestExecute_TimingToggle(t *testing.T) {
	r := NewRegistry()

	if !r.Timing {
		t.Error("timing should start as true")
	}

	r.Execute(context.Background(), nil, `\timing`)
	if r.Timing {
		t.Error("timing should be false after toggle")
	}

	r.Execute(context.Background(), nil, `\timing`)
	if !r.Timing {
		t.Error("timing should be true after second toggle")
	}
}

func TestExecute_Pager(t *testing.T) {
	r := NewRegistry()

	results, err := r.Execute(context.Background(), nil, `\pager less -SRXF`)
	if err != nil {
		t.Fatalf("\\pager should not error: %v", err)
	}

	if r.Pager != "less -SRXF" {
		t.Errorf("pager should be 'less -SRXF', got %q", r.Pager)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "less -SRXF") {
		t.Error("should report pager setting")
	}

	// Reset pager
	r.Execute(context.Background(), nil, `\pager`)
}

func TestExecute_Help(t *testing.T) {
	r := NewRegistry()

	results, err := r.Execute(context.Background(), nil, `\?`)
	if err != nil {
		t.Fatalf("\\? should not error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("help should return results")
	}
	if len(results[0].Columns) != 2 {
		t.Error("help should have 2 columns (Command, Description)")
	}
	if len(results[0].Rows) == 0 {
		t.Error("help should list commands")
	}
}

func TestExecute_Favorites(t *testing.T) {
	r := NewRegistry()

	// Save favorite
	results, err := r.Execute(context.Background(), nil, `\fs myquery SELECT * FROM users`)
	if err != nil {
		t.Fatalf("\\fs should not error: %v", err)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "myquery") {
		t.Error("should confirm saved favorite")
	}

	// Check it's stored
	if r.Favorites["myquery"] != "SELECT * FROM users" {
		t.Errorf("favorite should be stored, got %q", r.Favorites["myquery"])
	}

	// List favorites
	results, err = r.Execute(context.Background(), nil, `\f`)
	if err != nil {
		t.Fatalf("\\f should not error: %v", err)
	}
	if len(results) == 0 || len(results[0].Rows) != 1 {
		t.Error("should list one favorite")
	}

	// Execute favorite
	results, err = r.Execute(context.Background(), nil, `\f myquery`)
	if err != nil {
		t.Fatalf("\\f myquery should not error: %v", err)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "SELECT * FROM users") {
		t.Error("should show the query text")
	}

	// Delete favorite
	results, err = r.Execute(context.Background(), nil, `\fd myquery`)
	if err != nil {
		t.Fatalf("\\fd should not error: %v", err)
	}
	if _, ok := r.Favorites["myquery"]; ok {
		t.Error("favorite should be deleted")
	}
}

func TestExecute_FavoriteNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\f nonexistent`)
	if err == nil {
		t.Error("should error for nonexistent favorite")
	}
}

func TestExecute_FavoriteWithParams(t *testing.T) {
	r := NewRegistry()

	r.Favorites["user_by_id"] = "SELECT * FROM users WHERE id = $1"

	results, err := r.Execute(context.Background(), nil, `\f user_by_id 42`)
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "42") {
		t.Error("should substitute parameter")
	}
}

func TestExecute_SaveFavoriteInvalid(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\fs`)
	if err == nil {
		t.Error("\\fs with no args should error")
	}
}

func TestExecute_DeleteFavoriteNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\fd nonexistent`)
	if err == nil {
		t.Error("\\fd for nonexistent should error")
	}
}

func TestExecute_Watch(t *testing.T) {
	r := NewRegistry()

	results, err := r.Execute(context.Background(), nil, `\watch 5`)
	if err != nil {
		t.Fatalf("\\watch should not error: %v", err)
	}
	if r.WatchSecs != 5 {
		t.Errorf("watch seconds should be 5, got %d", r.WatchSecs)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "5") {
		t.Error("should report watch interval")
	}
}

func TestExecute_Refresh(t *testing.T) {
	r := NewRegistry()

	results, err := r.Execute(context.Background(), nil, `\#`)
	if err != nil {
		t.Fatalf("\\# should not error: %v", err)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "refreshed") {
		t.Error("should confirm refresh")
	}

	// Test alias
	results, err = r.Execute(context.Background(), nil, "rehash")
	if err != nil {
		t.Fatalf("rehash should not error: %v", err)
	}
}

func TestExecute_Shell(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\! echo hello`)
	if err != nil {
		t.Fatalf("\\! should not error for simple command: %v", err)
	}

	_, err = r.Execute(context.Background(), nil, `\!`)
	if err == nil {
		t.Error("\\! with no command should error")
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(context.Background(), nil, `\zzz`)
	if err == nil {
		t.Error("unknown command should error")
	}
}

func TestParseSpecialCommand(t *testing.T) {
	tests := []struct {
		input       string
		cmd, arg    string
		verbose     bool
	}{
		{`\dt`, `\dt`, "", false},
		{`\dt+`, `\dt`, "", true},
		{`\dt users`, `\dt`, "users", false},
		{`\dt+ public.*`, `\dt`, "public.*", true},
		{`\l`, `\l`, "", false},
		{`\pager less -SRXF`, `\pager`, "less -SRXF", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, arg, verbose := parseSpecialCommand(tt.input)
			if cmd != tt.cmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.cmd)
			}
			if arg != tt.arg {
				t.Errorf("arg = %q, want %q", arg, tt.arg)
			}
			if verbose != tt.verbose {
				t.Errorf("verbose = %v, want %v", verbose, tt.verbose)
			}
		})
	}
}

func TestFormatTiming(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"500µs", "ms"},
		{"1.5s", "s"},
	}

	_ = tests

	// Quick timing test
	result := FormatTiming(500000) // 500ms in nanoseconds
	if result == "" {
		t.Error("FormatTiming should produce output")
	}
}

func TestCommands_NoHidden(t *testing.T) {
	r := NewRegistry()
	cmds := r.Commands()

	for _, cmd := range cmds {
		if cmd.Hidden {
			t.Errorf("Commands() should not return hidden command: %q", cmd.Name)
		}
	}
}

func TestCommands_NoDuplicates(t *testing.T) {
	r := NewRegistry()
	cmds := r.Commands()

	seen := make(map[string]bool)
	for _, cmd := range cmds {
		if seen[cmd.Name] {
			t.Errorf("duplicate command: %q", cmd.Name)
		}
		seen[cmd.Name] = true
	}
}

func TestRegister_WithAliases(t *testing.T) {
	r := NewRegistry()

	r.Register(&Command{
		Name:    "test",
		Aliases: []string{"t", "tst"},
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return nil, nil
		},
	})

	if !r.IsSpecial("test") {
		t.Error("should recognize main command")
	}
	if !r.IsSpecial("t") {
		t.Error("should recognize alias 't'")
	}
	if !r.IsSpecial("tst") {
		t.Error("should recognize alias 'tst'")
	}
}

func TestExecute_EmptyFavorites(t *testing.T) {
	r := NewRegistry()

	results, err := r.Execute(context.Background(), nil, `\f`)
	if err != nil {
		t.Fatalf("\\f should not error: %v", err)
	}
	if len(results) == 0 || !strings.Contains(results[0].StatusText, "No favorites") {
		t.Error("should report no favorites")
	}
}
