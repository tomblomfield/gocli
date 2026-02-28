// Package special implements backslash/special commands for both PostgreSQL and MySQL.
package special

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tomblomfield/gocli/internal/format"
)

// ArgType specifies how command arguments are parsed.
type ArgType int

const (
	NoQuery     ArgType = iota // No SQL query generated
	ParsedQuery                // Arguments are parsed/split
	RawQuery                   // Arguments passed as raw string
)

// Command represents a registered special command.
type Command struct {
	Name          string
	Syntax        string
	Description   string
	ArgType       ArgType
	Hidden        bool
	CaseSensitive bool
	Aliases       []string
	Handler       CommandHandler
}

// CommandHandler is the function signature for special command handlers.
type CommandHandler func(ctx context.Context, executor interface{}, arg string, verbose bool) ([]*format.QueryResult, error)

// Registry holds registered special commands.
type Registry struct {
	commands map[string]*Command
	// Shared state
	Expanded  bool
	Timing    bool
	Pager     string
	Editor    string
	WatchSecs int
	Favorites map[string]string
}

// NewRegistry creates a new command registry with common commands.
func NewRegistry() *Registry {
	r := &Registry{
		commands:   make(map[string]*Command),
		Timing:     true,
		Pager:      os.Getenv("PAGER"),
		Editor:     getEditor(),
		WatchSecs:  2,
		Favorites:  make(map[string]string),
	}
	r.registerCommon()
	return r
}

func getEditor() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
	}
}

// IsSpecial returns true if the input starts with a special command.
func (r *Registry) IsSpecial(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}
	cmd := extractCommand(input)
	_, ok := r.commands[cmd]
	return ok
}

// Execute runs a special command.
func (r *Registry) Execute(ctx context.Context, executor interface{}, input string) ([]*format.QueryResult, error) {
	input = strings.TrimSpace(input)
	cmd, arg, verbose := parseSpecialCommand(input)

	handler, ok := r.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	return handler.Handler(ctx, executor, arg, verbose)
}

// Commands returns all registered commands (non-hidden).
func (r *Registry) Commands() []*Command {
	seen := make(map[*Command]bool)
	var cmds []*Command
	for _, cmd := range r.commands {
		if !seen[cmd] && !cmd.Hidden {
			seen[cmd] = true
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func extractCommand(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}
	cmd := strings.ToLower(parts[0])
	// Handle \d+ style (strip trailing +)
	if strings.HasPrefix(cmd, `\`) && strings.HasSuffix(cmd, "+") && len(cmd) > 2 {
		cmd = cmd[:len(cmd)-1]
	}
	return cmd
}

func parseSpecialCommand(input string) (cmd, arg string, verbose bool) {
	parts := strings.SplitN(input, " ", 2)
	cmd = parts[0]
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	// Check for verbose flag (+)
	if strings.HasSuffix(cmd, "+") && len(cmd) > 1 {
		cmd = cmd[:len(cmd)-1]
		verbose = true
	}
	cmd = strings.ToLower(cmd)
	return
}

func (r *Registry) registerCommon() {
	// \? - Help
	r.Register(&Command{
		Name:        `\?`,
		Syntax:      `\?`,
		Description: "Show help for special commands",
		ArgType:     NoQuery,
		Handler:     r.helpHandler,
	})

	// \q - Quit
	r.Register(&Command{
		Name:        `\q`,
		Syntax:      `\q`,
		Description: "Quit",
		ArgType:     NoQuery,
		Aliases:     []string{"quit", "exit"},
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return nil, ErrQuit
		},
	})

	// \x - Expanded output
	r.Register(&Command{
		Name:        `\x`,
		Syntax:      `\x`,
		Description: "Toggle expanded output",
		ArgType:     NoQuery,
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			r.Expanded = !r.Expanded
			state := "off"
			if r.Expanded {
				state = "on"
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Expanded display is %s.", state)}}, nil
		},
	})

	// \timing - Toggle timing
	r.Register(&Command{
		Name:        `\timing`,
		Syntax:      `\timing`,
		Description: "Toggle timing of commands",
		ArgType:     NoQuery,
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			r.Timing = !r.Timing
			state := "off"
			if r.Timing {
				state = "on"
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Timing is %s.", state)}}, nil
		},
	})

	// \pager - Set pager
	r.Register(&Command{
		Name:        `\pager`,
		Syntax:      `\pager [command]`,
		Description: "Set PAGER. Print the query results via PAGER",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				arg = os.Getenv("PAGER")
			}
			r.Pager = arg
			if arg == "" {
				return []*format.QueryResult{{StatusText: "Pager reset to system default."}}, nil
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("PAGER set to %s.", arg)}}, nil
		},
	})

	// \e - Edit in external editor
	r.Register(&Command{
		Name:        `\e`,
		Syntax:      `\e [filename]`,
		Description: "Edit query with external editor",
		ArgType:     RawQuery,
		Handler:     r.editHandler,
		Aliases:     []string{`\edit`},
	})

	// \! - Shell command
	r.Register(&Command{
		Name:        `\!`,
		Syntax:      `\! [command]`,
		Description: "Execute a shell command",
		ArgType:     RawQuery,
		Handler:     r.shellHandler,
	})

	// \f - List/execute favorites
	r.Register(&Command{
		Name:        `\f`,
		Syntax:      `\f [name]`,
		Description: "List or execute favorite queries",
		ArgType:     RawQuery,
		Handler:     r.favoritesHandler,
	})

	// \fs - Save favorite
	r.Register(&Command{
		Name:        `\fs`,
		Syntax:      `\fs name query`,
		Description: "Save a favorite query",
		ArgType:     RawQuery,
		Handler:     r.saveFavoriteHandler,
	})

	// \fd - Delete favorite
	r.Register(&Command{
		Name:        `\fd`,
		Syntax:      `\fd name`,
		Description: "Delete a favorite query",
		ArgType:     RawQuery,
		Handler:     r.deleteFavoriteHandler,
	})

	// \watch - Repeat query
	r.Register(&Command{
		Name:        `\watch`,
		Syntax:      `\watch [seconds]`,
		Description: "Execute query repeatedly",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg != "" {
				var secs int
				if _, err := fmt.Sscanf(arg, "%d", &secs); err == nil && secs > 0 {
					r.WatchSecs = secs
				}
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Watch every %ds.", r.WatchSecs)}}, nil
		},
	})

	// \# - Refresh completions
	r.Register(&Command{
		Name:        `\#`,
		Syntax:      `\#`,
		Description: "Refresh auto-completions",
		ArgType:     NoQuery,
		Aliases:     []string{`\refresh`, "rehash"},
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: "Auto-completions refreshed."}}, nil
		},
	})
}

