package completion

import (
	"strings"
	"testing"
)

func testMetadata() *Metadata {
	meta := NewMetadata()
	meta.Tables = []string{"users", "orders", "products", "user_settings", "django_migrations"}
	meta.Views = []string{"active_users", "order_summary"}
	meta.Columns = map[string][]string{
		"users":          {"id", "name", "email", "created_at", "is_active"},
		"orders":         {"id", "user_id", "product_id", "quantity", "total", "created_at"},
		"products":       {"id", "name", "price", "description", "category"},
		"user_settings":  {"id", "user_id", "setting_key", "setting_value"},
	}
	meta.Functions = []string{"count", "sum", "avg", "max", "min", "now", "coalesce"}
	meta.Schemas = []string{"public", "auth", "billing"}
	meta.Databases = []string{"mydb", "testdb", "production"}
	meta.Datatypes = []string{"integer", "text", "boolean", "timestamp", "jsonb", "uuid"}
	meta.Specials = []string{`\dt`, `\di`, `\dv`, `\df`, `\dn`, `\du`, `\l`, `\x`, `\q`}
	meta.Favorites = []string{"active_users_query", "daily_report"}
	return meta
}

func TestFuzzyMatch_ExactPrefix(t *testing.T) {
	if !fuzzyMatch("use", "users") {
		t.Error("'use' should match 'users' (prefix)")
	}
}

func TestFuzzyMatch_Substring(t *testing.T) {
	if !fuzzyMatch("ser", "users") {
		t.Error("'ser' should match 'users' (substring)")
	}
}

func TestFuzzyMatch_FuzzyCharSequence(t *testing.T) {
	if !fuzzyMatch("djmi", "django_migrations") {
		t.Error("'djmi' should match 'django_migrations' (fuzzy)")
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	if !fuzzyMatch("SEL", "select") {
		t.Error("'SEL' should match 'select' (case insensitive)")
	}
	if !fuzzyMatch("sel", "SELECT") {
		t.Error("'sel' should match 'SELECT' (case insensitive)")
	}
}

func TestFuzzyMatch_Empty(t *testing.T) {
	if !fuzzyMatch("", "anything") {
		t.Error("empty input should match everything")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	if fuzzyMatch("xyz", "abc") {
		t.Error("'xyz' should not match 'abc'")
	}
}

func TestFuzzyMatch_UnderscoreWords(t *testing.T) {
	if !fuzzyMatch("us", "user_settings") {
		t.Error("'us' should match 'user_settings'")
	}
}

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		input, candidate string
		expectLower      int // score should be <= this
	}{
		{"users", "users", 0},             // exact
		{"use", "users", 1},               // prefix
		{"ser", "users", 2},               // substring
		{"djmi", "django_migrations", 3},  // fuzzy
	}

	for _, tt := range tests {
		score := FuzzyScore(tt.input, tt.candidate)
		if score > tt.expectLower {
			t.Errorf("FuzzyScore(%q, %q) = %d, want <= %d", tt.input, tt.candidate, score, tt.expectLower)
		}
	}
}

func TestComplete_SmartSelectColumns(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT ", 7)

	// Should suggest columns, functions, keywords (not just tables)
	hasColumn := false
	hasFunction := false
	hasKeyword := false
	for _, s := range suggestions {
		switch s.Type {
		case SuggestColumn:
			hasColumn = true
		case SuggestFunction:
			hasFunction = true
		case SuggestKeyword:
			hasKeyword = true
		}
	}

	if !hasColumn {
		t.Error("SELECT context should suggest columns")
	}
	if !hasFunction {
		t.Error("SELECT context should suggest functions")
	}
	if !hasKeyword {
		t.Error("SELECT context should suggest keywords")
	}
}

func TestComplete_SmartFromTables(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT * FROM ", 14)

	hasTable := false
	hasView := false
	for _, s := range suggestions {
		switch s.Type {
		case SuggestTable:
			hasTable = true
		case SuggestView:
			hasView = true
		}
	}

	if !hasTable {
		t.Error("FROM context should suggest tables")
	}
	if !hasView {
		t.Error("FROM context should suggest views")
	}
}

func TestComplete_SmartWhereColumns(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT * FROM users WHERE ", 26)

	hasColumn := false
	for _, s := range suggestions {
		if s.Type == SuggestColumn {
			hasColumn = true
			break
		}
	}

	if !hasColumn {
		t.Error("WHERE context should suggest columns")
	}
}

func TestComplete_SmartJoinTables(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT * FROM users JOIN ", 25)

	hasTable := false
	for _, s := range suggestions {
		if s.Type == SuggestTable {
			hasTable = true
			break
		}
	}

	if !hasTable {
		t.Error("JOIN context should suggest tables")
	}
}

