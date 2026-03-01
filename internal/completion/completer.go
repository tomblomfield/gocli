// Package completion provides context-aware SQL auto-completion.
package completion

import (
	"strings"
	"unicode"
)

// SuggestionType identifies the type of completion suggestion.
type SuggestionType int

const (
	SuggestKeyword SuggestionType = iota
	SuggestTable
	SuggestView
	SuggestColumn
	SuggestFunction
	SuggestSchema
	SuggestDatabase
	SuggestDatatype
	SuggestAlias
	SuggestSpecial
	SuggestFavorite
)

// Suggestion represents a single completion suggestion.
type Suggestion struct {
	Text        string
	DisplayText string
	Type        SuggestionType
	Description string
}

// SpecialCmd holds a special command name and its description.
type SpecialCmd struct {
	Name        string
	Description string
}

// Metadata holds database schema information for completions.
type Metadata struct {
	Tables    []string
	Views     []string
	Columns   map[string][]string // table -> columns
	Functions []string
	Schemas   []string
	Databases []string
	Datatypes []string
	Specials  []SpecialCmd
	Favorites []string
}

// NewMetadata creates empty metadata.
func NewMetadata() *Metadata {
	return &Metadata{
		Columns: make(map[string][]string),
	}
}

// Completer provides context-aware SQL completions.
type Completer struct {
	meta          *Metadata
	smart         bool
	keywordCasing string // "upper", "lower", "auto"
}

// NewCompleter creates a new SQL completer.
func NewCompleter(meta *Metadata, smart bool) *Completer {
	return &Completer{
		meta:          meta,
		smart:         smart,
		keywordCasing: "auto",
	}
}

// SetKeywordCasing sets how keywords are cased in completions.
func (c *Completer) SetKeywordCasing(casing string) {
	c.keywordCasing = casing
}

// SetSmart toggles smart (context-aware) completion.
func (c *Completer) SetSmart(smart bool) {
	c.smart = smart
}

// UpdateMetadata replaces the current metadata.
func (c *Completer) UpdateMetadata(meta *Metadata) {
	c.meta = meta
}

// Complete returns completion suggestions for the given input at the cursor position.
func (c *Completer) Complete(text string, cursorPos int) []Suggestion {
	if cursorPos > len(text) {
		cursorPos = len(text)
	}
	textBefore := text[:cursorPos]

	// Get the word being typed
	word := lastWord(textBefore)

	if !c.smart {
		return c.allCompletions(word)
	}

	// Parse context
	ctx := analyzeContext(textBefore)

	// If after a dot, the filter word is just the part after the dot
	if ctx.AfterDot {
		if dotIdx := strings.LastIndex(word, "."); dotIdx >= 0 {
			word = word[dotIdx+1:]
		}
	}

	return c.contextCompletions(ctx, word)
}

// SQLContext represents the parsed SQL context at cursor position.
type SQLContext struct {
	InSelect     bool
	InFrom       bool
	InJoin       bool
	InWhere      bool
	InOrderBy    bool
	InGroupBy    bool
	InHaving     bool
	InInsert     bool
	InUpdate     bool
	InSet        bool
	InOn         bool
	InUsing      bool
	InCreate     bool
	InAlter      bool
	InDrop       bool
	AfterDot     bool // schema.table or table.column context
	BeforeDot    string // the identifier before the dot
	Tables       []tableRef // tables referenced in query
	IsBackslash  bool
}

type tableRef struct {
	Name  string
	Alias string
}

