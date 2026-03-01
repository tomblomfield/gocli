package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPGConfig(t *testing.T) {
	cfg := DefaultPGConfig()

	if !cfg.SmartCompletion {
		t.Error("smart_completion should default to true")
	}
	if cfg.MultiLine {
		t.Error("multi_line should default to false")
	}
	if cfg.ViMode {
		t.Error("vi mode should default to false")
	}
	if !cfg.Timing {
		t.Error("timing should default to true")
	}
	if cfg.TableFormat != "psql" {
		t.Errorf("table_format should default to 'psql', got %q", cfg.TableFormat)
	}
	if cfg.RowLimit != 1000 {
		t.Errorf("row_limit should default to 1000, got %d", cfg.RowLimit)
	}
	if cfg.KeywordCasing != "auto" {
		t.Errorf("keyword_casing should default to 'auto', got %q", cfg.KeywordCasing)
	}
	if !cfg.DestructiveWarning {
		t.Error("destructive_warning should default to true")
	}
	if len(cfg.DestructiveKeywords) == 0 {
		t.Error("should have default destructive keywords")
	}
	if cfg.Prompt != `\u@\h:\d> ` {
		t.Errorf("unexpected default prompt: %q", cfg.Prompt)
	}
}

func TestDefaultMySQLConfig(t *testing.T) {
	cfg := DefaultMySQLConfig()

	if cfg.TableFormat != "ascii" {
		t.Errorf("MySQL table_format should default to 'ascii', got %q", cfg.TableFormat)
	}
	if cfg.PromptContinuation != "-> " {
		t.Errorf("MySQL prompt_continuation should default to '-> ', got %q", cfg.PromptContinuation)
	}
	if !cfg.EnablePager {
		t.Error("enable_pager should default to true")
	}
}