func TestComplete_SmartOrderByColumns(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT * FROM users ORDER BY ", 29)

	hasColumn := false
	for _, s := range suggestions {
		if s.Type == SuggestColumn {
			hasColumn = true
			break
		}
	}

	if !hasColumn {
		t.Error("ORDER BY context should suggest columns")
	}
}

func TestComplete_SmartCreateTable(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("CREATE TABLE ", 13)

	hasTable := false
	hasKeyword := false
	for _, s := range suggestions {
		switch s.Type {
		case SuggestTable:
			hasTable = true
		case SuggestKeyword:
			hasKeyword = true
		}
	}

	if !hasTable {
		t.Error("CREATE context should suggest tables")
	}
	if !hasKeyword {
		t.Error("CREATE context should suggest keywords")
	}
}

func TestComplete_DotNotation(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	// After typing "users." should complete with user columns
	suggestions := comp.Complete("SELECT users.", 13)

	if len(suggestions) == 0 {
		t.Error("dot notation should return column suggestions")
	}

	for _, s := range suggestions {
		if s.Type != SuggestColumn {
			t.Errorf("dot notation should only suggest columns, got type %d", s.Type)
		}
	}
}

func TestComplete_BackslashCommand(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete(`\d`, 2)

	hasSpecial := false
	for _, s := range suggestions {
		if s.Type == SuggestSpecial {
			hasSpecial = true
			break
		}
	}

	if !hasSpecial {
		t.Error("backslash should suggest special commands")
	}
}

func TestComplete_FilterByPrefix(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("SELECT * FROM us", 16)

	for _, s := range suggestions {
		if s.Type == SuggestTable {
			if !fuzzyMatch("us", s.Text) {
				t.Errorf("suggestion %q should match prefix 'us'", s.Text)
			}
		}
	}
}

func TestComplete_NonSmart(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, false) // smart=false

	suggestions := comp.Complete("sel", 3)

	// Non-smart mode should return all types of completions
	types := make(map[SuggestionType]bool)
	for _, s := range suggestions {
		types[s.Type] = true
	}

	if len(types) < 2 {
		t.Error("non-smart mode should return multiple types of suggestions")
	}
}

func TestComplete_EmptyInput(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("", 0)

	// Empty input in smart mode should return all completions
	if len(suggestions) == 0 {
		t.Error("empty input should return suggestions")
	}
}

func TestComplete_InsertContext(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("INSERT INTO ", 12)

	hasTable := false
	for _, s := range suggestions {
		if s.Type == SuggestTable {
			hasTable = true
			break
		}
	}

	if !hasTable {
		t.Error("INSERT INTO context should suggest tables")
	}
}

func TestComplete_UpdateContext(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("UPDATE ", 7)

	hasTable := false
	for _, s := range suggestions {
		if s.Type == SuggestTable {
			hasTable = true
			break
		}
	}

	if !hasTable {
		t.Error("UPDATE context should suggest tables")
	}
}

func TestComplete_SetContext(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("UPDATE users SET ", 17)

	hasColumn := false
	for _, s := range suggestions {
		if s.Type == SuggestColumn {
			hasColumn = true
			break
		}
	}

	if !hasColumn {
		t.Error("SET context should suggest columns")
	}
}

func TestComplete_DropContext(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	suggestions := comp.Complete("DROP ", 5)

	hasTable := false
	hasKeyword := false
	for _, s := range suggestions {
		switch s.Type {
		case SuggestTable:
			hasTable = true
		case SuggestKeyword:
			hasKeyword = true
		}
	}

	if !hasTable {
		t.Error("DROP context should suggest tables")
	}
	if !hasKeyword {
		t.Error("DROP context should suggest keywords like TABLE, INDEX, etc.")
	}
}

func TestSetKeywordCasing(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, false)

	comp.SetKeywordCasing("upper")
	suggestions := comp.Complete("sel", 3)
	for _, s := range suggestions {
		if s.Type == SuggestKeyword && s.Text != strings.ToUpper(s.Text) {
			t.Errorf("with upper casing, keyword %q should be uppercase", s.Text)
		}
	}

	comp.SetKeywordCasing("lower")
	suggestions = comp.Complete("SEL", 3)
	for _, s := range suggestions {
		if s.Type == SuggestKeyword && s.Text != strings.ToLower(s.Text) {
			t.Errorf("with lower casing, keyword %q should be lowercase", s.Text)
		}
	}
}

