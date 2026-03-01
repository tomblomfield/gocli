// Package config handles configuration file parsing for both pgcli and mycli modes.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Config holds all configuration settings.
type Config struct {
	// Main settings
	SmartCompletion  bool
	MultiLine        bool
	MultiLineMode    string // "psql" or "safe"
	ViMode           bool
	KeyBindings      string // "emacs" or "vi"
	Timing           bool
	Pager            string
	EnablePager      bool
	TableFormat      string
	SyntaxStyle      string
	ExpandedOutput   bool
	AutoExpand       bool
	OnError          string // "STOP" or "RESUME"
	RowLimit         int
	MaxFieldWidth    int
	LessChatty       bool
	KeywordCasing    string // "auto", "upper", "lower"
	NullString       string
	Prompt           string
	PromptContinuation string
	WiderCompletion  bool
	ShowToolbar      bool
	LogFile          string
	LogLevel         string
	HistoryFile      string

	// Destructive warnings
	DestructiveWarning     bool
	DestructiveKeywords    []string

	// Named queries / favorites
	NamedQueries map[string]string

	// DSN aliases
	DSNAliases map[string]string

	// Color settings
	Colors map[string]string

	// Path to config file
	filePath string
}

// DefaultPGConfig returns default configuration for pgcli mode.
func DefaultPGConfig() *Config {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "pgcli")

	return &Config{
		SmartCompletion:  true,
		MultiLine:        false,
		MultiLineMode:    "psql",
		ViMode:           false,
		KeyBindings:      "emacs",
		Timing:           true,
		EnablePager:      true,
		Pager:            "less -SRXF",
		TableFormat:      "psql",
		SyntaxStyle:      "default",
		OnError:          "STOP",
		RowLimit:         1000,
		MaxFieldWidth:    500,
		KeywordCasing:    "auto",
		NullString:       "NULL",
		Prompt:           `\u@\h:\d> `,
		ShowToolbar:      true,
		LogLevel:         "INFO",
		HistoryFile:      filepath.Join(configDir, "history"),
		LogFile:          filepath.Join(configDir, "log"),
		DestructiveWarning: true,
		DestructiveKeywords: []string{"drop", "shutdown", "delete", "truncate", "alter", "update"},
		NamedQueries:     make(map[string]string),
		DSNAliases:       make(map[string]string),
		Colors:           make(map[string]string),
		filePath:         filepath.Join(configDir, "config"),
	}
}

// DefaultMySQLConfig returns default configuration for mycli mode.
func DefaultMySQLConfig() *Config {
	home, _ := os.UserHomeDir()

	return &Config{
		SmartCompletion:  true,
		MultiLine:        false,
		MultiLineMode:    "psql",
		ViMode:           false,
		KeyBindings:      "emacs",
		Timing:           true,
		EnablePager:      true,
		Pager:            "less -SRXF",
		TableFormat:      "ascii",
		SyntaxStyle:      "default",
		OnError:          "STOP",
		RowLimit:         1000,
		MaxFieldWidth:    500,
		KeywordCasing:    "auto",
		NullString:       "NULL",
		Prompt:           `\t \u@\h:\d> `,
		PromptContinuation: "-> ",
		ShowToolbar:      true,
		LogLevel:         "INFO",
		HistoryFile:      filepath.Join(home, ".mycli-history"),
		LogFile:          filepath.Join(home, ".mycli.log"),
		DestructiveWarning: true,
		DestructiveKeywords: []string{"drop", "shutdown", "delete", "truncate", "alter", "update"},
		NamedQueries:     make(map[string]string),
		DSNAliases:       make(map[string]string),
		Colors:           make(map[string]string),
		filePath:         filepath.Join(home, ".myclirc"),
	}
}

// PGConfigPaths returns the config file search paths for pgcli.
func PGConfigPaths() []string {
	home, _ := os.UserHomeDir()
	var paths []string

	// XDG config
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "pgcli", "config"))
	}
	paths = append(paths, filepath.Join(home, ".config", "pgcli", "config"))

	// Legacy location
	paths = append(paths, filepath.Join(home, ".pgclirc"))

	return paths
}

// MySQLConfigPaths returns the config file search paths for mycli.
func MySQLConfigPaths() []string {
	home, _ := os.UserHomeDir()
	var paths []string

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "mycli", "myclirc"))
	}
	paths = append(paths, filepath.Join(home, ".myclirc"))

	if runtime.GOOS != "windows" {
		paths = append(paths, "/etc/myclirc")
	}

	return paths
}

// Load reads the config file from disk.
func (c *Config) Load(path string) error {
	if path == "" {
		path = c.filePath
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Config file is optional
		}
		return fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close()

	c.filePath = path
	return c.parse(f)
}

