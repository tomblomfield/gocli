package cli

import (
	"testing"

	"github.com/tomblomfield/gocli/internal/config"
)

func TestSplitStatements_Single(t *testing.T) {
	stmts := splitStatements("SELECT * FROM users")
	if len(stmts) != 1 {
		t.Errorf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	if stmts[0] != "SELECT * FROM users" {
		t.Errorf("unexpected statement: %q", stmts[0])
	}
}

func TestSplitStatements_Multiple(t *testing.T) {
	stmts := splitStatements("SELECT 1; SELECT 2; SELECT 3")
	if len(stmts) != 3 {
		t.Errorf("expected 3 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_WithTrailingSemicolon(t *testing.T) {
	stmts := splitStatements("SELECT 1;")
	if len(stmts) != 1 {
		t.Errorf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_StringLiteral(t *testing.T) {
	stmts := splitStatements("SELECT 'hello; world'")
	if len(stmts) != 1 {
		t.Errorf("semicolon in string should not split: got %d statements: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_DoubleQuoted(t *testing.T) {
	stmts := splitStatements(`SELECT "col;name" FROM t`)
	if len(stmts) != 1 {
		t.Errorf("semicolon in double-quoted identifier should not split: got %d", len(stmts))
	}
}

func TestSplitStatements_SingleLineComment(t *testing.T) {
	stmts := splitStatements("SELECT 1; -- comment with ; in it\nSELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_BlockComment(t *testing.T) {
	stmts := splitStatements("SELECT /* comment; here */ 1; SELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_Empty(t *testing.T) {
	stmts := splitStatements("")
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements, got %d", len(stmts))
	}
}

func TestSplitStatements_OnlySemicolons(t *testing.T) {
	stmts := splitStatements(";;;")
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements from only semicolons, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_EscapedQuotes(t *testing.T) {
	stmts := splitStatements("SELECT 'it''s a test; really'; SELECT 2")
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_Whitespace(t *testing.T) {
	stmts := splitStatements("  SELECT 1  ;  SELECT 2  ")
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

	stmts := splitStatements(query)
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d", len(stmts))
	}
}

func TestSplitStatements_NestedComments(t *testing.T) {
	stmts := splitStatements("SELECT /* outer /* inner */ */ 1; SELECT 2")
	if len(stmts) < 1 {
		t.Error("should produce at least 1 statement")
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
