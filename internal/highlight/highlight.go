// Package highlight provides SQL syntax highlighting for terminal output.
package highlight

import (
	"strings"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Blue      = "\033[34m"
	Cyan      = "\033[36m"
	Green     = "\033[32m"
	Magenta   = "\033[35m"
	Red       = "\033[31m"
	Yellow    = "\033[33m"
	White     = "\033[37m"
	BrightBlue    = "\033[94m"
	BrightCyan    = "\033[96m"
	BrightGreen   = "\033[92m"
	BrightMagenta = "\033[95m"
	BrightYellow  = "\033[93m"
	BrightWhite   = "\033[97m"
)

// Style controls which colors are used for different token types.
type Style struct {
	Keyword  string
	Function string
	String   string
	Number   string
	Comment  string
	Operator string
	Name     string
}

// DefaultStyle returns the default color scheme.
func DefaultStyle() Style {
	return Style{
		Keyword:  BrightBlue + Bold,
		Function: BrightCyan,
		String:   BrightGreen,
		Number:   BrightYellow,
		Comment:  Dim + Green,
		Operator: BrightWhite,
		Name:     White,
	}
}

// MonokaiStyle returns a monokai-inspired color scheme.
func MonokaiStyle() Style {
	return Style{
		Keyword:  BrightMagenta + Bold,
		Function: BrightGreen,
		String:   BrightYellow,
		Number:   BrightMagenta,
		Comment:  Dim,
		Operator: Red,
		Name:     White,
	}
}

// GetStyle returns a style by name.
func GetStyle(name string) Style {
	switch strings.ToLower(name) {
	case "monokai":
		return MonokaiStyle()
	case "native":
		return Style{
			Keyword: BrightBlue + Bold, Function: BrightGreen,
			String: BrightYellow, Number: BrightMagenta,
			Comment: Dim + Cyan, Operator: White, Name: White,
		}
	case "vim":
		return Style{
			Keyword: BrightYellow + Bold, Function: BrightCyan,
			String: BrightGreen, Number: BrightMagenta,
			Comment: Dim + Blue, Operator: White, Name: White,
		}
	case "fruity":
		return Style{
			Keyword: BrightGreen + Bold, Function: BrightYellow,
			String: BrightCyan, Number: BrightMagenta,
			Comment: Dim + Red, Operator: White, Name: White,
		}
	default:
		return DefaultStyle()
	}
}

// TokenType represents a type of SQL token.
type TokenType int

const (
	TokenKeyword TokenType = iota
	TokenFunction
	TokenString
	TokenNumber
	TokenComment
	TokenOperator
	TokenName
	TokenWhitespace
	TokenPunctuation
)

// Token represents a lexed SQL token.
type Token struct {
	Type  TokenType
	Value string
}

// Highlight applies syntax highlighting to a SQL string.
func Highlight(sql string, style Style) string {
	tokens := Tokenize(sql)
	var result strings.Builder
	for _, tok := range tokens {
		switch tok.Type {
		case TokenKeyword:
			result.WriteString(style.Keyword)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		case TokenFunction:
			result.WriteString(style.Function)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		case TokenString:
			result.WriteString(style.String)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		case TokenNumber:
			result.WriteString(style.Number)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		case TokenComment:
			result.WriteString(style.Comment)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		case TokenOperator:
			result.WriteString(style.Operator)
			result.WriteString(tok.Value)
			result.WriteString(Reset)
		default:
			result.WriteString(tok.Value)
		}
	}
	return result.String()
}

