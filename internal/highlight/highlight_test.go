package highlight

import (
	"strings"
	"testing"
)

func TestTokenize_SimpleSelect(t *testing.T) {
	tokens := Tokenize("SELECT * FROM users")

	if len(tokens) == 0 {
		t.Fatal("should produce tokens")
	}

	// First token should be keyword SELECT
	if tokens[0].Type != TokenKeyword || tokens[0].Value != "SELECT" {
		t.Errorf("first token should be keyword 'SELECT', got %+v", tokens[0])
	}

	// Verify keywords are identified
	keywordCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			keywordCount++
		}
	}
	if keywordCount < 2 { // SELECT, FROM
		t.Errorf("expected at least 2 keywords, got %d", keywordCount)
	}
}

func TestTokenize_StringLiteral(t *testing.T) {
	tokens := Tokenize("SELECT 'hello world'")

	hasString := false
	for _, tok := range tokens {
		if tok.Type == TokenString && strings.Contains(tok.Value, "hello world") {
			hasString = true
		}
	}
	if !hasString {
		t.Error("should identify string literal")
	}
}

func TestTokenize_EscapedQuote(t *testing.T) {
	tokens := Tokenize("SELECT 'it''s a test'")

	hasString := false
	for _, tok := range tokens {
		if tok.Type == TokenString {
			hasString = true
			if !strings.Contains(tok.Value, "''") {
				t.Error("escaped quotes should be preserved")
			}
		}
	}
	if !hasString {
		t.Error("should identify string with escaped quotes")
	}
}

func TestTokenize_Number(t *testing.T) {
	tokens := Tokenize("SELECT 42, 3.14")

	numberCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenNumber {
			numberCount++
		}
	}
	if numberCount != 2 {
		t.Errorf("expected 2 numbers, got %d", numberCount)
	}
}

func TestTokenize_SingleLineComment(t *testing.T) {
	tokens := Tokenize("SELECT 1 -- this is a comment\nSELECT 2")

	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
			if !strings.Contains(tok.Value, "this is a comment") {
				t.Error("comment should contain comment text")
			}
		}
	}
	if !hasComment {
		t.Error("should identify single-line comment")
	}
}

func TestTokenize_MultiLineComment(t *testing.T) {
	tokens := Tokenize("SELECT /* block\ncomment */ 1")

	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
			if !strings.Contains(tok.Value, "block") {
				t.Error("comment should contain comment text")
			}
		}
	}
	if !hasComment {
		t.Error("should identify multi-line comment")
	}
}

func TestTokenize_DoubleQuotedIdentifier(t *testing.T) {
	tokens := Tokenize(`SELECT "Column Name" FROM "my table"`)

	nameCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenName && strings.HasPrefix(tok.Value, `"`) {
			nameCount++
		}
	}
	if nameCount != 2 {
		t.Errorf("expected 2 quoted identifiers, got %d", nameCount)
	}
}

func TestTokenize_BacktickIdentifier(t *testing.T) {
	tokens := Tokenize("SELECT `column` FROM `table`")

	nameCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenName && strings.HasPrefix(tok.Value, "`") {
			nameCount++
		}
	}
	if nameCount != 2 {
		t.Errorf("expected 2 backtick identifiers, got %d", nameCount)
	}
}

func TestTokenize_Operators(t *testing.T) {
	tokens := Tokenize("SELECT 1 + 2 * 3 = 6")

	opCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenOperator {
			opCount++
		}
	}
	if opCount < 3 { // +, *, =
		t.Errorf("expected at least 3 operators, got %d", opCount)
	}
}

func TestTokenize_Function(t *testing.T) {
	tokens := Tokenize("SELECT count(*) FROM users")

	hasFunc := false
	for _, tok := range tokens {
		if tok.Type == TokenFunction && strings.ToLower(tok.Value) == "count" {
			hasFunc = true
		}
	}
	if !hasFunc {
		t.Error("should identify 'count' as a function (followed by '(')")
	}
}

func TestTokenize_DollarQuotedString(t *testing.T) {
	tokens := Tokenize("SELECT $$hello world$$")

	hasString := false
	for _, tok := range tokens {
		if tok.Type == TokenString && strings.Contains(tok.Value, "hello world") {
			hasString = true
		}
	}
	if !hasString {
		t.Error("should identify dollar-quoted string")
	}
}

func TestTokenize_DollarTaggedString(t *testing.T) {
	tokens := Tokenize("SELECT $tag$hello world$tag$")

	hasString := false
	for _, tok := range tokens {
		if tok.Type == TokenString && strings.Contains(tok.Value, "hello world") {
			hasString = true
		}
	}
	if !hasString {
		t.Error("should identify dollar-tagged string")
	}
}

func TestTokenize_Empty(t *testing.T) {
	tokens := Tokenize("")
	if len(tokens) != 0 {
		t.Errorf("empty input should produce no tokens, got %d", len(tokens))
	}
}

func TestTokenize_WhitespaceOnly(t *testing.T) {
	tokens := Tokenize("   \t\n  ")
	for _, tok := range tokens {
		if tok.Type != TokenWhitespace {
			t.Errorf("whitespace-only input should only produce whitespace tokens, got %+v", tok)
		}
	}
}

