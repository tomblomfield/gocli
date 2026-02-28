// Package mysql implements MySQL-specific query execution and metadata.
package mysql

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

	_ "github.com/go-sql-driver/mysql"
)

// ConnectionConfig holds MySQL connection parameters.
type ConnectionConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Socket   string
	SSL      bool
	SSLCa    string
	SSLCert  string
	SSLKey   string
	Charset  string
	Options  map[string]string
}

// DefaultConfig returns default connection parameters.
func DefaultConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:    "localhost",
		Port:    3306,
		User:    os.Getenv("USER"),
		Charset: "utf8mb4",
	}
}

// DSN returns the MySQL DSN string for go-sql-driver/mysql.
func (c ConnectionConfig) DSN() string {
	var dsn strings.Builder
	if c.User != "" {
		dsn.WriteString(c.User)
		if c.Password != "" {
			dsn.WriteString(":")
			dsn.WriteString(c.Password)
		}
		dsn.WriteString("@")
	}
	if c.Socket != "" {
		dsn.WriteString(fmt.Sprintf("unix(%s)", c.Socket))
	} else {
		dsn.WriteString(fmt.Sprintf("tcp(%s:%d)", c.Host, c.Port))
	}
	dsn.WriteString("/")
	if c.Database != "" {
		dsn.WriteString(c.Database)
	}
	params := url.Values{}
	if c.Charset != "" {
		params.Set("charset", c.Charset)
	}
	params.Set("parseTime", "true")
	params.Set("multiStatements", "true")
	if c.SSL {
		params.Set("tls", "true")
	}
	for k, v := range c.Options {
		params.Set(k, v)
	}
	if len(params) > 0 {
		dsn.WriteString("?")
		dsn.WriteString(params.Encode())
	}
	return dsn.String()
}

// ParseDSN parses a MySQL URI or individual parameters.
func ParseDSN(dsn string) (ConnectionConfig, error) {
	cfg := DefaultConfig()

	if strings.HasPrefix(dsn, "mysql://") || strings.HasPrefix(dsn, "mysql+") {
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
		}
		if u.Path != "" && u.Path != "/" {
			cfg.Database = strings.TrimPrefix(u.Path, "/")
		}
		return cfg, nil
	}

	return cfg, nil
}

// ParseMyCnf reads MySQL credentials from my.cnf/my.ini.
func ParseMyCnf() ConnectionConfig {
	cfg := DefaultConfig()

	paths := myCnfPaths()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsed := parseMyCnfData(string(data))
		if v, ok := parsed["host"]; ok && v != "" {
			cfg.Host = v
		}
		if v, ok := parsed["port"]; ok && v != "" {
			p, err := strconv.Atoi(v)
			if err == nil {
				cfg.Port = p
			}
		}
		if v, ok := parsed["user"]; ok && v != "" {
			cfg.User = v
		}
		if v, ok := parsed["password"]; ok && v != "" {
			cfg.Password = v
		}
		if v, ok := parsed["database"]; ok && v != "" {
			cfg.Database = v
		}
		if v, ok := parsed["socket"]; ok && v != "" {
			cfg.Socket = v
		}
	}

	return cfg
}

func myCnfPaths() []string {
	var paths []string
	if runtime.GOOS == "windows" {
		paths = append(paths, filepath.Join(os.Getenv("WINDIR"), "my.ini"))
		paths = append(paths, filepath.Join(os.Getenv("WINDIR"), "my.cnf"))
	} else {
		paths = append(paths, "/etc/my.cnf", "/etc/mysql/my.cnf")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, filepath.Join(home, ".my.cnf"))
		paths = append(paths, filepath.Join(home, ".mylogin.cnf"))
	}
	return paths
}

func parseMyCnfData(data string) map[string]string {
	result := make(map[string]string)
	inClientSection := false

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section := strings.Trim(line, "[]")
			inClientSection = section == "client" || section == "mysql"
			continue
		}
		if !inClientSection {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove surrounding quotes
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}
	return result
}

// Executor handles MySQL query execution.
type Executor struct {
	db       *sql.DB
	config   ConnectionConfig
	database string
}

// NewExecutor creates a new MySQL executor.
func NewExecutor(config ConnectionConfig) (*Executor, error) {
	db, err := sql.Open("mysql", config.DSN())
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

// ServerVersion returns the MySQL server version.
func (e *Executor) ServerVersion() (string, error) {
	var version string
	err := e.db.QueryRow("SELECT VERSION()").Scan(&version)
	return version, err
}

// Execute runs a query and returns the result.
func (e *Executor) Execute(ctx context.Context, query string) (*format.QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	upper := strings.ToUpper(query)
	isSelect := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESCRIBE") ||
		strings.HasPrefix(upper, "DESC") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "TABLE")

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
		StatusText: fmt.Sprintf("%d row%s in set", len(resultRows), pluralS(len(resultRows))),
		RowCount:   len(resultRows),
	}, nil
}

func (e *Executor) executeExec(ctx context.Context, query string) (*format.QueryResult, error) {
	result, err := e.db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()

	// Track database changes from USE statements
	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "USE ") {
		db := strings.TrimSpace(query[4:])
		db = strings.Trim(db, "`\"' ;")
		e.database = db
	}

	return &format.QueryResult{
		StatusText: fmt.Sprintf("Query OK, %d row%s affected", affected, pluralS(int(affected))),
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

// Tables returns all table names.
func (e *Executor) Tables(ctx context.Context, database string) ([]string, error) {
	if database == "" {
		database = e.database
	}
	if database == "" {
		return nil, nil
	}
	query := `SELECT table_name FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name`
	return e.queryStrings(ctx, query, database)
}

// Columns returns all column names for the given table.
func (e *Executor) Columns(ctx context.Context, table string) ([]string, error) {
	parts := strings.SplitN(table, ".", 2)
	var query string
	var args []interface{}
	if len(parts) == 2 {
		query = `SELECT column_name FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`
		args = []interface{}{parts[0], parts[1]}
	} else {
		query = `SELECT column_name FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`
		args = []interface{}{e.database, parts[0]}
	}
	return e.queryStrings(ctx, query, args...)
}

// Schemas returns all database/schema names.
func (e *Executor) Schemas(ctx context.Context) ([]string, error) {
	query := `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`
	return e.queryStrings(ctx, query)
}

// Functions returns all function names.
func (e *Executor) Functions(ctx context.Context, database string) ([]string, error) {
	if database == "" {
		database = e.database
	}
	query := `SELECT routine_name FROM information_schema.routines WHERE routine_schema = ? ORDER BY routine_name`
	return e.queryStrings(ctx, query, database)
}

// Databases returns all database names.
func (e *Executor) Databases(ctx context.Context) ([]string, error) {
	query := `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`
	return e.queryStrings(ctx, query)
}

// Datatypes returns common MySQL data types.
func (e *Executor) Datatypes(_ context.Context) []string {
	return []string{
		"bigint", "binary", "bit", "blob", "boolean", "char",
		"date", "datetime", "decimal", "double", "enum",
		"float", "geometry", "int", "integer", "json",
		"linestring", "longblob", "longtext", "mediumblob",
		"mediumint", "mediumtext", "multilinestring", "multipoint",
		"multipolygon", "numeric", "point", "polygon",
		"real", "set", "smallint", "text", "time",
		"timestamp", "tinyblob", "tinyint", "tinytext",
		"varbinary", "varchar", "year",
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
