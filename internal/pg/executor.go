// Package pg implements PostgreSQL-specific query execution and metadata.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/tomblomfield/gocli/internal/format"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// ConnectionConfig holds PostgreSQL connection parameters.
type ConnectionConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	Options  map[string]string
}

// DefaultConfig returns default connection parameters.
func DefaultConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:    "localhost",
		Port:    5432,
		User:    os.Getenv("USER"),
		SSLMode: "prefer",
	}
}

// DSN returns the connection string.
func (c ConnectionConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Database,
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := u.Query()
	if c.SSLMode != "" {
		q.Set("sslmode", c.SSLMode)
	}
	for k, v := range c.Options {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ParseDSN parses a PostgreSQL URI or key=value connection string.
func ParseDSN(dsn string) (ConnectionConfig, error) {
	cfg := DefaultConfig()

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return cfg, fmt.Errorf("invalid connection URI: %w", err)
		}
		if u.Hostname() != "" {
			cfg.Host = u.Hostname()
		}
		if u.Port() != "" {
			p, err := strconv.Atoi(u.Port())
			if err == nil {
				cfg.Port = p
			}
		}
		if u.User != nil {
			cfg.User = u.User.Username()
			if pw, ok := u.User.Password(); ok {
				cfg.Password = pw
			}
		} else {
			cfg.User = ""
		}
		if u.Path != "" && u.Path != "/" {
			cfg.Database = strings.TrimPrefix(u.Path, "/")
		}
		q := u.Query()
		if sm := q.Get("sslmode"); sm != "" {
			cfg.SSLMode = sm
		}
		return cfg, nil
	}

	// Parse key=value format
	parts := strings.Fields(dsn)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch strings.ToLower(k) {
		case "host":
			cfg.Host = v
		case "port":
			p, err := strconv.Atoi(v)
			if err == nil {
				cfg.Port = p
			}
		case "user":
			cfg.User = v
		case "password":
			cfg.Password = v
		case "dbname":
			cfg.Database = v
		case "sslmode":
			cfg.SSLMode = v
		}
	}

	return cfg, nil
}

// ParsePgpass reads the .pgpass file and returns matching password.
func ParsePgpass(host string, port int, database, user string) string {
	var pgpassPath string
	if p := os.Getenv("PGPASSFILE"); p != "" {
		pgpassPath = p
	} else if runtime.GOOS == "windows" {
		pgpassPath = filepath.Join(os.Getenv("APPDATA"), "postgresql", "pgpass.conf")
	} else {
		home, _ := os.UserHomeDir()
		pgpassPath = filepath.Join(home, ".pgpass")
	}

	data, err := os.ReadFile(pgpassPath)
	if err != nil {
		return ""
	}

	portStr := strconv.Itoa(port)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := parsePgpassLine(line)
		if len(fields) != 5 {
			continue
		}
		if matchPgpassField(fields[0], host) &&
			matchPgpassField(fields[1], portStr) &&
			matchPgpassField(fields[2], database) &&
			matchPgpassField(fields[3], user) {
			return fields[4]
		}
	}
	return ""
}

func parsePgpassLine(line string) []string {
	var fields []string
	var current strings.Builder
	escaped := false
	for _, ch := range line {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == ':' {
			fields = append(fields, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	fields = append(fields, current.String())
	return fields
}

func matchPgpassField(pattern, value string) bool {
	return pattern == "*" || pattern == value
}

// Executor handles PostgreSQL query execution.
type Executor struct {
	db       *sql.DB
	config   ConnectionConfig
	database string
}

// NewExecutor creates a new PostgreSQL executor.
func NewExecutor(config ConnectionConfig) (*Executor, error) {
	db, err := sql.Open("pgx", config.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}
	return &Executor{
		db:       db,
		config:   config,
		database: config.Database,
	}, nil
}

// Close closes the database connection.
func (e *Executor) Close() error {
	return e.db.Close()
}

// DB returns the underlying database connection.
func (e *Executor) DB() *sql.DB {
	return e.db
}

// Database returns the current database name.
func (e *Executor) Database() string {
	return e.database
}

// ServerVersion returns the PostgreSQL server version.
func (e *Executor) ServerVersion() (string, error) {
	var version string
	err := e.db.QueryRow("SHOW server_version").Scan(&version)
	return version, err
}

// Execute runs a query and returns the result.
func (e *Executor) Execute(ctx context.Context, query string) (*format.QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// Determine if this is a query that returns rows
	upper := strings.ToUpper(query)
	isSelect := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "TABLE") ||
		strings.HasPrefix(upper, "VALUES") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "FETCH") ||
		strings.Contains(upper, "RETURNING")

	if isSelect {
		return e.executeQuery(ctx, query)
	}
	return e.executeExec(ctx, query)
}

func (e *Executor) executeQuery(ctx context.Context, query string) (*format.QueryResult, error) {
	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var resultRows [][]string
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, len(cols))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &format.QueryResult{
		Columns:    cols,
		Rows:       resultRows,
		StatusText: fmt.Sprintf("(%d row%s)", len(resultRows), pluralS(len(resultRows))),
		RowCount:   len(resultRows),
	}, nil
}

