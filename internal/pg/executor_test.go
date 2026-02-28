package pg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("default host should be 'localhost', got %q", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("default port should be 5432, got %d", cfg.Port)
	}
	if cfg.SSLMode != "prefer" {
		t.Errorf("default sslmode should be 'prefer', got %q", cfg.SSLMode)
	}
}

func TestConnectionConfig_DSN(t *testing.T) {
	cfg := ConnectionConfig{
		Host:     "myhost",
		Port:     5433,
		User:     "myuser",
		Password: "mypass",
		Database: "mydb",
		SSLMode:  "require",
	}

	dsn := cfg.DSN()

	if !strings.Contains(dsn, "myhost:5433") {
		t.Errorf("DSN should contain host:port, got %q", dsn)
	}
	if !strings.Contains(dsn, "myuser") {
		t.Errorf("DSN should contain user, got %q", dsn)
	}
	if !strings.Contains(dsn, "mydb") {
		t.Errorf("DSN should contain database, got %q", dsn)
	}
	if !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("DSN should contain sslmode, got %q", dsn)
	}
}

func TestConnectionConfig_DSN_NoPassword(t *testing.T) {
	cfg := ConnectionConfig{
		Host: "localhost",
		Port: 5432,
		User: "user",
	}

	dsn := cfg.DSN()
	if strings.Contains(dsn, ":@") {
		t.Error("DSN without password should not have ':@'")
	}
}

func TestConnectionConfig_DSN_WithOptions(t *testing.T) {
	cfg := ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Database: "db",
		Options:  map[string]string{"application_name": "test"},
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "application_name=test") {
		t.Errorf("DSN should contain custom options, got %q", dsn)
	}
}

func TestParseDSN_URI(t *testing.T) {
	tests := []struct {
		dsn      string
		host     string
		port     int
		user     string
		password string
		database string
	}{
		{
			"postgres://user:pass@host:5433/mydb",
			"host", 5433, "user", "pass", "mydb",
		},
		{
			"postgresql://admin@localhost/production",
			"localhost", 5432, "admin", "", "production",
		},
		{
			"postgres://localhost:5432/test?sslmode=disable",
			"localhost", 5432, "", "", "test",
		},
		{
			"postgres://user@host/db",
			"host", 5432, "user", "", "db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			cfg, err := ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Host != tt.host {
				t.Errorf("host = %q, want %q", cfg.Host, tt.host)
			}
			if cfg.Port != tt.port {
				t.Errorf("port = %d, want %d", cfg.Port, tt.port)
			}
			if cfg.User != tt.user {
				t.Errorf("user = %q, want %q", cfg.User, tt.user)
			}
			if cfg.Password != tt.password {
				t.Errorf("password = %q, want %q", cfg.Password, tt.password)
			}
			if cfg.Database != tt.database {
				t.Errorf("database = %q, want %q", cfg.Database, tt.database)
			}
		})
	}
}