func TestConfigLoad(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	content := `[main]
smart_completion = True
multi_line = True
vi = True
timing = False
table_format = ascii
syntax_style = monokai
keyword_casing = upper
row_limit = 500
less_chatty = True
prompt = \d>
null_string = <null>
destructive_warning = False
enable_pager = False
pager = less -SRXF
log_level = DEBUG
max_field_width = 200

[named queries]
active = SELECT * FROM users WHERE active = true
count = SELECT count(*) FROM $1

[alias_dsn]
mydb = postgres://user:pass@localhost:5432/mydb
testdb = postgres://test@localhost/testdb

[colors]
keyword = ansibrightblue
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := DefaultPGConfig()
	if err := cfg.Load(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.SmartCompletion {
		t.Error("smart_completion should be true")
	}
	if !cfg.MultiLine {
		t.Error("multi_line should be true")
	}
	if !cfg.ViMode {
		t.Error("vi should be true")
	}
	if cfg.Timing {
		t.Error("timing should be false")
	}
	if cfg.TableFormat != "ascii" {
		t.Errorf("table_format should be 'ascii', got %q", cfg.TableFormat)
	}
	if cfg.SyntaxStyle != "monokai" {
		t.Errorf("syntax_style should be 'monokai', got %q", cfg.SyntaxStyle)
	}
	if cfg.KeywordCasing != "upper" {
		t.Errorf("keyword_casing should be 'upper', got %q", cfg.KeywordCasing)
	}
	if cfg.RowLimit != 500 {
		t.Errorf("row_limit should be 500, got %d", cfg.RowLimit)
	}
	if !cfg.LessChatty {
		t.Error("less_chatty should be true")
	}
	if cfg.Prompt != `\d>` {
		t.Errorf("prompt should be '\\d>', got %q", cfg.Prompt)
	}
	if cfg.NullString != "<null>" {
		t.Errorf("null_string should be '<null>', got %q", cfg.NullString)
	}
	if cfg.DestructiveWarning {
		t.Error("destructive_warning should be false")
	}
	if cfg.EnablePager {
		t.Error("enable_pager should be false")
	}
	if cfg.Pager != "less -SRXF" {
		t.Errorf("pager should be 'less -SRXF', got %q", cfg.Pager)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("log_level should be 'DEBUG', got %q", cfg.LogLevel)
	}
	if cfg.MaxFieldWidth != 200 {
		t.Errorf("max_field_width should be 200, got %d", cfg.MaxFieldWidth)
	}

	// Named queries
	if len(cfg.NamedQueries) != 2 {
		t.Errorf("expected 2 named queries, got %d", len(cfg.NamedQueries))
	}
	if cfg.NamedQueries["active"] != "SELECT * FROM users WHERE active = true" {
		t.Errorf("unexpected named query: %q", cfg.NamedQueries["active"])
	}

	// DSN aliases
	if len(cfg.DSNAliases) != 2 {
		t.Errorf("expected 2 DSN aliases, got %d", len(cfg.DSNAliases))
	}
	if cfg.DSNAliases["mydb"] != "postgres://user:pass@localhost:5432/mydb" {
		t.Errorf("unexpected DSN alias: %q", cfg.DSNAliases["mydb"])
	}

	// Colors
	if cfg.Colors["keyword"] != "ansibrightblue" {
		t.Errorf("unexpected color: %q", cfg.Colors["keyword"])
	}
}

func TestConfigLoad_NonExistent(t *testing.T) {
	cfg := DefaultPGConfig()
	err := cfg.Load("/nonexistent/path/config")
	if err != nil {
		t.Error("loading nonexistent config should not error (it's optional)")
	}
}

func TestConfigLoad_MySQLFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "myclirc")

	content := `[main]
smart_completion = True
key_bindings = vi
table_format = grid
prompt = \t \u@\h:\d>
prompt_continuation = ...>

[favorite_queries]
users = SELECT * FROM users LIMIT 10
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := DefaultMySQLConfig()
	if err := cfg.Load(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.ViMode {
		t.Error("vi mode should be true from key_bindings=vi")
	}
	if cfg.KeyBindings != "vi" {
		t.Errorf("key_bindings should be 'vi', got %q", cfg.KeyBindings)
	}
	if cfg.TableFormat != "grid" {
		t.Errorf("table_format should be 'grid', got %q", cfg.TableFormat)
	}
	if cfg.PromptContinuation != "...>" {
		t.Errorf("prompt_continuation should be '...>', got %q", cfg.PromptContinuation)
	}
	if cfg.NamedQueries["users"] != "SELECT * FROM users LIMIT 10" {
		t.Errorf("unexpected favorite query: %q", cfg.NamedQueries["users"])
	}
}

func TestConfigSave(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "config")

	cfg := DefaultPGConfig()
	cfg.filePath = cfgPath
	cfg.MultiLine = true
	cfg.ViMode = true
	cfg.NamedQueries["test"] = "SELECT 1"
	cfg.DSNAliases["local"] = "postgres://localhost/test"

	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "multi_line = True") {
		t.Error("saved config should contain multi_line = True")
	}
	if !strings.Contains(content, "vi = True") {
		t.Error("saved config should contain vi = True")
	}
	if !strings.Contains(content, "[named queries]") {
		t.Error("saved config should contain [named queries] section")
	}
	if !strings.Contains(content, "test = SELECT 1") {
		t.Error("saved config should contain named query")
	}
	if !strings.Contains(content, "[alias_dsn]") {
		t.Error("saved config should contain [alias_dsn] section")
	}
}

func TestConfigLoad_BoolParsing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	// Test various bool formats
	content := `[main]
smart_completion = yes
multi_line = 1
vi = on
timing = no
enable_pager = false
less_chatty = off
destructive_warning = 0
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := DefaultPGConfig()
	if err := cfg.Load(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.SmartCompletion {
		t.Error("'yes' should parse as true")
	}
	if !cfg.MultiLine {
		t.Error("'1' should parse as true")
	}
	if !cfg.ViMode {
		t.Error("'on' should parse as true")
	}
	if cfg.Timing {
		t.Error("'no' should parse as false")
	}
	if cfg.EnablePager {
		t.Error("'false' should parse as false")
	}
	if cfg.LessChatty {
		t.Error("'off' should parse as false")
	}
	if cfg.DestructiveWarning {
		t.Error("'0' should parse as false")
	}
}

func TestConfigLoad_WithComments(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	content := `# This is a comment
[main]
; This is also a comment
smart_completion = True
# Another comment
table_format = grid
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := DefaultPGConfig()
	if err := cfg.Load(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.SmartCompletion {
		t.Error("should parse correctly with comments")
	}
	if cfg.TableFormat != "grid" {
		t.Error("should parse correctly with comments")
	}
}

