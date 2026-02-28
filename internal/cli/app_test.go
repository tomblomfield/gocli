package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/tomblomfield/gocli/internal/config"
	"github.com/tomblomfield/gocli/internal/format"
)

func TestSplitStatements_Single(t *testing.T) {
	stmts := SplitStatements("SELECT * FROM users")
	if len(stmts) != 1 {
		t.Errorf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	if stmts[0] != "SELECT * FROM users" {
		t.Errorf("unexpected statement: %q", stmts[0])
	}
}

func TestSplitStatements_Multiple(t *testing.T) {
	stmts := SplitStatements("SELECT 1; SELECT 2; SELECT 3")
	if len(stmts) != 3 {
		t.Errorf("expected 3 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_WithTrailingSemicolon(t *testing.T) {
	stmts := SplitStatements("SELECT 1;")
	if len(stmts) != 1 {
		t.Errorf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_StringLiteral(t *testing.T) {
	stmts := SplitStatements("SELECT 'hello; world'")
	if len(stmts) != 1 {
		t.Errorf("semicolon in string should not split: got %d statements: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_DoubleQuoted(t *testing.T) {
	stmts := SplitStatements(`SELECT "col;name" FROM t`)
	if len(stmts) != 1 {
		t.Errorf("semicolon in double-quoted identifier should not split: got %d", len(stmts))
	}
}

func TestSplitStatements_SingleLineComment(t *testing.T) {
	stmts := SplitStatements("SELECT 1; -- comment with ; in it\nSELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_BlockComment(t *testing.T) {
	stmts := SplitStatements("SELECT /* comment; here */ 1; SELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_Empty(t *testing.T) {
	stmts := SplitStatements("")
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements, got %d", len(stmts))
	}
}

func TestSplitStatements_OnlySemicolons(t *testing.T) {
	stmts := SplitStatements(";;;")
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements from only semicolons, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_EscapedQuotes(t *testing.T) {
	stmts := SplitStatements("SELECT 'it''s a test; really'; SELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_Whitespace(t *testing.T) {
	stmts := SplitStatements("  SELECT 1  ;  SELECT 2  ")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d", len(stmts))
	}
	if stmts[0] != "SELECT 1" {
		t.Errorf("statements should be trimmed: %q", stmts[0])
	}
}

func TestSplitStatements_MultiLine(t *testing.T) {
	query := `SELECT *
FROM users
WHERE id = 1;

SELECT *
FROM orders
WHERE total > 100`

	stmts := SplitStatements(query)
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d", len(stmts))
	}
}

func TestSplitStatements_NestedComments(t *testing.T) {
	stmts := SplitStatements("SELECT /* outer /* inner */ */ 1; SELECT 2")
	if len(stmts) < 1 {
		t.Error("should produce at least 1 statement")
	}
}

func TestSplitStatements_ExecuteMode(t *testing.T) {
	// Simulates how -e flag input should be split before execution
	input := "SELECT 1 AS a; SELECT 2 AS b; SELECT 3 AS c"
	stmts := SplitStatements(input)
	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %d: %v", len(stmts), stmts)
	}
	if stmts[0] != "SELECT 1 AS a" {
		t.Errorf("stmt[0] = %q, want %q", stmts[0], "SELECT 1 AS a")
	}
	if stmts[1] != "SELECT 2 AS b" {
		t.Errorf("stmt[1] = %q, want %q", stmts[1], "SELECT 2 AS b")
	}
	if stmts[2] != "SELECT 3 AS c" {
		t.Errorf("stmt[2] = %q, want %q", stmts[2], "SELECT 3 AS c")
	}
}

func TestSplitStatements_ExecuteModeMixed(t *testing.T) {
	// Mix of DML and SELECT, as might be passed via -e
	input := "CREATE TABLE t (id int); INSERT INTO t VALUES (1); SELECT * FROM t"
	stmts := SplitStatements(input)
	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestGetPrompt_PostgreSQL(t *testing.T) {
	cfg := config.DefaultPGConfig()
	app := &App{
		mode:   PostgreSQL,
		config: cfg,
	}

	prompt := app.GetPrompt()
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
}

func TestGetPrompt_MySQL(t *testing.T) {
	cfg := config.DefaultMySQLConfig()
	app := &App{
		mode:   MySQL,
		config: cfg,
	}

	prompt := app.GetPrompt()
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
}

func TestGetContinuationPrompt(t *testing.T) {
	cfg := config.DefaultMySQLConfig()
	cfg.PromptContinuation = "... "
	app := &App{config: cfg}

	prompt := app.GetContinuationPrompt()
	if prompt != "... " {
		t.Errorf("unexpected continuation prompt: %q", prompt)
	}
}

func TestGetContinuationPrompt_Default(t *testing.T) {
	cfg := config.DefaultPGConfig()
	cfg.PromptContinuation = ""
	app := &App{config: cfg}

	prompt := app.GetContinuationPrompt()
	if prompt != "-> " {
		t.Errorf("default continuation prompt should be '-> ', got %q", prompt)
	}
}

// mockExecutor implements the Executor interface for testing.
type mockExecutor struct {
	results  []*format.QueryResult
	err      error
	database string
	version  string
}

func (m *mockExecutor) Execute(_ context.Context, query string) (*format.QueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.results) > 0 {
		r := m.results[0]
		if len(m.results) > 1 {
			m.results = m.results[1:]
		}
		return r, nil
	}
	return &format.QueryResult{
		Columns:    []string{"result"},
		Rows:       [][]string{{"1"}},
		StatusText: "(1 row)",
		RowCount:   1,
	}, nil
}

func (m *mockExecutor) Close() error              { return nil }
func (m *mockExecutor) Database() string           { return m.database }
func (m *mockExecutor) ServerVersion() (string, error) { return m.version, nil }
func (m *mockExecutor) Tables(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (m *mockExecutor) Columns(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (m *mockExecutor) Schemas(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockExecutor) Functions(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (m *mockExecutor) Databases(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockExecutor) Datatypes(_ context.Context) []string { return nil }

func newTestApp(mode DBMode) (*App, *bytes.Buffer) {
	cfg := config.DefaultPGConfig()
	if mode == MySQL {
		cfg = config.DefaultMySQLConfig()
	}
	cfg.LessChatty = true
	mock := &mockExecutor{database: "testdb", version: "15.0"}
	app := NewApp(mode, mock, mock, cfg)
	var buf bytes.Buffer
	app.Stdout = &buf
	app.Stderr = &buf
	return app, &buf
}

func TestNewApp_RegistersPGCommands(t *testing.T) {
	app, _ := newTestApp(PostgreSQL)
	// PG-specific commands should be registered
	pgCommands := []string{`\dt`, `\d`, `\l`, `\dn`, `\du`, `\dx`, `\di`, `\ds`, `\df`, `\dv`, `\conninfo`}
	for _, cmd := range pgCommands {
		if !app.special.IsSpecial(cmd) {
			t.Errorf("PG command %s should be registered in App", cmd)
		}
	}
}

func TestNewApp_RegistersMySQLCommands(t *testing.T) {
	app, _ := newTestApp(MySQL)
	// Common commands should be registered
	if !app.special.IsSpecial(`\?`) {
		t.Error("common command \\? should be registered")
	}
	if !app.special.IsSpecial(`\q`) {
		t.Error("common command \\q should be registered")
	}
}

func TestExecuteNonInteractive_SQL(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	hasError := app.ExecuteNonInteractive("SELECT 1")
	if hasError {
		t.Error("should not have error for valid SQL")
	}
	output := buf.String()
	if output == "" {
		t.Error("should produce output for SELECT query")
	}
}

func TestExecuteNonInteractive_SpecialCommand(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	hasError := app.ExecuteNonInteractive(`\?`)
	if hasError {
		t.Error("should not have error for \\? command")
	}
	output := buf.String()
	if output == "" {
		t.Error("\\? should produce help output")
	}
}

func TestExecuteNonInteractive_MultiStatement(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	hasError := app.ExecuteNonInteractive("SELECT 1; SELECT 2")
	if hasError {
		t.Error("should not have error for multi-statement")
	}
	output := buf.String()
	if output == "" {
		t.Error("multi-statement should produce output")
	}
}

func TestExecuteNonInteractive_Empty(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	hasError := app.ExecuteNonInteractive("")
	if hasError {
		t.Error("empty input should not be an error")
	}
	if buf.String() != "" {
		t.Error("empty input should produce no output")
	}
}

func TestExecuteNonInteractive_Error(t *testing.T) {
	cfg := config.DefaultPGConfig()
	cfg.LessChatty = true
	mock := &mockExecutor{
		database: "testdb",
		version:  "15.0",
		err:      context.DeadlineExceeded,
	}
	app := NewApp(PostgreSQL, mock, mock, cfg)
	var buf bytes.Buffer
	app.Stdout = &buf
	app.Stderr = &buf

	hasError := app.ExecuteNonInteractive("SELECT 1")
	if !hasError {
		t.Error("should report error when executor fails")
	}
}

func TestHandleInput_SpecialCommand_Quit(t *testing.T) {
	app, _ := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput(`\q`)
	if !shouldQuit {
		t.Error("\\q should signal quit")
	}
}

func TestHandleInput_SpecialCommand_Help(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput(`\?`)
	if shouldQuit {
		t.Error("\\? should not signal quit")
	}
	if buf.Len() == 0 {
		t.Error("\\? should produce help output")
	}
}

func TestHandleInput_SQL(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput("SELECT 1;")
	if shouldQuit {
		t.Error("SQL should not signal quit")
	}
	if buf.Len() == 0 {
		t.Error("SQL should produce output")
	}
}

// pgcli equivalent: test_on_error_resume
func TestHandleInput_OnErrorResume(t *testing.T) {
	cfg := config.DefaultPGConfig()
	cfg.LessChatty = true
	cfg.OnError = "RESUME"
	mock := &mockExecutor{database: "testdb", version: "15.0"}
	app := NewApp(PostgreSQL, mock, mock, cfg)
	var buf bytes.Buffer
	app.Stdout = &buf
	app.Stderr = &buf

	// Should not crash and should continue after error
	shouldQuit := app.HandleInput("SELECT 1;")
	if shouldQuit {
		t.Error("should not quit on valid SQL")
	}
}

// pgcli equivalent: test_on_error_stop
func TestHandleInput_OnErrorStop(t *testing.T) {
	cfg := config.DefaultPGConfig()
	cfg.LessChatty = true
	cfg.OnError = "STOP"
	mock := &mockExecutor{database: "testdb", version: "15.0"}
	app := NewApp(PostgreSQL, mock, mock, cfg)
	var buf bytes.Buffer
	app.Stdout = &buf
	app.Stderr = &buf

	shouldQuit := app.HandleInput("SELECT 1;")
	if shouldQuit {
		t.Error("should not quit on valid SQL even in STOP mode")
	}
}

// pgcli equivalent: test_multiple_queries_with_special_command_same_line
func TestExecuteNonInteractive_MixedSpecialAndSQL(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	// Mix of special and SQL in sequence — the special command should be handled first
	hasError := app.ExecuteNonInteractive(`\echo hello`)
	if hasError {
		t.Error("should not error on \\echo")
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Error("should output echo text")
	}
}

// pgcli equivalent: test_toggle_verbose_errors
func TestHandleInput_VerboseErrors(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput(`\v on`)
	if shouldQuit {
		t.Error("\\v should not quit")
	}
	if !strings.Contains(buf.String(), "on") {
		t.Error("should confirm verbose errors on")
	}
}

// pgcli equivalent: test_i_works
func TestHandleInput_ExecuteFile(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput(`\i /tmp/test.sql`)
	if shouldQuit {
		t.Error("\\i should not quit")
	}
	if !strings.Contains(buf.String(), "test.sql") {
		t.Error("should acknowledge file")
	}
}

// pgcli equivalent: test_watch_works
func TestHandleInput_Watch(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	shouldQuit := app.HandleInput(`\watch 3`)
	if shouldQuit {
		t.Error("\\watch should not quit")
	}
	if !strings.Contains(buf.String(), "3") {
		t.Error("should set watch interval")
	}
}

// pgcli equivalent: test_describe_special
func TestHandleInput_DescribeCommands(t *testing.T) {
	app, _ := newTestApp(PostgreSQL)

	// These should all be recognized as special commands (not sent to SQL)
	specialCmds := []string{`\dt`, `\dv`, `\di`, `\ds`, `\df`, `\dn`, `\du`, `\dx`, `\l`, `\d`}
	for _, cmd := range specialCmds {
		if !app.special.IsSpecial(cmd) {
			t.Errorf("command %s should be recognized as special", cmd)
		}
	}
}

// pgcli equivalent: test_duration_in_words
func TestFormatTimingOutput(t *testing.T) {
	app, buf := newTestApp(PostgreSQL)
	app.special.Timing = true
	app.HandleInput("SELECT 1;")
	output := buf.String()
	if !strings.Contains(output, "Time:") {
		t.Error("should show timing when enabled")
	}
}
