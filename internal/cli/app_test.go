package cli

import (
	"testing"

	"github.com/tomblomfield/gocli/internal/config"
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
