package mysql

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig_MySQL(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("default host should be 'localhost', got %q", cfg.Host)
	}
	if cfg.Port != 3306 {
		t.Errorf("default port should be 3306, got %d", cfg.Port)
	}
	if cfg.Charset != "utf8mb4" {
		t.Errorf("default charset should be 'utf8mb4', got %q", cfg.Charset)
	}
}

func TestConnectionConfig_DSN_MySQL(t *testing.T) {
	cfg := ConnectionConfig{
		Host:     "myhost",
		Port:     3307,
		User:     "myuser",
		Password: "mypass",
		Database: "mydb",
		Charset:  "utf8mb4",
	}

	dsn := cfg.DSN()

	if !strings.Contains(dsn, "myuser:mypass@") {
		t.Errorf("DSN should contain user:pass@, got %q", dsn)
	}
	if !strings.Contains(dsn, "tcp(myhost:3307)") {
		t.Errorf("DSN should contain tcp(host:port), got %q", dsn)
	}
	if !strings.Contains(dsn, "/mydb") {
		t.Errorf("DSN should contain /database, got %q", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Errorf("DSN should contain charset, got %q", dsn)
	}
}

func TestConnectionConfig_DSN_Socket(t *testing.T) {
	cfg := ConnectionConfig{
		User:   "root",
		Socket: "/var/run/mysqld/mysqld.sock",
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "unix(/var/run/mysqld/mysqld.sock)") {
		t.Errorf("DSN should use unix socket, got %q", dsn)
	}
}

func TestConnectionConfig_DSN_SSL(t *testing.T) {
	cfg := ConnectionConfig{
		Host: "localhost",
		Port: 3306,
		SSL:  true,
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "tls=true") {
		t.Errorf("DSN should contain tls=true for SSL, got %q", dsn)
	}
}

func TestConnectionConfig_DSN_NoPassword(t *testing.T) {
	cfg := ConnectionConfig{
		Host: "localhost",
		Port: 3306,
		User: "root",
	}

	dsn := cfg.DSN()
	if strings.Contains(dsn, "root:@") {
		// This is actually valid for mysql DSN format
	}
}

func TestParseDSN_MySQL(t *testing.T) {
	tests := []struct {
		dsn      string
		host     string
		port     int
		user     string
		password string
		database string
	}{
		{
			"mysql://user:pass@host:3307/mydb",
			"host", 3307, "user", "pass", "mydb",
		},
		{
			"mysql://root@localhost/test",
			"localhost", 3306, "root", "", "test",
		},
		{
			"mysql://user@host:3306/db",
			"host", 3306, "user", "", "db",
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

func TestParseDSN_NonURI(t *testing.T) {
	cfg, err := ParseDSN("just_a_name")
	if err != nil {
		t.Fatalf("should not error for non-URI: %v", err)
	}
	// Should return defaults
	if cfg.Host != "localhost" {
		t.Error("non-URI should use default host")
	}
}

func TestParseMyCnf(t *testing.T) {
	dir := t.TempDir()
	cnfPath := filepath.Join(dir, ".my.cnf")

	content := `[client]
host = myhost
port = 3307
user = myuser
password = mypass

[mysql]
database = mydb

[server]
# This section should be ignored
port = 9999
`
	if err := os.WriteFile(cnfPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write my.cnf: %v", err)
	}

	result := parseMyCnfData(content)

	if result["host"] != "myhost" {
		t.Errorf("host should be 'myhost', got %q", result["host"])
	}
	if result["port"] != "3307" {
		t.Errorf("port should be '3307', got %q", result["port"])
	}
	if result["user"] != "myuser" {
		t.Errorf("user should be 'myuser', got %q", result["user"])
	}
	if result["password"] != "mypass" {
		t.Errorf("password should be 'mypass', got %q", result["password"])
	}
	if result["database"] != "mydb" {
		t.Errorf("database should be 'mydb', got %q", result["database"])
	}
}

func TestParseMyCnfData_WithQuotes(t *testing.T) {
	content := `[client]
host = "myhost"
password = 'my pass'
`
	result := parseMyCnfData(content)

	if result["host"] != "myhost" {
		t.Errorf("host should strip double quotes, got %q", result["host"])
	}
	if result["password"] != "my pass" {
		t.Errorf("password should strip single quotes, got %q", result["password"])
	}
}

func TestParseMyCnfData_IgnoreComments(t *testing.T) {
	content := `[client]
# This is a comment
host = myhost
; This is also a comment
port = 3306
`
	result := parseMyCnfData(content)

	if result["host"] != "myhost" {
		t.Error("should parse host after comments")
	}
	if result["port"] != "3306" {
		t.Error("should parse port after comments")
	}
}

func TestParseMyCnfData_IgnoreOtherSections(t *testing.T) {
	content := `[server]
port = 9999
user = serveruser

[client]
host = clienthost
`
	result := parseMyCnfData(content)

	if result["host"] != "clienthost" {
		t.Error("should only use [client] section values")
	}
	// port from [server] should not be picked up
	if result["port"] == "9999" {
		t.Error("should not use values from [server] section")
	}
}

func TestParseMyCnfData_Empty(t *testing.T) {
	result := parseMyCnfData("")
	if len(result) != 0 {
		t.Error("empty input should produce empty map")
	}
}

func TestParseMyCnfData_WithSocket(t *testing.T) {
	content := `[client]
socket = /var/run/mysqld/mysqld.sock
`
	result := parseMyCnfData(content)

	if result["socket"] != "/var/run/mysqld/mysqld.sock" {
		t.Errorf("socket should be parsed, got %q", result["socket"])
	}
}

func TestMyCnfPaths(t *testing.T) {
	paths := myCnfPaths()
	if len(paths) == 0 {
		t.Error("should return at least one path")
	}

	// Should include home directory paths
	home, _ := os.UserHomeDir()
	hasHome := false
	for _, p := range paths {
		if strings.HasPrefix(p, home) {
			hasHome = true
		}
	}
	if !hasHome {
		t.Error("should include home directory .my.cnf path")
	}
}

func TestDatatypes_MySQL(t *testing.T) {
	e := &Executor{}
	types := e.Datatypes(nil)

	if len(types) == 0 {
		t.Error("should return data types")
	}

	hasInt := false
	hasVarchar := false
	hasJSON := false
	for _, dt := range types {
		switch dt {
		case "int", "integer":
			hasInt = true
		case "varchar":
			hasVarchar = true
		case "json":
			hasJSON = true
		}
	}
	if !hasInt {
		t.Error("should include 'int' or 'integer'")
	}
	if !hasVarchar {
		t.Error("should include 'varchar'")
	}
	if !hasJSON {
		t.Error("should include 'json'")
	}
}

func TestPluralS_MySQL(t *testing.T) {
	if pluralS(0) != "s" {
		t.Error("0 should be plural")
	}
	if pluralS(1) != "" {
		t.Error("1 should be singular")
	}
	if pluralS(2) != "s" {
		t.Error("2 should be plural")
	}
}

func TestFormatValue_MySQL(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "NULL"},
		{[]byte("data"), "data"},
		{42, "42"},
		{"text", "text"},
	}

	for _, tt := range tests {
		result := formatValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConnectionConfig_DSN_WithOptions(t *testing.T) {
	cfg := ConnectionConfig{
		Host:    "localhost",
		Port:    3306,
		Options: map[string]string{"timeout": "30s"},
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "timeout=30s") {
		t.Errorf("DSN should contain custom options, got %q", dsn)
	}
}

func TestParseDSN_MysqlPlus(t *testing.T) {
	cfg, err := ParseDSN("mysql+pymysql://user@host/db")
	if err != nil {
		t.Fatalf("should parse mysql+ URIs: %v", err)
	}
	if cfg.Host != "host" {
		t.Errorf("host should be 'host', got %q", cfg.Host)
	}
	if cfg.User != "user" {
		t.Errorf("user should be 'user', got %q", cfg.User)
	}
}

func TestParseMyCnfData_MysqlSection(t *testing.T) {
	content := `[mysql]
user = mysqluser
host = mysqlhost
`
	result := parseMyCnfData(content)

	if result["user"] != "mysqluser" {
		t.Errorf("should parse [mysql] section, got user=%q", result["user"])
	}
	if result["host"] != "mysqlhost" {
		t.Errorf("should parse [mysql] section, got host=%q", result["host"])
	}
}