// Save writes the config to disk.
func (c *Config) Save() error {
	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	f, err := os.Create(c.filePath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// Write main section
	fmt.Fprintln(w, "[main]")
	fmt.Fprintf(w, "smart_completion = %s\n", boolStr(c.SmartCompletion))
	fmt.Fprintf(w, "multi_line = %s\n", boolStr(c.MultiLine))
	fmt.Fprintf(w, "multi_line_mode = %s\n", c.MultiLineMode)
	fmt.Fprintf(w, "vi = %s\n", boolStr(c.ViMode))
	fmt.Fprintf(w, "timing = %s\n", boolStr(c.Timing))
	fmt.Fprintf(w, "table_format = %s\n", c.TableFormat)
	fmt.Fprintf(w, "syntax_style = %s\n", c.SyntaxStyle)
	fmt.Fprintf(w, "keyword_casing = %s\n", c.KeywordCasing)
	fmt.Fprintf(w, "row_limit = %d\n", c.RowLimit)
	fmt.Fprintf(w, "enable_pager = %s\n", boolStr(c.EnablePager))
	if c.Pager != "" {
		fmt.Fprintf(w, "pager = %s\n", c.Pager)
	}
	fmt.Fprintf(w, "less_chatty = %s\n", boolStr(c.LessChatty))
	fmt.Fprintf(w, "prompt = %s\n", c.Prompt)
	if c.PromptContinuation != "" {
		fmt.Fprintf(w, "prompt_continuation = %s\n", c.PromptContinuation)
	}
	fmt.Fprintf(w, "null_string = %s\n", c.NullString)
	fmt.Fprintf(w, "destructive_warning = %s\n", boolStr(c.DestructiveWarning))
	fmt.Fprintf(w, "log_level = %s\n", c.LogLevel)
	fmt.Fprintln(w)

	// Named queries
	if len(c.NamedQueries) > 0 {
		fmt.Fprintln(w, "[named queries]")
		for name, query := range c.NamedQueries {
			fmt.Fprintf(w, "%s = %s\n", name, query)
		}
		fmt.Fprintln(w)
	}

	// DSN aliases
	if len(c.DSNAliases) > 0 {
		fmt.Fprintln(w, "[alias_dsn]")
		for name, dsn := range c.DSNAliases {
			fmt.Fprintf(w, "%s = %s\n", name, dsn)
		}
		fmt.Fprintln(w)
	}

	return w.Flush()
}

func (c *Config) parse(f *os.File) error {
	scanner := bufio.NewScanner(f)
	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}

		// Key = Value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := stripQuotes(strings.TrimSpace(parts[1]))

		switch section {
		case "main", "":
			c.parseMainOption(key, value)
		case "named queries", "favorite_queries":
			c.NamedQueries[key] = value
		case "alias_dsn":
			c.DSNAliases[key] = value
		case "colors":
			c.Colors[key] = value
		}
	}

	return scanner.Err()
}

func (c *Config) parseMainOption(key, value string) {
	switch strings.ToLower(key) {
	case "smart_completion":
		c.SmartCompletion = parseBool(value)
	case "multi_line":
		c.MultiLine = parseBool(value)
	case "multi_line_mode":
		c.MultiLineMode = value
	case "vi":
		c.ViMode = parseBool(value)
	case "key_bindings":
		c.KeyBindings = value
		c.ViMode = value == "vi"
	case "timing":
		c.Timing = parseBool(value)
	case "pager":
		c.Pager = value
	case "enable_pager":
		c.EnablePager = parseBool(value)
	case "table_format":
		c.TableFormat = value
	case "syntax_style":
		c.SyntaxStyle = value
	case "expand", "expanded_output":
		c.ExpandedOutput = parseBool(value)
	case "auto_expand", "auto_vertical_output":
		c.AutoExpand = parseBool(value)
	case "on_error":
		c.OnError = value
	case "row_limit":
		if n, err := strconv.Atoi(value); err == nil {
			c.RowLimit = n
		}
	case "max_field_width":
		if n, err := strconv.Atoi(value); err == nil {
			c.MaxFieldWidth = n
		}
	case "less_chatty":
		c.LessChatty = parseBool(value)
	case "keyword_casing":
		c.KeywordCasing = value
	case "null_string":
		c.NullString = value
	case "prompt":
		c.Prompt = value
	case "prompt_continuation":
		c.PromptContinuation = value
	case "wider_completion_menu":
		c.WiderCompletion = parseBool(value)
	case "show_bottom_toolbar":
		c.ShowToolbar = parseBool(value)
	case "log_file":
		c.LogFile = value
	case "log_level":
		c.LogLevel = value
	case "history_file":
		c.HistoryFile = value
	case "destructive_warning":
		c.DestructiveWarning = parseBool(value)
	case "destructive_keywords":
		c.DestructiveKeywords = strings.Fields(value)
	}
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}

func boolStr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// FormatPrompt replaces prompt format tokens with actual values.
func FormatPrompt(promptFmt string, user, host, database, port string, isSuperuser bool) string {
	r := strings.NewReplacer(
		`\u`, user,
		`\h`, host,
		`\d`, database,
		`\p`, port,
		`\n`, "\n",
		`\_`, " ",
	)
	result := r.Replace(promptFmt)

	// Handle \# (superuser indicator)
	if isSuperuser {
		result = strings.ReplaceAll(result, `\#`, "@")
	} else {
		result = strings.ReplaceAll(result, `\#`, ">")
	}

	return result
}

// IsDestructive checks if a query matches destructive keywords.
func (c *Config) IsDestructive(query string) bool {
	if !c.DestructiveWarning {
		return false
	}
	upper := strings.ToUpper(strings.TrimSpace(query))
	words := strings.Fields(upper)
	if len(words) == 0 {
		return false
	}
	firstWord := strings.ToLower(words[0])
	for _, kw := range c.DestructiveKeywords {
		if firstWord == strings.ToLower(kw) {
			return true
		}
	}
	return false
}
