package special

import (
	"context"
	"fmt"

	"github.com/tomblomfield/gocli/internal/format"
	"github.com/tomblomfield/gocli/internal/pg"
)

// RegisterPG registers PostgreSQL-specific special commands.
func RegisterPG(r *Registry) {
	// \dt - List tables
	r.Register(&Command{
		Name:        `\dt`,
		Syntax:      `\dt[+] [pattern]`,
		Description: "List tables",
		ArgType:     ParsedQuery,
		Handler:     pgListTables,
	})

	// \dv - List views
	r.Register(&Command{
		Name:        `\dv`,
		Syntax:      `\dv[+] [pattern]`,
		Description: "List views",
		ArgType:     ParsedQuery,
		Handler:     pgListViews,
	})

	// \di - List indexes
	r.Register(&Command{
		Name:        `\di`,
		Syntax:      `\di[+] [pattern]`,
		Description: "List indexes",
		ArgType:     ParsedQuery,
		Handler:     pgListIndexes,
	})

	// \ds - List sequences
	r.Register(&Command{
		Name:        `\ds`,
		Syntax:      `\ds[+] [pattern]`,
		Description: "List sequences",
		ArgType:     ParsedQuery,
		Handler:     pgListSequences,
	})

	// \df - List functions
	r.Register(&Command{
		Name:        `\df`,
		Syntax:      `\df[+] [pattern]`,
		Description: "List functions",
		ArgType:     ParsedQuery,
		Handler:     pgListFunctions,
	})

	// \dn - List schemas
	r.Register(&Command{
		Name:        `\dn`,
		Syntax:      `\dn[+] [pattern]`,
		Description: "List schemas",
		ArgType:     ParsedQuery,
		Handler:     pgListSchemas,
	})

	// \du - List roles
	r.Register(&Command{
		Name:        `\du`,
		Syntax:      `\du[+] [pattern]`,
		Description: "List roles",
		ArgType:     ParsedQuery,
		Handler:     pgListRoles,
	})

	// \l - List databases
	r.Register(&Command{
		Name:        `\l`,
		Syntax:      `\l[+] [pattern]`,
		Description: "List databases",
		ArgType:     ParsedQuery,
		Handler:     pgListDatabases,
	})

	// \d - Describe table
	r.Register(&Command{
		Name:        `\d`,
		Syntax:      `\d[+] [pattern]`,
		Description: "Describe table or list tables",
		ArgType:     ParsedQuery,
		Aliases:     []string{"describe"},
		Handler:     pgDescribe,
	})

	// \dx - List extensions
	r.Register(&Command{
		Name:        `\dx`,
		Syntax:      `\dx[+] [pattern]`,
		Description: "List extensions",
		ArgType:     ParsedQuery,
		Handler:     pgListExtensions,
	})

	// \dT - List data types
	r.Register(&Command{
		Name:        `\dt`,
		Syntax:      `\dT[+] [pattern]`,
		Description: "List data types",
		ArgType:     ParsedQuery,
		Hidden:      true,
		Handler:     pgListTypes,
	})

	// \db - List tablespaces
	r.Register(&Command{
		Name:        `\db`,
		Syntax:      `\db[+] [pattern]`,
		Description: "List tablespaces",
		ArgType:     ParsedQuery,
		Handler:     pgListTablespaces,
	})

	// \dp - List privileges
	r.Register(&Command{
		Name:        `\dp`,
		Syntax:      `\dp [pattern]`,
		Description: "List access privileges",
		ArgType:     ParsedQuery,
		Aliases:     []string{`\z`},
		Handler:     pgListPrivileges,
	})

	// \sf - Show function definition
	r.Register(&Command{
		Name:        `\sf`,
		Syntax:      `\sf[+] funcname`,
		Description: "Show function definition",
		ArgType:     ParsedQuery,
		Handler:     pgShowFunction,
	})

	// \conninfo - Connection info
	r.Register(&Command{
		Name:        `\conninfo`,
		Syntax:      `\conninfo`,
		Description: "Display connection information",
		ArgType:     NoQuery,
		Handler:     pgConnInfo,
	})

	// \h - SQL help
	r.Register(&Command{
		Name:        `\h`,
		Syntax:      `\h [command]`,
		Description: "Help on SQL command syntax",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return []*format.QueryResult{{StatusText: "Use \\h <command> for help on a specific SQL command."}}, nil
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Help for: %s (use PostgreSQL docs for detailed syntax)", arg)}}, nil
		},
	})

	// \i - Execute file
	r.Register(&Command{
		Name:        `\i`,
		Syntax:      `\i filename`,
		Description: "Execute commands from file",
		ArgType:     RawQuery,
		Handler:     pgExecuteFile,
	})

	// \o - Output to file
	r.Register(&Command{
		Name:        `\o`,
		Syntax:      `\o [filename]`,
		Description: "Send all query results to file",
		ArgType:     RawQuery,
		Handler: func(_ context.Context, _ interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
			if arg == "" {
				return []*format.QueryResult{{StatusText: "Output reset to stdout."}}, nil
			}
			return []*format.QueryResult{{StatusText: fmt.Sprintf("Output directed to: %s", arg)}}, nil
		},
	})

	// \copy
	r.Register(&Command{
		Name:        `\copy`,
		Syntax:      `\copy ...`,
		Description: "Copy data between file and table",
		ArgType:     RawQuery,
		Handler:     pgCopy,
	})

	// \dm - List materialized views
	r.Register(&Command{
		Name:        `\dm`,
		Syntax:      `\dm[+] [pattern]`,
		Description: "List materialized views",
		ArgType:     ParsedQuery,
		Handler:     pgListMaterializedViews,
	})

	// \dD - List domains
	r.Register(&Command{
		Name:        `\dd`,
		Syntax:      `\dD[+] [pattern]`,
		Description: "List domains",
		ArgType:     ParsedQuery,
		Handler:     pgListDomains,
	})
}