func TestFormatPrompt(t *testing.T) {
	tests := []struct {
		format      string
		user, host  string
		db, port    string
		superuser   bool
		expected    string
	}{
		{`\u@\h:\d> `, "alice", "localhost", "mydb", "5432", false, "alice@localhost:mydb> "},
		{`\u@\h:\d# `, "postgres", "server", "production", "5432", false, "postgres@server:production# "},
		{`\d> `, "", "", "test", "", false, "test> "},
		{`[\u@\H:\p/\d]\# `, "admin", "db.host", "mydb", "5433", true, "[admin@db.host:5433/mydb]@ "},
		{`[\u@\h:\p/\d]\# `, "admin", "db.host", "mydb", "5433", true, "[admin@db:5433/mydb]@ "},
		{`[\u@\h:\p/\d]\# `, "user", "host", "db", "5432", false, "[user@host:5432/db]> "},
		{`\u@\h:\d> `, "user", "1.2.3.4", "db", "5432", false, "user@1.2.3.4:db> "},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := FormatPrompt(tt.format, tt.user, tt.host, tt.db, tt.port, tt.superuser)
			if result != tt.expected {
				t.Errorf("FormatPrompt(%q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

func TestIsDestructive(t *testing.T) {
	cfg := DefaultPGConfig()

	tests := []struct {
		query    string
		expected bool
	}{
		{"DROP TABLE users", true},
		{"DELETE FROM users", true},
		{"TRUNCATE users", true},
		{"ALTER TABLE users ADD COLUMN", true},
		{"UPDATE users SET name = 'test'", true},
		{"SELECT * FROM users", false},
		{"INSERT INTO users VALUES (1)", false},
		{"CREATE TABLE test (id int)", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := cfg.IsDestructive(tt.query)
			if result != tt.expected {
				t.Errorf("IsDestructive(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestIsDestructive_Disabled(t *testing.T) {
	cfg := DefaultPGConfig()
	cfg.DestructiveWarning = false

	if cfg.IsDestructive("DROP TABLE users") {
		t.Error("should not warn when destructive_warning is false")
	}
}

func TestIsDestructive_CaseInsensitive(t *testing.T) {
	cfg := DefaultPGConfig()

	if !cfg.IsDestructive("drop table users") {
		t.Error("should match lowercase 'drop'")
	}
	if !cfg.IsDestructive("DROP TABLE users") {
		t.Error("should match uppercase 'DROP'")
	}
}

func TestPGConfigPaths(t *testing.T) {
	paths := PGConfigPaths()
	if len(paths) == 0 {
		t.Error("should return at least one config path")
	}

	foundConfig := false
	for _, p := range paths {
		if strings.Contains(p, "pgcli") {
			foundConfig = true
		}
	}
	if !foundConfig {
		t.Error("paths should contain pgcli config directory")
	}
}

func TestMySQLConfigPaths(t *testing.T) {
	paths := MySQLConfigPaths()
	if len(paths) == 0 {
		t.Error("should return at least one config path")
	}

	foundRC := false
	for _, p := range paths {
		if strings.Contains(p, "mycli") {
			foundRC = true
		}
	}
	if !foundRC {
		t.Error("paths should contain myclirc")
	}
}

func TestParseBool(t *testing.T) {
	trueValues := []string{"True", "true", "TRUE", "yes", "YES", "1", "on", "ON"}
	for _, v := range trueValues {
		if !parseBool(v) {
			t.Errorf("parseBool(%q) should be true", v)
		}
	}

	falseValues := []string{"False", "false", "FALSE", "no", "NO", "0", "off", "OFF", "anything"}
	for _, v := range falseValues {
		if parseBool(v) {
			t.Errorf("parseBool(%q) should be false", v)
		}
	}
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "True" {
		t.Error("boolStr(true) should be 'True'")
	}
	if boolStr(false) != "False" {
		t.Error("boolStr(false) should be 'False'")
	}
}

func TestConfigLoad_ExpandedOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	content := `[main]
expand = True
auto_expand = True
wider_completion_menu = True
show_bottom_toolbar = False
on_error = RESUME
history_file = /tmp/test-history
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := DefaultPGConfig()
	if err := cfg.Load(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.ExpandedOutput {
		t.Error("expand should be true")
	}
	if !cfg.AutoExpand {
		t.Error("auto_expand should be true")
	}
	if !cfg.WiderCompletion {
		t.Error("wider_completion_menu should be true")
	}
	if cfg.ShowToolbar {
		t.Error("show_bottom_toolbar should be false")
	}
	if cfg.OnError != "RESUME" {
		t.Errorf("on_error should be 'RESUME', got %q", cfg.OnError)
	}
	if cfg.HistoryFile != "/tmp/test-history" {
		t.Errorf("history_file should be '/tmp/test-history', got %q", cfg.HistoryFile)
	}
}