func (r *Registry) helpHandler(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
	var rows [][]string
	for _, cmd := range r.Commands() {
		rows = append(rows, []string{cmd.Syntax, cmd.Description})
	}
	return []*format.QueryResult{{
		Columns: []string{"Command", "Description"},
		Rows:    rows,
	}}, nil
}

func (r *Registry) editHandler(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	filename := arg
	if filename == "" {
		// Create temp file
		f, err := os.CreateTemp("", "gocli-*.sql")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		filename = f.Name()
		f.Close()
		defer os.Remove(filename)
	}

	cmd := exec.Command(r.Editor, filename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor failed: %w", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	query := strings.TrimSpace(string(data))
	if query == "" {
		return nil, nil
	}

	return []*format.QueryResult{{StatusText: fmt.Sprintf("Query from editor: %s", query)}}, nil
}

func (r *Registry) shellHandler(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	if arg == "" {
		return nil, fmt.Errorf("no command specified")
	}
	cmd := exec.Command("sh", "-c", arg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}
	return nil, nil
}

func (r *Registry) favoritesHandler(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	if arg == "" {
		// List all favorites
		var rows [][]string
		for name, query := range r.Favorites {
			rows = append(rows, []string{name, query})
		}
		if len(rows) == 0 {
			return []*format.QueryResult{{StatusText: "No favorites saved. Use \\fs to save a query."}}, nil
		}
		return []*format.QueryResult{{
			Columns: []string{"Name", "Query"},
			Rows:    rows,
		}}, nil
	}

	// Execute named favorite
	parts := strings.Fields(arg)
	name := parts[0]
	query, ok := r.Favorites[name]
	if !ok {
		return nil, fmt.Errorf("favorite '%s' not found", name)
	}

	// Parameter substitution
	if len(parts) > 1 {
		for i, param := range parts[1:] {
			query = strings.ReplaceAll(query, fmt.Sprintf("$%d", i+1), param)
		}
	}

	return []*format.QueryResult{{StatusText: fmt.Sprintf("Executing: %s", query)}}, nil
}

func (r *Registry) saveFavoriteHandler(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	parts := strings.SplitN(arg, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("usage: \\fs name query")
	}
	name := parts[0]
	query := parts[1]
	r.Favorites[name] = query
	return []*format.QueryResult{{StatusText: fmt.Sprintf("Saved favorite: %s", name)}}, nil
}

func (r *Registry) deleteFavoriteHandler(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	name := strings.TrimSpace(arg)
	if name == "" {
		return nil, fmt.Errorf("usage: \\fd name")
	}
	if _, ok := r.Favorites[name]; !ok {
		return nil, fmt.Errorf("favorite '%s' not found", name)
	}
	delete(r.Favorites, name)
	return []*format.QueryResult{{StatusText: fmt.Sprintf("Deleted favorite: %s", name)}}, nil
}

// ErrQuit is returned when the user wants to quit.
var ErrQuit = fmt.Errorf("quit")

// FormatTiming returns a human-readable timing string.
func FormatTiming(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("Time: %.3fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("Time: %.3fs", d.Seconds())
}