func TestParseDSN_KeyValue(t *testing.T) {
	dsn := "host=myhost port=5433 user=myuser password=secret dbname=mydb sslmode=require"

	cfg, err := ParseDSN(dsn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "myhost" {
		t.Errorf("host = %q, want 'myhost'", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("port = %d, want 5433", cfg.Port)
	}
	if cfg.User != "myuser" {
		t.Errorf("user = %q, want 'myuser'", cfg.User)
	}
	if cfg.Password != "secret" {
		t.Errorf("password = %q, want 'secret'", cfg.Password)
	}
	if cfg.Database != "mydb" {
		t.Errorf("database = %q, want 'mydb'", cfg.Database)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("sslmode = %q, want 'require'", cfg.SSLMode)
	}
}

func TestParseDSN_Invalid(t *testing.T) {
	// Key-value format with partial data should still parse
	cfg, err := ParseDSN("host=localhost")
	if err != nil {
		t.Fatalf("should not error for partial DSN: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Error("should parse partial host")
	}
}

func TestParsePgpass(t *testing.T) {
	dir := t.TempDir()
	pgpassPath := filepath.Join(dir, ".pgpass")

	content := `# This is a comment
localhost:5432:mydb:myuser:secret123
*:5432:*:admin:adminpass
server.example.com:5433:production:deploy:deploypass
`
	if err := os.WriteFile(pgpassPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write pgpass: %v", err)
	}

	// Set PGPASSFILE env var for testing
	oldEnv := os.Getenv("PGPASSFILE")
	os.Setenv("PGPASSFILE", pgpassPath)
	defer os.Setenv("PGPASSFILE", oldEnv)

	// Exact match
	pw := ParsePgpass("localhost", 5432, "mydb", "myuser")
	if pw != "secret123" {
		t.Errorf("expected 'secret123', got %q", pw)
	}

	// Wildcard match
	pw = ParsePgpass("anyhost", 5432, "anydb", "admin")
	if pw != "adminpass" {
		t.Errorf("expected 'adminpass', got %q", pw)
	}

	// Specific host match
	pw = ParsePgpass("server.example.com", 5433, "production", "deploy")
	if pw != "deploypass" {
		t.Errorf("expected 'deploypass', got %q", pw)
	}

	// No match
	pw = ParsePgpass("unknown", 9999, "nodb", "nouser")
	if pw != "" {
		t.Errorf("expected empty string for no match, got %q", pw)
	}
}

func TestParsePgpass_Escaped(t *testing.T) {
	dir := t.TempDir()
	pgpassPath := filepath.Join(dir, ".pgpass")

	// Test escaped colons and backslashes
	content := `host\:name:5432:db:user:pass\:word
`
	if err := os.WriteFile(pgpassPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write pgpass: %v", err)
	}

	oldEnv := os.Getenv("PGPASSFILE")
	os.Setenv("PGPASSFILE", pgpassPath)
	defer os.Setenv("PGPASSFILE", oldEnv)

	pw := ParsePgpass("host:name", 5432, "db", "user")
	if pw != "pass:word" {
		t.Errorf("expected 'pass:word', got %q", pw)
	}
}

func TestParsePgpass_NoFile(t *testing.T) {
	oldEnv := os.Getenv("PGPASSFILE")
	os.Setenv("PGPASSFILE", "/nonexistent/path/.pgpass")
	defer os.Setenv("PGPASSFILE", oldEnv)

	pw := ParsePgpass("localhost", 5432, "db", "user")
	if pw != "" {
		t.Error("should return empty for missing file")
	}
}

func TestParsePgpassLine(t *testing.T) {
	tests := []struct {
		line     string
		expected []string
	}{
		{"host:5432:db:user:pass", []string{"host", "5432", "db", "user", "pass"}},
		{`host\:name:5432:db:user:p\:ass`, []string{"host:name", "5432", "db", "user", "p:ass"}},
		{"*:*:*:*:wildcard", []string{"*", "*", "*", "*", "wildcard"}},
	}

	for _, tt := range tests {
		result := parsePgpassLine(tt.line)
		if len(result) != len(tt.expected) {
			t.Errorf("parsePgpassLine(%q) = %v, want %v", tt.line, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("parsePgpassLine(%q)[%d] = %q, want %q", tt.line, i, v, tt.expected[i])
			}
		}
	}
}

func TestMatchPgpassField(t *testing.T) {
	if !matchPgpassField("*", "anything") {
		t.Error("* should match anything")
	}
	if !matchPgpassField("exact", "exact") {
		t.Error("exact match should work")
	}
	if matchPgpassField("foo", "bar") {
		t.Error("different values should not match")
	}
}

func TestDatatypes(t *testing.T) {
	e := &Executor{}
	types := e.Datatypes(nil)

	if len(types) == 0 {
		t.Error("should return data types")
	}

	// Check some common types
	hasInt := false
	hasText := false
	hasJSON := false
	for _, dt := range types {
		switch dt {
		case "integer":
			hasInt = true
		case "text":
			hasText = true
		case "jsonb":
			hasJSON = true
		}
	}
	if !hasInt {
		t.Error("should include 'integer'")
	}
	if !hasText {
		t.Error("should include 'text'")
	}
	if !hasJSON {
		t.Error("should include 'jsonb'")
	}
}

func TestPluralS(t *testing.T) {
	if pluralS(0) != "s" {
		t.Error("0 should be plural")
	}
	if pluralS(1) != "" {
		t.Error("1 should be singular")
	}
	if pluralS(2) != "s" {
		t.Error("2 should be plural")
	}
	if pluralS(100) != "s" {
		t.Error("100 should be plural")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "NULL"},
		{[]byte("hello"), "hello"},
		{42, "42"},
		{"text", "text"},
		{3.14, "3.14"},
		{true, "true"},
	}

	for _, tt := range tests {
		result := formatValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseDSN_URI_NoUserDoesNotInheritDefault(t *testing.T) {
	cfg, err := ParseDSN("postgres://localhost:5432/testdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "" {
		t.Errorf("URI without user should have empty user, got %q", cfg.User)
	}

	defCfg := DefaultConfig()
	if defCfg.User == "" {
		t.Skip("USER env not set, cannot verify default differs")
	}
	if cfg.User == defCfg.User {
		t.Errorf("ParseDSN without user should not inherit DefaultConfig user %q", defCfg.User)
	}
}

func TestParseDSN_SSLMode(t *testing.T) {
	cfg, err := ParseDSN("postgres://localhost/db?sslmode=disable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("sslmode should be 'disable', got %q", cfg.SSLMode)
	}
}

func TestParseDSN_Defaults(t *testing.T) {
	cfg, err := ParseDSN("host=myhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have defaults for unspecified fields
	if cfg.Port != 5432 {
		t.Errorf("default port should be 5432, got %d", cfg.Port)
	}
	if cfg.SSLMode != "prefer" {
		t.Errorf("default sslmode should be 'prefer', got %q", cfg.SSLMode)
	}
}