func TestSetSmart(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	comp.SetSmart(false)
	suggestions := comp.Complete("SELECT ", 7)

	// In non-smart mode, all types should be suggested
	types := make(map[SuggestionType]bool)
	for _, s := range suggestions {
		types[s.Type] = true
	}
	if len(types) < 3 {
		t.Error("non-smart should suggest many types")
	}
}

func TestUpdateMetadata(t *testing.T) {
	meta := testMetadata()
	comp := NewCompleter(meta, true)

	// Update with new metadata
	newMeta := NewMetadata()
	newMeta.Tables = []string{"new_table"}
	comp.UpdateMetadata(newMeta)

	suggestions := comp.Complete("SELECT * FROM ", 14)
	found := false
	for _, s := range suggestions {
		if s.Text == "new_table" {
			found = true
		}
	}
	if !found {
		t.Error("should suggest from updated metadata")
	}
}

func TestLastWord(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT ", ""},
		{"SELECT * FROM us", "us"},
		{"SELECT u.", "u."},
		{`\dt`, `\dt`},
		{"", ""},
		{"SELECT * FROM users WHERE id", "id"},
		{"SELECT * FROM users WHERE ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := lastWord(tt.input)
			if result != tt.expected {
				t.Errorf("lastWord(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("SELECT * FROM users WHERE id = 1")
	expected := []string{"SELECT", "*", "FROM", "USERS", "WHERE", "ID", "=", "1"}

	if len(tokens) != len(expected) {
		t.Errorf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

func TestTokenize_WithQuotes(t *testing.T) {
	tokens := tokenize(`SELECT "Column Name" FROM 'table'`)
	// Quoted strings should be kept as single tokens
	found := false
	for _, tok := range tokens {
		if strings.Contains(tok, "COLUMN NAME") {
			found = true
		}
	}
	if !found {
		t.Errorf("quoted identifier should be a single token: %v", tokens)
	}
}

func TestExtractTableRefs(t *testing.T) {
	tokens := tokenize("SELECT * FROM USERS U JOIN ORDERS O ON U.ID = O.USER_ID")
	refs := extractTableRefs(tokens)

	if len(refs) < 2 {
		t.Fatalf("expected at least 2 table refs, got %d: %v", len(refs), refs)
	}

	// Check first ref
	if refs[0].Name != "users" {
		t.Errorf("first table should be 'users', got %q", refs[0].Name)
	}
	if refs[0].Alias != "u" {
		t.Errorf("first alias should be 'u', got %q", refs[0].Alias)
	}

	// Check second ref
	if refs[1].Name != "orders" {
		t.Errorf("second table should be 'orders', got %q", refs[1].Name)
	}
}

func TestAnalyzeContext_Select(t *testing.T) {
	ctx := analyzeContext("SELECT ")
	if !ctx.InSelect {
		t.Error("should detect SELECT context")
	}
}

func TestAnalyzeContext_From(t *testing.T) {
	ctx := analyzeContext("SELECT * FROM ")
	if !ctx.InFrom {
		t.Error("should detect FROM context")
	}
}

func TestAnalyzeContext_Where(t *testing.T) {
	ctx := analyzeContext("SELECT * FROM users WHERE ")
	if !ctx.InWhere {
		t.Error("should detect WHERE context")
	}
}

func TestAnalyzeContext_Join(t *testing.T) {
	ctx := analyzeContext("SELECT * FROM users JOIN ")
	if !ctx.InJoin {
		t.Error("should detect JOIN context")
	}
}

func TestAnalyzeContext_OrderBy(t *testing.T) {
	ctx := analyzeContext("SELECT * FROM users ORDER BY ")
	if !ctx.InOrderBy {
		t.Error("should detect ORDER BY context")
	}
}

func TestAnalyzeContext_GroupBy(t *testing.T) {
	ctx := analyzeContext("SELECT * FROM users GROUP BY ")
	if !ctx.InGroupBy {
		t.Error("should detect GROUP BY context")
	}
}

func TestAnalyzeContext_Backslash(t *testing.T) {
	ctx := analyzeContext(`\dt`)
	if !ctx.IsBackslash {
		t.Error("should detect backslash context")
	}
}

func TestAnalyzeContext_DotContext(t *testing.T) {
	ctx := analyzeContext("SELECT users.")
	if !ctx.AfterDot {
		t.Error("should detect dot context")
	}
	if ctx.BeforeDot != "users" {
		t.Errorf("BeforeDot should be 'users', got %q", ctx.BeforeDot)
	}
}

func TestAnalyzeContext_Empty(t *testing.T) {
	ctx := analyzeContext("")
	if ctx.InSelect || ctx.InFrom || ctx.InWhere || ctx.IsBackslash {
		t.Error("empty context should have no flags set")
	}
}

// Ensure strings import is used
var _ = strings.ToLower