// Tokenize splits SQL into tokens for syntax highlighting.
func Tokenize(sql string) []Token {
	var tokens []Token
	i := 0

	for i < len(sql) {
		ch := sql[i]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			start := i
			for i < len(sql) && (sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r') {
				i++
			}
			tokens = append(tokens, Token{TokenWhitespace, sql[start:i]})
			continue
		}

		// Single-line comment (--)
		if ch == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			start := i
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			tokens = append(tokens, Token{TokenComment, sql[start:i]})
			continue
		}

		// Multi-line comment (/* */)
		if ch == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			start := i
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < len(sql) {
				i += 2
			}
			tokens = append(tokens, Token{TokenComment, sql[start:i]})
			continue
		}

		// String literal
		if ch == '\'' {
			start := i
			i++
			for i < len(sql) {
				if sql[i] == '\'' {
					if i+1 < len(sql) && sql[i+1] == '\'' {
						i += 2 // escaped quote
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{TokenString, sql[start:i]})
			continue
		}

		// Dollar-quoted string (PostgreSQL)
		if ch == '$' {
			// Check for $tag$...$tag$ pattern
			start := i
			tag := readDollarTag(sql, i)
			if tag != "" {
				i += len(tag)
				end := strings.Index(sql[i:], tag)
				if end >= 0 {
					i += end + len(tag)
				} else {
					i = len(sql)
				}
				tokens = append(tokens, Token{TokenString, sql[start:i]})
				continue
			}
			// Just a dollar sign
			tokens = append(tokens, Token{TokenOperator, "$"})
			i++
			continue
		}

		// Double-quoted identifier
		if ch == '"' {
			start := i
			i++
			for i < len(sql) && sql[i] != '"' {
				i++
			}
			if i < len(sql) {
				i++
			}
			tokens = append(tokens, Token{TokenName, sql[start:i]})
			continue
		}

		// Backtick-quoted identifier (MySQL)
		if ch == '`' {
			start := i
			i++
			for i < len(sql) && sql[i] != '`' {
				i++
			}
			if i < len(sql) {
				i++
			}
			tokens = append(tokens, Token{TokenName, sql[start:i]})
			continue
		}

		// Number
		if ch >= '0' && ch <= '9' {
			start := i
			for i < len(sql) && ((sql[i] >= '0' && sql[i] <= '9') || sql[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{TokenNumber, sql[start:i]})
			continue
		}

		// Word (identifier or keyword)
		if isIdentStart(ch) {
			start := i
			for i < len(sql) && isIdentPart(sql[i]) {
				i++
			}
			word := sql[start:i]
			if isKeyword(strings.ToUpper(word)) {
				tokens = append(tokens, Token{TokenKeyword, word})
			} else if isFunction(strings.ToUpper(word)) && i < len(sql) && sql[i] == '(' {
				tokens = append(tokens, Token{TokenFunction, word})
			} else {
				tokens = append(tokens, Token{TokenName, word})
			}
			continue
		}

		// Operators and punctuation
		if isOperator(ch) {
			tokens = append(tokens, Token{TokenOperator, string(ch)})
			i++
			continue
		}

		// Punctuation
		tokens = append(tokens, Token{TokenPunctuation, string(ch)})
		i++
	}

	return tokens
}

func readDollarTag(sql string, pos int) string {
	if pos >= len(sql) || sql[pos] != '$' {
		return ""
	}
	end := pos + 1
	for end < len(sql) && (isIdentPart(sql[end]) || sql[end] == '$') {
		if sql[end] == '$' {
			return sql[pos : end+1]
		}
		end++
	}
	if end < len(sql) && sql[end] == '$' {
		return sql[pos : end+1]
	}
	return ""
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '\\'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '#'
}

func isOperator(ch byte) bool {
	return ch == '+' || ch == '-' || ch == '*' || ch == '/' ||
		ch == '=' || ch == '<' || ch == '>' || ch == '!' ||
		ch == '~' || ch == '&' || ch == '|' || ch == '^'
}

func isKeyword(word string) bool {
	_, ok := sqlKeywords[word]
	return ok
}

func isFunction(word string) bool {
	_, ok := sqlFunctions[word]
	return ok
}

var sqlKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true,
	"NOT": true, "INSERT": true, "INTO": true, "VALUES": true, "UPDATE": true,
	"SET": true, "DELETE": true, "CREATE": true, "ALTER": true, "DROP": true,
	"TABLE": true, "INDEX": true, "VIEW": true, "DATABASE": true, "SCHEMA": true,
	"IF": true, "EXISTS": true, "PRIMARY": true, "KEY": true, "FOREIGN": true,
	"REFERENCES": true, "UNIQUE": true, "CHECK": true, "DEFAULT": true,
	"NULL": true, "CONSTRAINT": true, "CASCADE": true, "RESTRICT": true,
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
	"OUTER": true, "CROSS": true, "NATURAL": true, "ON": true, "USING": true,
	"GROUP": true, "BY": true, "HAVING": true, "ORDER": true, "ASC": true,
	"DESC": true, "LIMIT": true, "OFFSET": true, "UNION": true, "ALL": true,
	"DISTINCT": true, "AS": true, "IN": true, "BETWEEN": true, "LIKE": true,
	"ILIKE": true, "IS": true, "TRUE": true, "FALSE": true, "CASE": true,
	"WHEN": true, "THEN": true, "ELSE": true, "END": true, "BEGIN": true,
	"COMMIT": true, "ROLLBACK": true, "SAVEPOINT": true, "GRANT": true,
	"REVOKE": true, "WITH": true, "RECURSIVE": true, "RETURNING": true,
	"EXPLAIN": true, "ANALYZE": true, "VERBOSE": true, "COSTS": true,
	"BUFFERS": true, "FORMAT": true, "COPY": true, "TO": true, "STDIN": true,
	"STDOUT": true, "DELIMITER": true, "CSV": true, "HEADER": true,
	"QUOTE": true, "ESCAPE": true, "FORCE": true, "VACUUM": true,
	"REINDEX": true, "CLUSTER": true, "COMMENT": true, "TRUNCATE": true,
	"FUNCTION": true, "PROCEDURE": true, "TRIGGER": true, "TYPE": true,
	"DOMAIN": true, "EXTENSION": true, "SEQUENCE": true, "MATERIALIZED": true,
	"OVER": true, "PARTITION": true, "ROWS": true, "RANGE": true,
	"UNBOUNDED": true, "PRECEDING": true, "FOLLOWING": true, "CURRENT": true,
	"ROW": true, "FILTER": true, "WITHIN": true, "LATERAL": true,
	"FETCH": true, "NEXT": true, "PRIOR": true, "ABSOLUTE": true,
	"RELATIVE": true, "FORWARD": true, "BACKWARD": true, "DECLARE": true,
	"CURSOR": true, "FOR": true, "CLOSE": true, "MOVE": true,
	"INTERSECT": true, "EXCEPT": true, "WINDOW": true, "COLUMN": true,
	"ADD": true, "RENAME": true, "OWNER": true, "REPLACE": true,
	"TEMP": true, "TEMPORARY": true, "UNLOGGED": true, "CONCURRENTLY": true,
	"ONLY": true, "SOME": true, "ANY": true,
	// MySQL-specific
	"USE": true, "SHOW": true, "DESCRIBE": true, "DATABASES": true,
	"TABLES": true, "COLUMNS": true, "STATUS": true, "ENGINE": true,
	"AUTO_INCREMENT": true, "CHARSET": true, "COLLATE": true,
	"ENUM": true, "LOAD": true, "DATA": true, "INFILE": true,
	"LOCAL": true, "IGNORE": true,
}