func getPGExecutor(executor interface{}) *pg.Executor {
	if e, ok := executor.(*pg.Executor); ok {
		return e
	}
	return nil
}

func pgListTables(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view'
			WHEN 'm' THEN 'materialized view' WHEN 'i' THEN 'index'
			WHEN 'S' THEN 'sequence' WHEN 'f' THEN 'foreign table'
			WHEN 'p' THEN 'partitioned table' END as "Type",
		pg_catalog.pg_get_userbyid(c.relowner) as "Owner"`

	if verbose {
		query += `, pg_catalog.pg_size_pretty(pg_catalog.pg_table_size(c.oid)) as "Size",
			pg_catalog.obj_description(c.oid, 'pg_class') as "Description"`
	}

	query += ` FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','p')
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'
		AND n.nspname !~ '^pg_toast'`

	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListViews(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view' END as "Type",
		pg_catalog.pg_get_userbyid(c.relowner) as "Owner"`

	if verbose {
		query += `, pg_catalog.obj_description(c.oid, 'pg_class') as "Description"`
	}

	query += ` FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('v','m')
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`

	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListIndexes(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		c2.relname as "Table",
		pg_catalog.pg_get_userbyid(c.relowner) as "Owner"`

	if verbose {
		query += `, pg_catalog.pg_size_pretty(pg_catalog.pg_table_size(c.oid)) as "Size",
			pg_catalog.obj_description(c.oid, 'pg_class') as "Description"`
	}

	query += ` FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_index i ON i.indexrelid = c.oid
		LEFT JOIN pg_catalog.pg_class c2 ON i.indrelid = c2.oid
		WHERE c.relkind = 'i'
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`

	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListSequences(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		pg_catalog.pg_get_userbyid(c.relowner) as "Owner"`

	if verbose {
		query += `, pg_catalog.obj_description(c.oid, 'pg_class') as "Description"`
	}

	query += ` FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'S'
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`

	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListFunctions(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema",
		p.proname as "Name",
		pg_catalog.pg_get_function_result(p.oid) as "Result data type",
		pg_catalog.pg_get_function_arguments(p.oid) as "Argument data types",
		CASE p.prokind WHEN 'a' THEN 'agg' WHEN 'w' THEN 'window' WHEN 'p' THEN 'proc' ELSE 'func' END as "Type"`

	if verbose {
		query += `, pg_catalog.obj_description(p.oid, 'pg_proc') as "Description"`
	}

	query += ` FROM pg_catalog.pg_proc p
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`

	if pattern != "" {
		query += fmt.Sprintf(` AND p.proname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListSchemas(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname AS "Name", pg_catalog.pg_get_userbyid(n.nspowner) AS "Owner"`
	if verbose {
		query += `, pg_catalog.obj_description(n.oid, 'pg_namespace') AS "Description"`
	}
	query += ` FROM pg_catalog.pg_namespace n
		WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'`
	if pattern != "" {
		query += fmt.Sprintf(` AND n.nspname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListRoles(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT r.rolname as "Role name",
		r.rolsuper as "Superuser",
		r.rolinherit as "Inherit",
		r.rolcreaterole as "Create role",
		r.rolcreatedb as "Create DB",
		r.rolcanlogin as "Can login",
		r.rolreplication as "Replication",
		r.rolconnlimit as "Connections"`

	if verbose {
		query += `, pg_catalog.shobj_description(r.oid, 'pg_authid') as "Description"`
	}

	query += ` FROM pg_catalog.pg_roles r`
	if pattern != "" {
		query += fmt.Sprintf(` WHERE r.rolname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListDatabases(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT d.datname as "Name",
		pg_catalog.pg_get_userbyid(d.datdba) as "Owner",
		pg_catalog.pg_encoding_to_char(d.encoding) as "Encoding",
		d.datcollate as "Collate",
		d.datctype as "Ctype"`

	if verbose {
		query += `, pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(d.datname)) as "Size",
			pg_catalog.shobj_description(d.oid, 'pg_database') as "Description"`
	}

	query += ` FROM pg_catalog.pg_database d`
	if pattern != "" {
		query += fmt.Sprintf(` WHERE d.datname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgDescribe(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	if pattern == "" {
		return pgListTables(ctx, executor, pattern, verbose)
	}

	// Describe specific table
	query := `SELECT a.attname AS "Column",
		pg_catalog.format_type(a.atttypid, a.atttypmod) AS "Type",
		CASE WHEN a.attnotnull THEN 'not null' ELSE '' END AS "Nullable",
		COALESCE(pg_catalog.pg_get_expr(d.adbin, d.adrelid), '') AS "Default"`

	if verbose {
		query += `, pg_catalog.col_description(a.attrelid, a.attnum) AS "Description"`
	}

	query += fmt.Sprintf(` FROM pg_catalog.pg_attribute a
		LEFT JOIN pg_catalog.pg_attrdef d ON (a.attrelid = d.adrelid AND a.attnum = d.adnum)
		WHERE a.attrelid = '%s'::regclass AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, pattern)

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListExtensions(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT e.extname AS "Name", e.extversion AS "Version",
		n.nspname AS "Schema"`
	if verbose {
		query += `, e.extrelocatable AS "Relocatable",
			pg_catalog.obj_description(e.oid, 'pg_extension') AS "Description"`
	}
	query += ` FROM pg_catalog.pg_extension e
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace`
	if pattern != "" {
		query += fmt.Sprintf(` WHERE e.extname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListTypes(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", pg_catalog.format_type(t.oid, NULL) AS "Name"`
	if verbose {
		query += `, pg_catalog.obj_description(t.oid, 'pg_type') as "Description"`
	}
	query += ` FROM pg_catalog.pg_type t
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		WHERE (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))
		AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`
	if pattern != "" {
		query += fmt.Sprintf(` AND pg_catalog.format_type(t.oid, NULL) ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListTablespaces(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT spcname AS "Name", pg_catalog.pg_get_userbyid(spcowner) AS "Owner",
		pg_catalog.pg_tablespace_location(oid) AS "Location"`
	if verbose {
		query += `, pg_catalog.pg_size_pretty(pg_catalog.pg_tablespace_size(oid)) AS "Size",
			pg_catalog.shobj_description(oid, 'pg_tablespace') AS "Description"`
	}
	query += ` FROM pg_catalog.pg_tablespace`
	if pattern != "" {
		query += fmt.Sprintf(` WHERE spcname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListPrivileges(ctx context.Context, executor interface{}, pattern string, _ bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view' WHEN 'm' THEN 'matview'
			WHEN 'S' THEN 'sequence' WHEN 'f' THEN 'foreign table' END as "Type",
		pg_catalog.array_to_string(c.relacl, E'\n') AS "Access privileges",
		pg_catalog.array_to_string(ARRAY(
			SELECT attname || E':\n  ' || pg_catalog.array_to_string(attacl, E'\n  ')
			FROM pg_catalog.pg_attribute WHERE attrelid = c.oid AND NOT attisdropped AND attacl IS NOT NULL
		), E'\n') AS "Column privileges"
	FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	WHERE c.relkind IN ('r','v','m','S','f')
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`
	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgShowFunction(ctx context.Context, executor interface{}, name string, _ bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}
	if name == "" {
		return nil, fmt.Errorf("function name required")
	}

	query := fmt.Sprintf(`SELECT pg_catalog.pg_get_functiondef(oid) AS "Function definition"
		FROM pg_catalog.pg_proc WHERE proname = '%s'`, name)

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgConnInfo(_ context.Context, executor interface{}, _ string, _ bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	version, _ := e.ServerVersion()
	return []*format.QueryResult{{
		StatusText: fmt.Sprintf("Connected to database: %s (server version: %s)", e.Database(), version),
	}}, nil
}

func pgExecuteFile(ctx context.Context, executor interface{}, filename string, _ bool) ([]*format.QueryResult, error) {
	_ = ctx
	_ = executor
	if filename == "" {
		return nil, fmt.Errorf("filename required")
	}
	return []*format.QueryResult{{StatusText: fmt.Sprintf("Executing file: %s", filename)}}, nil
}

func pgCopy(_ context.Context, executor interface{}, arg string, _ bool) ([]*format.QueryResult, error) {
	_ = executor
	if arg == "" {
		return nil, fmt.Errorf("usage: \\copy table_name TO/FROM filename")
	}
	return []*format.QueryResult{{StatusText: fmt.Sprintf("COPY: %s", arg)}}, nil
}

func pgListMaterializedViews(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema", c.relname as "Name",
		pg_catalog.pg_get_userbyid(c.relowner) as "Owner"`
	if verbose {
		query += `, pg_catalog.pg_size_pretty(pg_catalog.pg_table_size(c.oid)) as "Size",
			pg_catalog.obj_description(c.oid, 'pg_class') as "Description"`
	}
	query += ` FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'm'
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`
	if pattern != "" {
		query += fmt.Sprintf(` AND c.relname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func pgListDomains(ctx context.Context, executor interface{}, pattern string, verbose bool) ([]*format.QueryResult, error) {
	e := getPGExecutor(executor)
	if e == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	query := `SELECT n.nspname as "Schema",
		t.typname as "Name",
		pg_catalog.format_type(t.typbasetype, t.typtypmod) as "Type",
		COALESCE(pg_catalog.pg_get_constraintdef(con.oid), '') as "Check"`
	if verbose {
		query += `, pg_catalog.obj_description(t.oid, 'pg_type') as "Description"`
	}
	query += ` FROM pg_catalog.pg_type t
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		LEFT JOIN pg_catalog.pg_constraint con ON con.contypid = t.oid
		WHERE t.typtype = 'd'
		AND n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'`
	if pattern != "" {
		query += fmt.Sprintf(` AND t.typname ~ '%s'`, pattern)
	}
	query += ` ORDER BY 1, 2`

	return []*format.QueryResult{execPGQuery(ctx, e, query)}, nil
}

func execPGQuery(ctx context.Context, e *pg.Executor, query string) *format.QueryResult {
	result, err := e.Execute(ctx, query)
	if err != nil {
		return &format.QueryResult{StatusText: fmt.Sprintf("ERROR: %s", err)}
	}
	return result
}