func (e *Executor) executeExec(ctx context.Context, query string) (*format.QueryResult, error) {
	result, err := e.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	upper := strings.ToUpper(strings.TrimSpace(query))
	cmd := strings.Fields(upper)[0]
	statusText := fmt.Sprintf("%s %d", cmd, affected)

	// Check if db changed
	if strings.HasPrefix(upper, "\\C ") || strings.HasPrefix(upper, "\\CONNECT") {
		// handled by special commands
	}

	return &format.QueryResult{
		StatusText: statusText,
	}, nil
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Schema metadata queries

// SearchPath returns the current search_path schemas.
func (e *Executor) SearchPath(ctx context.Context) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT unnest(current_schemas(true))")
	if err != nil {
		return []string{"public"}, nil
	}
	defer rows.Close()
	var schemas []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			schemas = append(schemas, s)
		}
	}
	if len(schemas) == 0 {
		return []string{"public"}, nil
	}
	return schemas, nil
}

// Tables returns all table names in the given schema (or all schemas if empty).
// Tables on the search_path are returned without schema prefix.
func (e *Executor) Tables(ctx context.Context, schema string) ([]string, error) {
	if schema != "" {
		query := `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name`
		return e.queryStrings(ctx, query, schema)
	}

	searchPath, _ := e.SearchPath(ctx)
	spSet := make(map[string]bool)
	for _, s := range searchPath {
		spSet[s] = true
	}

	query := `SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name`
	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var tableSchema, tableName string
		if err := rows.Scan(&tableSchema, &tableName); err != nil {
			return nil, err
		}
		if spSet[tableSchema] {
			results = append(results, tableName)
		} else {
			results = append(results, tableSchema+"."+tableName)
		}
	}
	return results, rows.Err()
}

// Columns returns all column names for the given table.
func (e *Executor) Columns(ctx context.Context, table string) ([]string, error) {
	parts := strings.SplitN(table, ".", 2)
	var query string
	var args []interface{}
	if len(parts) == 2 {
		query = `SELECT column_name FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
		args = []interface{}{parts[0], parts[1]}
	} else {
		query = `SELECT column_name FROM information_schema.columns
			WHERE table_name = $1 ORDER BY ordinal_position`
		args = []interface{}{parts[0]}
	}
	return e.queryStrings(ctx, query, args...)
}

// Schemas returns all schema names.
func (e *Executor) Schemas(ctx context.Context) ([]string, error) {
	query := `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`
	return e.queryStrings(ctx, query)
}

// Functions returns all function names.
func (e *Executor) Functions(ctx context.Context, schema string) ([]string, error) {
	query := `SELECT routine_name FROM information_schema.routines
		WHERE routine_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY routine_name`
	if schema != "" {
		query = `SELECT routine_name FROM information_schema.routines
			WHERE routine_schema = $1 ORDER BY routine_name`
		return e.queryStrings(ctx, query, schema)
	}
	return e.queryStrings(ctx, query)
}

// Databases returns all database names.
func (e *Executor) Databases(ctx context.Context) ([]string, error) {
	query := `SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`
	return e.queryStrings(ctx, query)
}

// Views returns all view names.
func (e *Executor) Views(ctx context.Context, schema string) ([]string, error) {
	query := `SELECT table_name FROM information_schema.views
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_name`
	if schema != "" {
		query = `SELECT table_name FROM information_schema.views WHERE table_schema = $1 ORDER BY table_name`
		return e.queryStrings(ctx, query, schema)
	}
	return e.queryStrings(ctx, query)
}

// Datatypes returns common PostgreSQL data types.
func (e *Executor) Datatypes(_ context.Context) []string {
	return []string{
		"bigint", "bigserial", "bit", "bit varying", "boolean", "box",
		"bytea", "character", "character varying", "cidr", "circle",
		"date", "double precision", "inet", "integer", "interval",
		"json", "jsonb", "line", "lseg", "macaddr", "macaddr8",
		"money", "numeric", "path", "pg_lsn", "pg_snapshot", "point",
		"polygon", "real", "smallint", "smallserial", "serial",
		"text", "time", "time with time zone", "timestamp",
		"timestamp with time zone", "tsquery", "tsvector", "txid_snapshot",
		"uuid", "xml",
	}
}

func (e *Executor) queryStrings(ctx context.Context, query string, args ...interface{}) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}