var sqlFunctions = map[string]bool{
	// Aggregate
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"ARRAY_AGG": true, "STRING_AGG": true, "JSON_AGG": true, "JSONB_AGG": true,
	"BOOL_AND": true, "BOOL_OR": true, "EVERY": true,
	// String
	"LENGTH": true, "UPPER": true, "LOWER": true, "TRIM": true,
	"SUBSTRING": true, "CONCAT": true, "CONCAT_WS": true,
	"LPAD": true, "RPAD": true, "REVERSE": true,
	"SPLIT_PART": true, "REGEXP_REPLACE": true, "REGEXP_MATCHES": true,
	"TRANSLATE": true, "REPEAT": true, "POSITION": true, "STRPOS": true,
	"INITCAP": true, "CHR": true, "ASCII": true, "ENCODE": true,
	"DECODE": true, "MD5": true, "FORMAT": false,
	// Numeric
	"ABS": true, "CEIL": true, "CEILING": true, "FLOOR": true,
	"ROUND": true, "TRUNC": true, "MOD": true, "POWER": true,
	"SQRT": true, "RANDOM": true, "SIGN": true, "PI": true,
	"LOG": true, "LN": true, "EXP": true, "GREATEST": true,
	"LEAST": true,
	// Date/Time
	"NOW": true, "CURRENT_TIMESTAMP": true, "CURRENT_DATE": true,
	"CURRENT_TIME": true, "AGE": true, "DATE_PART": true,
	"DATE_TRUNC": true, "EXTRACT": true, "TO_CHAR": true,
	"TO_DATE": true, "TO_TIMESTAMP": true, "TO_NUMBER": true,
	"MAKE_DATE": true, "MAKE_TIME": true, "MAKE_TIMESTAMP": true,
	"CLOCK_TIMESTAMP": true, "STATEMENT_TIMESTAMP": true,
	"TIMEOFDAY": true,
	// JSON
	"JSON_BUILD_OBJECT": true, "JSON_BUILD_ARRAY": true,
	"JSONB_BUILD_OBJECT": true, "JSONB_BUILD_ARRAY": true,
	"JSON_OBJECT": true, "JSON_ARRAY_LENGTH": true,
	"JSONB_OBJECT": true, "JSONB_SET": true, "JSONB_INSERT": true,
	"JSON_EXTRACT_PATH": true, "JSON_EXTRACT_PATH_TEXT": true,
	"ROW_TO_JSON": true, "TO_JSON": true, "TO_JSONB": true,
	// Type casting
	"CAST": true, "COALESCE": true, "NULLIF": true,
	// Window
	"ROW_NUMBER": true, "RANK": true, "DENSE_RANK": true,
	"PERCENT_RANK": true, "CUME_DIST": true, "NTILE": true,
	"LAG": true, "LEAD": true, "FIRST_VALUE": true,
	"LAST_VALUE": true, "NTH_VALUE": true,
	// Conditional
	"IF": false, "IFNULL": true, "IIF": true,
	// System
	"VERSION": true, "CURRENT_USER": true, "SESSION_USER": true,
	"CURRENT_SCHEMA": true, "CURRENT_DATABASE": true,
	"PG_TYPEOF": true, "PG_SIZE_PRETTY": true,
	// MySQL specific
	"GROUP_CONCAT": true, "FOUND_ROWS": true, "LAST_INSERT_ID": true,
	"CONNECTION_ID": true, "DATABASE": false, "USER": true,
	"CURDATE": true, "CURTIME": true, "SYSDATE": true,
	"DATE_FORMAT": true, "STR_TO_DATE": true, "DATEDIFF": true,
	"DATE_ADD": true, "DATE_SUB": true, "ADDDATE": true,
	"SUBDATE": true, "TIMESTAMPDIFF": true,
	"CHAR_LENGTH": true, "CHARACTER_LENGTH": true,
	"LOCATE": true, "INSTR": true, "FIELD": true,
	"ELT": true, "MAKE_SET": true,
	"CONV": true, "HEX": true, "UNHEX": true, "BIN": true, "OCT": true,
	"SHA1": true, "SHA2": true, "CRC32": true,
	"INET_ATON": true, "INET_NTOA": true,
	"UUID": true, "SLEEP": true, "BENCHMARK": true,
}
