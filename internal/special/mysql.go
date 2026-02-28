package special

import (
	"context"
	"fmt"

	"github.com/tomblomfield/gocli/internal/format"
	"github.com/tomblomfield/gocli/internal/mysql"
)

// RegisterMySQL registers MySQL-specific special commands.
func RegisterMySQL(r *Registry) {
	// \dt - List tables
	r.Register(&Command{
		Name:        `\dt`,
		Syntax:      `\dt[+] [table]`,
		Description: "List or describe tables",
		ArgType:     ParsedQuery,
		Handler:     mysqlListTables,
	})

	// \l - List databases
	r.Register(&Command{
		Name:        `\l`,
		Syntax:      `\l`,
		Description: "List databases",
		ArgType:     NoQuery,
		Handler:     mysqlListDatabases,
	})

	// status / \s - Server status
	r.Register(&Command{
		Name:        `\s`,
		Syntax:      `\s`,
		Description: "Get server status",
		ArgType:     NoQuery,
		Aliases:     []string{"status"},
		Handler:     mysqlStatus,
	})

	// use / \u - Switch database
	r.Register(&Command{
		Name:        `\u`,
		Syntax:      `\u database`,
		Description: "Switch default database",
		ArgType:     RawQuery,
		Aliases:     []string{"use"},
		Handler:     mysqlUse,
	})

	// connect / \r - Reconnect
	r.Register(&Command{
		Name:        `\r`,
		Syntax:      `\r [database]`,
		Description: "Reconnect to the server",
		ArgType:     RawQuery,
		Aliases:     []string{"connect"},
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Reconnecting to: %s", arg)}}, nil
		},
	})

	// source / \. - Execute file
	r.Register(&Command{
		Name:        `\.`,
		Syntax:      `\. filename`,
		Description: "Execute SQL from file",
		ArgType:     RawQuery,
		Aliases:     []string{"source"},
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return nil, fmt.Errorf("filename required")
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Executing: %s", arg)}}, nil
		},
	})

	// system - Shell command
	r.Register(&Command{
		Name:        "system",
		Syntax:      "system command",
		Description: "Execute a system shell command",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return nil, fmt.Errorf("no command specified")
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Running: %s", arg)}}, nil
		},
	})

	// delimiter
	r.Register(&Command{
		Name:        "delimiter",
		Syntax:      "delimiter string",
		Description: "Change query delimiter",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return nil, fmt.Errorf("delimiter string required")
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Delimiter set to: %s", arg)}}, nil
		},
	})

	// \G - Vertical output
	r.Register(&Command{
		Name:        `\g`,
		Syntax:      `\G`,
		Description: "Display results vertically",
		ArgType:     NoQuery,
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: "Next result will be displayed vertically."}}, nil
		},
	})

	// warnings / nowarnings
	r.Register(&Command{
		Name:        `\w`,
		Syntax:      `\W`,
		Description: "Show warnings after every statement",
		ArgType:     NoQuery,
		Aliases:     []string{"warnings"},
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: "Show warnings enabled."}}, nil
		},
	})

	r.Register(&Command{
		Name:        "nowarnings",
		Syntax:      "nowarnings",
		Description: "Don't show warnings after every statement",
		ArgType:     NoQuery,
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: "Show warnings disabled."}}, nil
		},
	})

	// tee / notee
	r.Register(&Command{
		Name:        "tee",
		Syntax:      "tee [-o] filename",
		Description: "Append all results to given file",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return nil, fmt.Errorf("filename required")
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Logging to: %s", arg)}}, nil
		},
	})

	r.Register(&Command{
		Name:        "notee",
		Syntax:      "notee",
		Description: "Stop writing results to file",
		ArgType:     NoQuery,
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: "Logging stopped."}}, nil
		},
	})

	// nopager
	r.Register(&Command{
		Name:        "nopager",
		Syntax:      "nopager",
		Description: "Disable pager, print to stdout",
		ArgType:     NoQuery,
		Aliases:     []string{`\n`},
		Handler: func(_ context.Context, _ interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
			r.Pager = ""
			return []*format.QueryResult{{StatusText: "Pager disabled."}}, nil
		},
	})

	// prompt / \R
	r.Register(&Command{
		Name:        `\r`,
		Syntax:      `\R [format]`,
		Description: "Change prompt format",
		ArgType:     RawQuery,
		Aliases:     []string{"prompt"},
		Hidden:      true,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Prompt set to: %s", arg)}}, nil
		},
	})
}

func getMySQLExecutor(executor interface{}) *mysql.Executor {
	if e, ok := executor.(*mysql.Executor); ok {
		return e
	}
	return nil
}

func mysqlListTables(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getMySQLExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to MySQL")
	}

	if pattern != "" {
		// Describe specific table
		query := fmt.Sprintf("DESCRIBE `%s`", pattern)
		result, err := e.Execute(ctx, query)
		if err != nil {
			return nil, err
		}
		return []*format.QueryResult{result}, nil
	}

	query := "SHOW TABLES"
	if verbose {
		query = "SHOW TABLE STATUS"
	}
	result, err := e.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	return []*format.QueryResult{result}, nil
}

func mysqlListDatabases(ctx context.Context, executor interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
	e := getMySQLExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to MySQL")
	}

	result, err := e.Execute(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	return []*format.QueryResult{result}, nil
}

func mysqlStatus(ctx context.Context, executor interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
	e := getMySQLExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to MySQL")
	}

	version, _ := e.ServerVersion()
	var rows [][]string
	rows = append(rows, []string{"Server version", version})
	rows = append(rows, []string{"Current database", e.Database()})

	// Get additional status info
	statusResult, err := e.Execute(ctx, "SHOW STATUS LIKE 'Uptime'")
	if err == nil && len(statusResult.Rows) > 0 {
		rows = append(rows, []string{"Uptime", statusResult.Rows[0][1] + " seconds"})
	}

	charsetResult, err := e.Execute(ctx, "SELECT @@character_set_client, @@character_set_connection, @@character_set_database, @@character_set_results")
	if err == nil && len(charsetResult.Rows) > 0 {
		rows = append(rows, []string{"Server characterset", charsetResult.Rows[0][2]})
		rows = append(rows, []string{"Client characterset", charsetResult.Rows[0][0]})
		rows = append(rows, []string{"Conn. characterset", charsetResult.Rows[0][1]})
	}

	connResult, err := e.Execute(ctx, "SELECT CONNECTION_ID()")
	if err == nil && len(connResult.Rows) > 0 {
		rows = append(rows, []string{"Connection id", connResult.Rows[0][0]})
	}

	return []*format.QueryResult{{
		Columns: []string{"Variable", "Value"},
		Rows:    rows,
	}}, nil
}

func mysqlUse(ctx context.Context, executor interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	e := getMySQLExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to MySQL")
	}
	if arg == "" {
		return nil, fmt.Errorf("database name required")
	}

	query := fmt.Sprintf("USE `%s`", arg)
	_, err := e.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	return []*format.QueryResult{{StatusText: fmt.Sprintf("Database changed to: %s", arg)}}, nil
}