func TestTokenize_ComplexQuery(t *testing.T) {
	query := `SELECT u.name, count(o.id) AS order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE u.created_at > '2024-01-01'
GROUP BY u.name
HAVING count(o.id) > 5
ORDER BY order_count DESC
LIMIT 10`

	tokens := Tokenize(query)
	if len(tokens) == 0 {
		t.Fatal("complex query should produce tokens")
	}

	// Count different token types
	types := make(map[TokenType]int)
	for _, tok := range tokens {
		types[tok.Type]++
	}

	if types[TokenKeyword] == 0 {
		t.Error("complex query should have keywords")
	}
	if types[TokenNumber] == 0 {
		t.Error("complex query should have numbers")
	}
	if types[TokenString] == 0 {
		t.Error("complex query should have strings")
	}
	if types[TokenName] == 0 {
		t.Error("complex query should have names/identifiers")
	}
}

func TestHighlight_ProducesOutput(t *testing.T) {
	style := DefaultStyle()
	result := Highlight("SELECT * FROM users", style)

	if result == "" {
		t.Error("highlight should produce output")
	}

	// Should contain ANSI escape codes
	if !strings.Contains(result, "\033[") {
		t.Error("highlighted output should contain ANSI escape codes")
	}

	// Should contain Reset codes
	if !strings.Contains(result, Reset) {
		t.Error("highlighted output should contain reset codes")
	}
}

func TestHighlight_PreservesContent(t *testing.T) {
	style := DefaultStyle()
	input := "SELECT name FROM users WHERE id = 1"
	result := Highlight(input, style)

	// Strip ANSI codes and check content
	stripped := stripANSI(result)
	if stripped != input {
		t.Errorf("after stripping ANSI, content should be preserved\nGot:  %q\nWant: %q", stripped, input)
	}
}

func TestHighlight_EmptyInput(t *testing.T) {
	style := DefaultStyle()
	result := Highlight("", style)
	if result != "" {
		t.Error("empty input should produce empty output")
	}
}

func TestGetStyle(t *testing.T) {
	tests := []string{"default", "monokai", "native", "vim", "fruity", "unknown"}
	for _, name := range tests {
		style := GetStyle(name)
		if style.Keyword == "" {
			t.Errorf("style %q should have keyword color", name)
		}
		if style.String == "" {
			t.Errorf("style %q should have string color", name)
		}
	}
}

func TestDefaultStyle_HasAllColors(t *testing.T) {
	style := DefaultStyle()

	if style.Keyword == "" {
		t.Error("Keyword color should not be empty")
	}
	if style.Function == "" {
		t.Error("Function color should not be empty")
	}
	if style.String == "" {
		t.Error("String color should not be empty")
	}
	if style.Number == "" {
		t.Error("Number color should not be empty")
	}
	if style.Comment == "" {
		t.Error("Comment color should not be empty")
	}
	if style.Operator == "" {
		t.Error("Operator color should not be empty")
	}
}

func TestIsKeyword(t *testing.T) {
	keywords := []string{"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP"}
	for _, kw := range keywords {
		if !isKeyword(kw) {
			t.Errorf("%q should be a keyword", kw)
		}
	}

	nonKeywords := []string{"users", "id", "name", "foobar"}
	for _, w := range nonKeywords {
		if isKeyword(strings.ToUpper(w)) {
			t.Errorf("%q should not be a keyword", w)
		}
	}
}

func TestIsFunction(t *testing.T) {
	functions := []string{"COUNT", "SUM", "AVG", "MAX", "MIN", "NOW", "COALESCE"}
	for _, fn := range functions {
		if !isFunction(fn) {
			t.Errorf("%q should be a function", fn)
		}
	}
}

func TestTokenize_MySQLSpecific(t *testing.T) {
	// MySQL-specific keywords
	tokens := Tokenize("SHOW DATABASES")
	if len(tokens) == 0 {
		t.Fatal("should produce tokens")
	}

	keywordCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			keywordCount++
		}
	}
	if keywordCount != 2 {
		t.Errorf("'SHOW DATABASES' should have 2 keywords, got %d", keywordCount)
	}
}

func TestTokenize_PGSpecific(t *testing.T) {
	// PostgreSQL-specific features
	tokens := Tokenize("SELECT jsonb_build_object('key', value) FROM data")

	hasFunc := false
	for _, tok := range tokens {
		if tok.Type == TokenFunction {
			hasFunc = true
		}
	}
	if !hasFunc {
		t.Error("should identify PostgreSQL functions")
	}
}

func TestHighlight_DifferentStyles(t *testing.T) {
	input := "SELECT * FROM users"
	styles := []string{"default", "monokai", "native", "vim", "fruity"}

	for _, name := range styles {
		style := GetStyle(name)
		result := Highlight(input, style)
		if result == "" {
			t.Errorf("style %q should produce output", name)
		}
		// Each style should produce different output (different colors)
		if !strings.Contains(result, "\033[") {
			t.Errorf("style %q should contain ANSI codes", name)
		}
	}
}

// Helper to strip ANSI escape codes
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			// Skip ESC[...m sequence
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) && s[i] != 'm' {
					i++
				}
				if i < len(s) {
					i++ // skip 'm'
				}
			}
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

func BenchmarkTokenize(b *testing.B) {
	query := `SELECT u.name, count(o.id) AS order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE u.created_at > '2024-01-01'
GROUP BY u.name
HAVING count(o.id) > 5
ORDER BY order_count DESC
LIMIT 10`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Tokenize(query)
	}
}

func BenchmarkHighlight(b *testing.B) {
	query := "SELECT u.name, count(o.id) FROM users u JOIN orders o ON u.id = o.user_id WHERE u.active = true"
	style := DefaultStyle()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Highlight(query, style)
	}
}
