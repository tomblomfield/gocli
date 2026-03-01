// Package cli implements the interactive REPL (Read-Eval-Print Loop) for gocli.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/tomblomfield/gocli/internal/completion"
	"github.com/tomblomfield/gocli/internal/config"
	"github.com/tomblomfield/gocli/internal/format"
	"github.com/tomblomfield/gocli/internal/highlight"
	"github.com/tomblomfield/gocli/internal/special"
)

// DBMode specifies the database type.
type DBMode string

const (
	PostgreSQL DBMode = "postgresql"
	MySQL      DBMode = "mysql"
)

// Executor is the interface that database executors must implement.
type Executor interface {
	Execute(ctx context.Context, query string) (*format.QueryResult, error)
	Close() error
	Database() string
	ServerVersion() (string, error)
}

// MetadataProvider provides schema metadata for completions.
type MetadataProvider interface {
	Tables(ctx context.Context, schema string) ([]string, error)
	Columns(ctx context.Context, table string) ([]string, error)
	Schemas(ctx context.Context) ([]string, error)
	Functions(ctx context.Context, schema string) ([]string, error)
	Databases(ctx context.Context) ([]string, error)
	Datatypes(ctx context.Context) []string
}

// App is the main CLI application.
type App struct {
	mode     DBMode
	executor Executor
	meta     MetadataProvider
	config   *config.Config
	special  *special.Registry
	comp     *completion.Completer
	compMeta *completion.Metadata

	// State
	multiLineBuffer strings.Builder
	inMultiLine     bool
	lastQuery       string
	outputFile      *os.File

	// I/O (can be overridden for testing)
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewApp creates a new CLI application.
func NewApp(mode DBMode, executor Executor, meta MetadataProvider, cfg *config.Config) *App {
	reg := special.NewRegistry()
	reg.Timing = cfg.Timing
	reg.Pager = cfg.Pager

	switch mode {
	case PostgreSQL:
		special.RegisterPG(reg)
	case MySQL:
		special.RegisterMySQL(reg)
	}

	compMeta := completion.NewMetadata()
	comp := completion.NewCompleter(compMeta, cfg.SmartCompletion)
	comp.SetKeywordCasing(cfg.KeywordCasing)

	app := &App{
		mode:     mode,
		executor: executor,
		meta:     meta,
		config:   cfg,
		special:  reg,
		comp:     comp,
		compMeta: compMeta,
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}

	// Load favorites into special registry
	for name, query := range cfg.NamedQueries {
		reg.Favorites[name] = query
	}

	return app
}

// RefreshCompletions reloads schema metadata for auto-completion.
func (a *App) RefreshCompletions() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	meta := completion.NewMetadata()

	if tables, err := a.meta.Tables(ctx, ""); err == nil {
		meta.Tables = tables
	}
	if schemas, err := a.meta.Schemas(ctx); err == nil {
		meta.Schemas = schemas
	}
	if funcs, err := a.meta.Functions(ctx, ""); err == nil {
		meta.Functions = funcs
	}
	if dbs, err := a.meta.Databases(ctx); err == nil {
		meta.Databases = dbs
	}
	meta.Datatypes = a.meta.Datatypes(ctx)

	// Load columns for each table
	for _, table := range meta.Tables {
		if cols, err := a.meta.Columns(ctx, table); err == nil {
			meta.Columns[table] = cols
		}
	}

	// Add special commands
	for _, cmd := range a.special.Commands() {
		meta.Specials = append(meta.Specials, completion.SpecialCmd{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}

	// Add favorites
	for name := range a.special.Favorites {
		meta.Favorites = append(meta.Favorites, name)
	}

	a.compMeta = meta
	a.comp.UpdateMetadata(meta)
}

// Complete returns completions for the given text at cursor position.
func (a *App) Complete(text string, cursorPos int) []completion.Suggestion {
	return a.comp.Complete(text, cursorPos)
}

// HandleInput processes a line of input from the user.
func (a *App) HandleInput(input string) (shouldQuit bool) {
	input = strings.TrimRight(input, "\n\r")

	// Handle multi-line mode
	if a.config.MultiLine {
		return a.handleMultiLine(input)
	}

	// Single-line mode: check for \G suffix (vertical output)
	forceVertical := false
	if strings.HasSuffix(strings.TrimSpace(input), `\G`) {
		input = strings.TrimSuffix(strings.TrimSpace(input), `\G`)
		forceVertical = true
	}

	return a.executeInput(strings.TrimSpace(input), forceVertical)
}

func (a *App) handleMultiLine(input string) bool {
	// In psql mode: execute when semicolon is at end
	trimmed := strings.TrimSpace(input)

	if a.inMultiLine {
		a.multiLineBuffer.WriteString("\n")
		a.multiLineBuffer.WriteString(input)

		if strings.HasSuffix(trimmed, ";") || strings.HasPrefix(trimmed, `\`) {
			query := a.multiLineBuffer.String()
			a.multiLineBuffer.Reset()
			a.inMultiLine = false
			return a.executeInput(strings.TrimSpace(query), false)
		}
		return false
	}

	// Check if this starts a multi-line query
	if trimmed != "" && !strings.HasSuffix(trimmed, ";") && !strings.HasPrefix(trimmed, `\`) {
		a.multiLineBuffer.WriteString(input)
		a.inMultiLine = true
		return false
	}

	return a.executeInput(trimmed, false)
}

func (a *App) executeInput(input string, forceVertical bool) bool {
	if input == "" {
		return false
	}

	// Check for special commands
	if a.special.IsSpecial(input) {
		results, err := a.special.Execute(context.Background(), a.executor, input)
		if err != nil {
			if err == special.ErrQuit {
				return true
			}
			fmt.Fprintf(a.Stderr, "Error: %s\n", err)
			return false
		}
		a.displayResults(results, forceVertical)
		return false
	}

	// Check destructive warning
	if a.config.IsDestructive(input) {
		fmt.Fprintf(a.Stdout, "You're about to run a destructive command.\nDo you want to proceed? (y/n): ")
		// In a real implementation, we'd read confirmation here
	}

	// Execute SQL query
	a.lastQuery = input
	start := time.Now()
	ctx := context.Background()

	// Split on semicolons for multi-statement
	queries := SplitStatements(input)
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		result, err := a.executor.Execute(ctx, query)
		if err != nil {
			fmt.Fprintf(a.Stderr, "Error: %s\n", err)
			if a.config.OnError == "STOP" {
				break
			}
			continue
		}
		if result != nil {
			a.displayResults([]*format.QueryResult{result}, forceVertical)
		}
	}

	if a.special.Timing {
		elapsed := time.Since(start)
		fmt.Fprintln(a.Stdout, special.FormatTiming(elapsed))
	}

	return false
}

func (a *App) displayResults(results []*format.QueryResult, forceVertical bool) {
	for _, result := range results {
		if result == nil {
			continue
		}

		// Determine output writer (pager or stdout)
		writer := a.getOutputWriter(result)

		if len(result.Columns) > 0 {
			opts := format.DefaultOptions()
			opts.NullValue = a.config.NullString

			// Determine format
			switch a.config.TableFormat {
			case "ascii":
				opts.Style = format.ASCIIStyle
			case "psql":
				opts.Style = format.PsqlStyle
			case "unicode":
				opts.Style = format.UnicodeStyle
			case "csv":
				opts.Format = format.CSVFormat
			case "tsv":
				opts.Format = format.TSVFormat
			case "json":
				opts.Format = format.JSONFormat
			case "vertical":
				opts.Format = format.VerticalFormat
			}

			if forceVertical || a.special.Expanded || a.config.ExpandedOutput {
				opts.Expanded = true
			}

			// Auto-expand: switch to vertical if result is wider than terminal
			if !opts.Expanded && a.config.AutoExpand && len(result.Columns) > 0 {
				tableWidth := 1 // leading border
				for _, col := range result.Columns {
					w := len(col)
					for _, row := range result.Rows {
						for ci, cell := range row {
							if ci < len(result.Columns) && len(cell) > w {
								w = len(cell)
							}
						}
					}
					tableWidth += w + 3 // cell + padding + border
				}
				if termWidth := getTerminalWidth(); termWidth > 0 && tableWidth > termWidth {
					opts.Expanded = true
				}
			}

			format.Format(writer, result, opts)
		}

		if result.StatusText != "" {
			fmt.Fprintln(writer, result.StatusText)
		}

		// Close pager if we opened one
		if closer, ok := writer.(io.Closer); ok && writer != a.Stdout {
			closer.Close()
		}
	}
}

func (a *App) getOutputWriter(result *format.QueryResult) io.Writer {
	if a.outputFile != nil {
		return a.outputFile
	}

	if !a.config.EnablePager || a.special.Pager == "" {
		return a.Stdout
	}

	// Only use pager for results that exceed terminal height
	if len(result.Rows) == 0 {
		return a.Stdout
	}
	termHeight := getTerminalHeight()
	outputLines := len(result.Rows) + 3 // rows + header + separator + status
	if termHeight > 0 && outputLines < termHeight {
		return a.Stdout
	}

	return a.openPager()
}

func (a *App) openPager() io.Writer {
	pagerCmd := a.special.Pager
	if pagerCmd == "" {
		pagerCmd = os.Getenv("PAGER")
	}
	if pagerCmd == "" {
		return a.Stdout
	}

	parts := strings.Fields(pagerCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = a.Stdout
	cmd.Stderr = a.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return a.Stdout
	}

	if err := cmd.Start(); err != nil {
		return a.Stdout
	}

	return &pagerWriter{pipe: pipe, cmd: cmd}
}

type pagerWriter struct {
	pipe io.WriteCloser
	cmd  *exec.Cmd
}

func (pw *pagerWriter) Write(p []byte) (n int, err error) {
	return pw.pipe.Write(p)
}

func (pw *pagerWriter) Close() error {
	pw.pipe.Close()
	return pw.cmd.Wait()
}

// GetPrompt returns the formatted prompt string.
func (a *App) GetPrompt() string {
	user := os.Getenv("USER")
	host := "localhost"
	database := ""
	port := ""

	if a.executor != nil {
		database = a.executor.Database()
	}

	switch a.mode {
	case PostgreSQL:
		if host == "" {
			host = "localhost"
		}
		port = "5432"
	case MySQL:
		port = "3306"
	}

	return config.FormatPrompt(a.config.Prompt, user, host, database, port, false)
}

// GetContinuationPrompt returns the prompt for multi-line continuation.
func (a *App) GetContinuationPrompt() string {
	if a.config.PromptContinuation != "" {
		return a.config.PromptContinuation
	}
	return "-> "
}

// ExecuteNonInteractive runs one or more statements (split by semicolons) and
// prints results using the configured table format. It handles both special
// commands and SQL queries, matching what `-e` should do. Returns true if any
// statement produced an error.
func (a *App) ExecuteNonInteractive(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// Special commands are not split on semicolons
	if a.special.IsSpecial(input) {
		results, err := a.special.Execute(context.Background(), a.executor, input)
		if err != nil {
			fmt.Fprintf(a.Stderr, "Error: %s\n", err)
			return true
		}
		a.displayResults(results, false)
		return false
	}

	hasError := false
	queries := SplitStatements(input)
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		// Check if this individual statement is a special command
		if a.special.IsSpecial(query) {
			results, err := a.special.Execute(context.Background(), a.executor, query)
			if err != nil {
				fmt.Fprintf(a.Stderr, "Error: %s\n", err)
				hasError = true
				continue
			}
			a.displayResults(results, false)
			continue
		}

		result, err := a.executor.Execute(context.Background(), query)
		if err != nil {
			fmt.Fprintf(a.Stderr, "Error: %s\n", err)
			hasError = true
			continue
		}
		if result != nil {
			a.displayResults([]*format.QueryResult{result}, false)
		}
	}
	return hasError
}

// HighlightInput applies syntax highlighting to input text.
func (a *App) HighlightInput(input string) string {
	style := highlight.GetStyle(a.config.SyntaxStyle)
	return highlight.Highlight(input, style)
}

// Run starts the interactive REPL loop.
func (a *App) Run() error {
	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Print welcome message
	if !a.config.LessChatty {
		version, _ := a.executor.ServerVersion()
		fmt.Fprintf(a.Stdout, "gocli %s\n", a.mode)
		if version != "" {
			fmt.Fprintf(a.Stdout, "Server: %s\n", version)
		}
		fmt.Fprintln(a.Stdout, "Type \\? for help.")
		fmt.Fprintln(a.Stdout)
	}

	// Initial completion refresh
	go a.RefreshCompletions()

	// Main REPL loop - uses readline when available
	// For now, use simple line reading (readline integration in cmd/)
	return nil
}

// SplitStatements splits SQL input on semicolons, respecting strings and comments.
func SplitStatements(input string) []string {
	var statements []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	i := 0

	for i < len(input) {
		ch := input[i]

		if inLineComment {
			current.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			i++
			continue
		}

		if inBlockComment {
			current.WriteByte(ch)
			if ch == '*' && i+1 < len(input) && input[i+1] == '/' {
				current.WriteByte('/')
				inBlockComment = false
				i += 2
				continue
			}
			i++
			continue
		}

		if inSingleQuote {
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(input) && input[i+1] == '\'' {
					current.WriteByte('\'')
					i += 2
					continue
				}
				inSingleQuote = false
			}
			i++
			continue
		}

		if inDoubleQuote {
			current.WriteByte(ch)
			if ch == '"' {
				inDoubleQuote = false
			}
			i++
			continue
		}

		switch {
		case ch == '-' && i+1 < len(input) && input[i+1] == '-':
			current.WriteByte(ch)
			inLineComment = true
			i++
		case ch == '/' && i+1 < len(input) && input[i+1] == '*':
			current.WriteByte(ch)
			current.WriteByte('*')
			inBlockComment = true
			i += 2
		case ch == '\'':
			current.WriteByte(ch)
			inSingleQuote = true
			i++
		case ch == '"':
			current.WriteByte(ch)
			inDoubleQuote = true
			i++
		case ch == ';':
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			i++
		default:
			current.WriteByte(ch)
			i++
		}
	}

	// Don't forget the last statement (without trailing semicolon)
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTerminalWidth() int {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if err != 0 || ws.Col == 0 {
		return 0
	}
	return int(ws.Col)
}

func getTerminalHeight() int {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if err != 0 || ws.Row == 0 {
		return 0
	}
	return int(ws.Row)
}