func analyzeContext(text string) SQLContext {
	ctx := SQLContext{}
	text = strings.TrimSpace(text)

	if text == "" {
		return ctx
	}

	// Check for backslash command
	if strings.HasPrefix(text, `\`) {
		ctx.IsBackslash = true
		return ctx
	}

	tokens := tokenize(text)

	// Check for dot context
	if strings.HasSuffix(text, ".") {
		parts := strings.Split(lastWord(text[:len(text)-1]), ".")
		if len(parts) > 0 {
			ctx.AfterDot = true
			ctx.BeforeDot = strings.ToLower(parts[len(parts)-1])
		}
	}

	// Track tables referenced in the query
	ctx.Tables = extractTableRefs(tokens)

	// Determine clause context from the last significant keyword
	for i := len(tokens) - 1; i >= 0; i-- {
		tok := tokens[i]
		switch tok {
		case "SELECT":
			ctx.InSelect = true
			return ctx
		case "FROM":
			ctx.InFrom = true
			return ctx
		case "JOIN", "INNER", "LEFT", "RIGHT", "CROSS", "FULL", "NATURAL":
			if tok == "JOIN" || (i+1 < len(tokens) && tokens[i+1] == "JOIN") {
				ctx.InJoin = true
				return ctx
			}
		case "WHERE":
			ctx.InWhere = true
			return ctx
		case "ORDER":
			if i+1 < len(tokens) && tokens[i+1] == "BY" {
				ctx.InOrderBy = true
				return ctx
			}
		case "GROUP":
			if i+1 < len(tokens) && tokens[i+1] == "BY" {
				ctx.InGroupBy = true
				return ctx
			}
		case "HAVING":
			ctx.InHaving = true
			return ctx
		case "INSERT", "INTO":
			ctx.InInsert = true
			return ctx
		case "UPDATE":
			ctx.InUpdate = true
			return ctx
		case "SET":
			ctx.InSet = true
			return ctx
		case "ON":
			ctx.InOn = true
			return ctx
		case "USING":
			ctx.InUsing = true
			return ctx
		case "CREATE":
			ctx.InCreate = true
			return ctx
		case "ALTER":
			ctx.InAlter = true
			return ctx
		case "DROP":
			ctx.InDrop = true
			return ctx
		}
	}

	return ctx
}

func (c *Completer) contextCompletions(ctx SQLContext, word string) []Suggestion {
	var suggestions []Suggestion

	if ctx.IsBackslash {
		for _, s := range c.meta.Specials {
			if fuzzyMatch(word, s.Name) {
				suggestions = append(suggestions, Suggestion{
					Text: s.Name, Type: SuggestSpecial, Description: s.Description,
				})
			}
		}
		return suggestions
	}

	if ctx.AfterDot {
		// After a dot: suggest columns for the table/alias
		tableName := resolveTableName(ctx.BeforeDot, ctx.Tables)
		if cols, ok := c.meta.Columns[tableName]; ok {
			for _, col := range cols {
				if fuzzyMatch(word, col) {
					suggestions = append(suggestions, Suggestion{
						Text: col, Type: SuggestColumn, Description: tableName,
					})
				}
			}
		}
		return suggestions
	}

	switch {
	case ctx.InSelect:
		suggestions = append(suggestions, c.keywordSuggestions(word, selectKeywords)...)
		suggestions = append(suggestions, c.functionSuggestions(word)...)
		suggestions = append(suggestions, c.tableSuggestions(word)...)
		suggestions = append(suggestions, c.columnSuggestions(ctx, word)...)

	case ctx.InFrom, ctx.InJoin:
		suggestions = append(suggestions, c.keywordSuggestions(word, fromKeywords)...)
		suggestions = append(suggestions, c.tableSuggestions(word)...)
		suggestions = append(suggestions, c.viewSuggestions(word)...)
		suggestions = append(suggestions, c.schemaSuggestions(word)...)

	case ctx.InWhere, ctx.InHaving, ctx.InOn:
		suggestions = append(suggestions, c.keywordSuggestions(word, whereKeywords)...)
		suggestions = append(suggestions, c.columnSuggestions(ctx, word)...)
		suggestions = append(suggestions, c.functionSuggestions(word)...)

	case ctx.InOrderBy, ctx.InGroupBy:
		suggestions = append(suggestions, c.columnSuggestions(ctx, word)...)

	case ctx.InInsert:
		suggestions = append(suggestions, c.tableSuggestions(word)...)
		suggestions = append(suggestions, c.columnSuggestions(ctx, word)...)

	case ctx.InUpdate:
		suggestions = append(suggestions, c.tableSuggestions(word)...)

	case ctx.InSet:
		suggestions = append(suggestions, c.columnSuggestions(ctx, word)...)

	case ctx.InCreate, ctx.InAlter, ctx.InDrop:
		suggestions = append(suggestions, c.keywordSuggestions(word, ddlKeywords)...)
		suggestions = append(suggestions, c.tableSuggestions(word)...)
		suggestions = append(suggestions, c.schemaSuggestions(word)...)
		suggestions = append(suggestions, c.datatypeSuggestions(word)...)

	default:
		suggestions = append(suggestions, c.allCompletions(word)...)
	}

	return suggestions
}

func (c *Completer) allCompletions(word string) []Suggestion {
	var suggestions []Suggestion
	suggestions = append(suggestions, c.keywordSuggestions(word, allKeywords)...)
	suggestions = append(suggestions, c.tableSuggestions(word)...)
	suggestions = append(suggestions, c.viewSuggestions(word)...)
	suggestions = append(suggestions, c.columnSuggestionsAll(word)...)
	suggestions = append(suggestions, c.functionSuggestions(word)...)
	suggestions = append(suggestions, c.schemaSuggestions(word)...)
	suggestions = append(suggestions, c.datatypeSuggestions(word)...)
	for _, s := range c.meta.Specials {
		if fuzzyMatch(word, s.Name) {
			suggestions = append(suggestions, Suggestion{Text: s.Name, Type: SuggestSpecial, Description: s.Description})
		}
	}
	for _, f := range c.meta.Favorites {
		if fuzzyMatch(word, f) {
			suggestions = append(suggestions, Suggestion{Text: f, Type: SuggestFavorite})
		}
	}
	return suggestions
}

func (c *Completer) tableSuggestions(word string) []Suggestion {
	var s []Suggestion
	for _, t := range c.meta.Tables {
		if fuzzyMatch(word, t) {
			s = append(s, Suggestion{Text: t, Type: SuggestTable, Description: "table"})
		}
	}
	return s
}

func (c *Completer) viewSuggestions(word string) []Suggestion {
	var s []Suggestion
	for _, v := range c.meta.Views {
		if fuzzyMatch(word, v) {
			s = append(s, Suggestion{Text: v, Type: SuggestView, Description: "view"})
		}
	}
	return s
}

func (c *Completer) columnSuggestions(ctx SQLContext, word string) []Suggestion {
	var s []Suggestion
	if len(ctx.Tables) > 0 {
		for _, tRef := range ctx.Tables {
			tableName := tRef.Name
			if cols, ok := c.meta.Columns[tableName]; ok {
				for _, col := range cols {
					if fuzzyMatch(word, col) {
						s = append(s, Suggestion{Text: col, Type: SuggestColumn, Description: tableName})
					}
				}
			}
		}
	}
	if len(s) == 0 {
		// Fall back to all columns
		s = c.columnSuggestionsAll(word)
	}
	return s
}

func (c *Completer) columnSuggestionsAll(word string) []Suggestion {
	var s []Suggestion
	for table, cols := range c.meta.Columns {
		for _, col := range cols {
			if fuzzyMatch(word, col) {
				s = append(s, Suggestion{Text: col, Type: SuggestColumn, Description: table})
			}
		}
	}
	return s
}

func (c *Completer) functionSuggestions(word string) []Suggestion {
	var s []Suggestion
	for _, f := range c.meta.Functions {
		if fuzzyMatch(word, f) {
			s = append(s, Suggestion{Text: f + "()", Type: SuggestFunction, Description: "function"})
		}
	}
	return s
}

func (c *Completer) schemaSuggestions(word string) []Suggestion {
	var s []Suggestion
	for _, sc := range c.meta.Schemas {
		if fuzzyMatch(word, sc) {
			s = append(s, Suggestion{Text: sc, Type: SuggestSchema, Description: "schema"})
		}
	}
	return s
}

func (c *Completer) datatypeSuggestions(word string) []Suggestion {
	var s []Suggestion
	for _, dt := range c.meta.Datatypes {
		if fuzzyMatch(word, dt) {
			s = append(s, Suggestion{Text: dt, Type: SuggestDatatype, Description: "type"})
		}
	}
	return s
}

func (c *Completer) keywordSuggestions(word string, keywords []string) []Suggestion {
	var s []Suggestion
	for _, kw := range keywords {
		if fuzzyMatch(word, kw) {
			text := c.caseKeyword(kw, word)
			s = append(s, Suggestion{Text: text, Type: SuggestKeyword, Description: "keyword"})
		}
	}
	return s
}

func (c *Completer) caseKeyword(kw, input string) string {
	switch c.keywordCasing {
	case "upper":
		return strings.ToUpper(kw)
	case "lower":
		return strings.ToLower(kw)
	default: // auto
		if input == "" {
			return strings.ToUpper(kw)
		}
		if input == strings.ToUpper(input) {
			return strings.ToUpper(kw)
		}
		return strings.ToLower(kw)
	}
}

// fuzzyMatch implements fuzzy matching similar to pgcli/mycli.
func fuzzyMatch(input, candidate string) bool {
	if input == "" {
		return true
	}
	inputLower := strings.ToLower(input)
	candidateLower := strings.ToLower(candidate)

	// Exact prefix match
	if strings.HasPrefix(candidateLower, inputLower) {
		return true
	}

	// Substring match
	if strings.Contains(candidateLower, inputLower) {
		return true
	}

	// Fuzzy character sequence match (like pgcli)
	j := 0
	for i := 0; i < len(candidateLower) && j < len(inputLower); i++ {
		if candidateLower[i] == inputLower[j] {
			j++
		}
	}
	return j == len(inputLower)
}

// FuzzyScore returns a score for how well input matches candidate (lower is better).
func FuzzyScore(input, candidate string) int {
	if input == "" {
		return 100
	}
	inputLower := strings.ToLower(input)
	candidateLower := strings.ToLower(candidate)

	// Exact match
	if candidateLower == inputLower {
		return 0
	}

	// Prefix match
	if strings.HasPrefix(candidateLower, inputLower) {
		return 1
	}

	// Substring match
	if strings.Contains(candidateLower, inputLower) {
		return 2
	}

	// Fuzzy match - score by gap
	return 3
}

// lastWord extracts the last word (identifier) being typed.
func lastWord(text string) string {
	if text == "" {
		return ""
	}

	// If text ends with whitespace, user is starting a new word
	lastChar := rune(text[len(text)-1])
	if lastChar == ' ' || lastChar == '\t' || lastChar == '\n' {
		return ""
	}

	// Walk backwards to find word start
	i := len(text) - 1
	for i >= 0 {
		r := rune(text[i])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '\\' || r == '#' {
			i--
		} else {
			break
		}
	}
	return text[i+1:]
}

func tokenize(sql string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range sql {
		if inQuote {
			if r == quoteChar {
				inQuote = false
			}
			current.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = true
			quoteChar = r
			current.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) || r == ',' || r == '(' || r == ')' || r == ';' {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToUpper(current.String()))
				current.Reset()
			}
			continue
		}
		// Operators like =, <, > become their own tokens
		if r == '=' || r == '<' || r == '>' || r == '!' {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToUpper(current.String()))
				current.Reset()
			}
			tokens = append(tokens, string(r))
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToUpper(current.String()))
	}
	return tokens
}

func extractTableRefs(tokens []string) []tableRef {
	var refs []tableRef
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "FROM" || tok == "JOIN" || tok == "UPDATE" || tok == "INTO" {
			if i+1 < len(tokens) {
				name := tokens[i+1]
				if isKeyword(name) {
					continue
				}
				ref := tableRef{Name: strings.ToLower(name)}
				// Check for alias
				if i+2 < len(tokens) {
					next := tokens[i+2]
					if next == "AS" && i+3 < len(tokens) {
						ref.Alias = strings.ToLower(tokens[i+3])
					} else if !isKeyword(next) && next != "," && next != "ON" && next != "WHERE" && next != "SET" {
						ref.Alias = strings.ToLower(next)
					}
				}
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

func resolveTableName(alias string, refs []tableRef) string {
	for _, ref := range refs {
		if ref.Alias == alias {
			return ref.Name
		}
		if ref.Name == alias {
			return ref.Name
		}
	}
	return alias
}

func isKeyword(s string) bool {
	upper := strings.ToUpper(s)
	for _, kw := range allKeywords {
		if kw == upper {
			return true
		}
	}
	return false
}

// SQL keyword lists

var selectKeywords = []string{
	"DISTINCT", "ALL", "AS", "FROM", "WHERE", "GROUP", "BY",
	"HAVING", "ORDER", "LIMIT", "OFFSET", "UNION", "INTERSECT",
	"EXCEPT", "CASE", "WHEN", "THEN", "ELSE", "END", "AND",
	"OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE", "ILIKE",
	"IS", "NULL", "TRUE", "FALSE", "ASC", "DESC", "NULLS",
	"FIRST", "LAST", "OVER", "PARTITION", "ROWS", "RANGE",
	"UNBOUNDED", "PRECEDING", "FOLLOWING", "CURRENT", "ROW",
	"FILTER", "WITHIN", "LATERAL", "CROSS", "NATURAL",
}

var fromKeywords = []string{
	"JOIN", "INNER", "LEFT", "RIGHT", "FULL", "OUTER", "CROSS",
	"NATURAL", "ON", "USING", "WHERE", "GROUP", "BY", "HAVING",
	"ORDER", "LIMIT", "OFFSET", "AS", "UNION", "INTERSECT",
	"EXCEPT",
}

var whereKeywords = []string{
	"AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE",
	"ILIKE", "IS", "NULL", "TRUE", "FALSE", "ANY", "ALL",
	"SOME", "GROUP", "BY", "HAVING", "ORDER", "LIMIT", "OFFSET",
}

var ddlKeywords = []string{
	"TABLE", "INDEX", "VIEW", "MATERIALIZED", "SEQUENCE",
	"SCHEMA", "DATABASE", "FUNCTION", "PROCEDURE", "TRIGGER",
	"TYPE", "DOMAIN", "EXTENSION", "IF", "EXISTS", "NOT",
	"CASCADE", "RESTRICT", "COLUMN", "CONSTRAINT", "PRIMARY",
	"KEY", "UNIQUE", "REFERENCES", "FOREIGN", "CHECK", "DEFAULT",
}

var allKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER",
	"DROP", "TRUNCATE", "FROM", "WHERE", "JOIN", "INNER", "LEFT",
	"RIGHT", "FULL", "OUTER", "CROSS", "NATURAL", "ON", "USING",
	"GROUP", "BY", "HAVING", "ORDER", "LIMIT", "OFFSET", "AS",
	"DISTINCT", "ALL", "UNION", "INTERSECT", "EXCEPT", "AND",
	"OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE", "ILIKE",
	"IS", "NULL", "TRUE", "FALSE", "CASE", "WHEN", "THEN",
	"ELSE", "END", "ASC", "DESC", "NULLS", "FIRST", "LAST",
	"INTO", "VALUES", "SET", "DEFAULT", "RETURNING", "WITH",
	"RECURSIVE", "TABLE", "INDEX", "VIEW", "MATERIALIZED",
	"SEQUENCE", "SCHEMA", "DATABASE", "FUNCTION", "PROCEDURE",
	"TRIGGER", "TYPE", "DOMAIN", "EXTENSION", "IF", "CASCADE",
	"RESTRICT", "COLUMN", "CONSTRAINT", "PRIMARY", "KEY",
	"UNIQUE", "REFERENCES", "FOREIGN", "CHECK", "BEGIN",
	"COMMIT", "ROLLBACK", "SAVEPOINT", "GRANT", "REVOKE",
	"EXPLAIN", "ANALYZE", "VERBOSE", "COSTS", "BUFFERS",
	"FORMAT", "COPY", "TO", "STDIN", "STDOUT", "DELIMITER",
	"CSV", "HEADER", "QUOTE", "ESCAPE", "FORCE", "VACUUM",
	"REINDEX", "CLUSTER", "COMMENT", "OVER", "PARTITION",
	"ROWS", "RANGE", "UNBOUNDED", "PRECEDING", "FOLLOWING",
	"CURRENT", "ROW", "FILTER", "WITHIN", "LATERAL",
	"FETCH", "NEXT", "PRIOR", "ABSOLUTE", "RELATIVE",
	"FORWARD", "BACKWARD", "DECLARE", "CURSOR", "FOR",
	"CLOSE", "MOVE",
}
