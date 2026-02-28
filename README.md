# gocli

A fast Go reimplementation of [pgcli](https://www.pgcli.com/) and [mycli](https://www.mycli.net/) — interactive CLI clients for PostgreSQL and MySQL with auto-completion and syntax highlighting.

## Features

- **Context-aware auto-completion** — suggests tables after `FROM`, columns after `SELECT`/`WHERE`, keywords in the right context, with fuzzy matching
- **Syntax highlighting** — colorizes SQL keywords, strings, numbers, comments, and functions
- **Multiple output formats** — table (ASCII/Unicode), CSV, TSV, JSON, vertical/expanded
- **Special commands** — full support for `\d`, `\dt`, `\l`, `\sf`, `\x`, `\timing`, favorites, and more
- **Config file compatibility** — reads existing pgcli/mycli INI-style config files
- **Connection options** — URIs, `.pgpass`/`.my.cnf`, DSN aliases, environment variables, SSL

## Building

```bash
# Build both binaries
go build -o pgcli ./cmd/pgcli
go build -o mycli ./cmd/mycli
```

## Testing

```bash
# Run the full test suite (no database required)
go test ./...

# Verbose output
go test -v ./...

# Run a specific package's tests
go test -v ./internal/completion/
go test -v ./internal/format/
```

All 150+ tests run without a live database — they test parsing, formatting, completion, config, highlighting, and command handling using unit tests.

## Usage: pgcli (PostgreSQL)

```bash
# Connect to a local database
./pgcli mydb

# Specify host, port, user
./pgcli -h localhost -p 5432 -U myuser mydb

# Use a connection URI
./pgcli postgres://myuser:mypass@localhost:5432/mydb

# Use environment variables (same as psql)
PGHOST=localhost PGUSER=myuser PGDATABASE=mydb ./pgcli

# Force password prompt
./pgcli -W -U myuser mydb

# Execute a query and exit
./pgcli -e "SELECT * FROM users LIMIT 10" mydb

# List databases
./pgcli -l

# Ping and exit (check connectivity)
./pgcli --ping mydb

# Use a DSN alias from config
./pgcli -D production

# Custom config file
./pgcli --pgclirc /path/to/config mydb
```

### pgcli flags

| Flag | Description |
|------|-------------|
| `-h` | Host address (default: `localhost`) |
| `-p` | Port number (default: `5432`) |
| `-U` | Username |
| `-W` | Force password prompt |
| `-w` | Never prompt for password |
| `-d` | Database name |
| `-D` | DSN alias from config |
| `-l` | List databases and exit |
| `-e` | Execute command and exit |
| `-v` | Print version |
| `--pgclirc` | Config file path |
| `--sslmode` | SSL mode |
| `--less-chatty` | Skip intro/goodbye messages |
| `--prompt` | Custom prompt format |
| `--auto-vertical-output` | Auto-switch to vertical for wide results |
| `--row-limit` | Limit rows returned |
| `--ping` | Test connectivity and exit |
| `--init-command` | SQL to run after connecting |
| `--log-file` | Log queries to file |
| `--single-connection` | Single connection mode |
| `--application-name` | Application name (default: `gocli`) |

### pgcli special commands

| Command | Description |
|---------|-------------|
| `\dt[+]` | List tables |
| `\dv[+]` | List views |
| `\di[+]` | List indexes |
| `\df[+]` | List functions |
| `\dn[+]` | List schemas |
| `\du` | List roles |
| `\l` | List databases |
| `\d <name>` | Describe table/view |
| `\dx` | List extensions |
| `\sf <name>` | Show function definition |
| `\x` | Toggle expanded output |
| `\timing` | Toggle query timing |
| `\pager <cmd>` | Set pager |
| `\e` | Edit query in `$EDITOR` |
| `\!` | Execute shell command |
| `\f` / `\fs` / `\fd` | List/save/delete favorites |
| `\?` | Show help |
| `\q` | Quit |

## Usage: mycli (MySQL)

```bash
# Connect to a local database
./mycli mydb

# Specify host, port, user
./mycli -h localhost -P 3306 -u myuser mydb

# Use a connection URI
./mycli mysql://myuser:mypass@localhost:3306/mydb

# Prompt for password
./mycli -p -u myuser mydb

# Execute and exit
./mycli -e "SHOW TABLES" mydb
```

### mycli flags

| Flag | Description |
|------|-------------|
| `-h` | Host address (default: `localhost`) |
| `-P` | Port number (default: `3306`) |
| `-u` | Username |
| `-p` | Prompt for password |
| `-D` | Database name |
| `-d` | DSN alias from config |
| `-S` | Socket file path |
| `-e` | Execute command and exit |
| `-V` | Print version |
| `--myclirc` | Config file path |
| `--less-chatty` | Skip intro/goodbye |
| `-R` | Custom prompt format |
| `--auto-vertical-output` | Auto vertical for wide results |
| `--init-command` | SQL to run after connecting |
| `--ssl-mode` | SSL mode: `auto`, `on`, `off` |
| `--ssl-ca/cert/key` | SSL certificate files |
| `--charset` | Character set |
| `--warn` | Warn before destructive commands (default: true) |
| `-l` | Audit log file |
| `-t` | Force table output |
| `--csv` | Force CSV output |

## Testing with a local PostgreSQL database

### Quick start

```bash
# 1. Build
go build -o pgcli ./cmd/pgcli

# 2. Connect (uses PGHOST/PGUSER/PGDATABASE env vars, or defaults)
./pgcli -h localhost -U postgres mydb

# 3. You'll see:
#   gocli (pgcli) 0.1.0
#   Server: PostgreSQL 16.2
#   Database: mydb
#   Type \? for help.
#
#   mydb>
```

### Common test queries

```sql
-- List tables
\dt

-- Describe a table
\d users

-- Run a query (results in table format)
SELECT * FROM users LIMIT 5;

-- Toggle expanded/vertical output
\x
SELECT * FROM users LIMIT 1;

-- Show timing
\timing
SELECT count(*) FROM users;

-- List databases
\l

-- List schemas
\dn

-- Show function source
\sf my_function
```

### Connection methods

```bash
# Standard flags
./pgcli -h localhost -p 5432 -U postgres mydb

# URI
./pgcli postgres://postgres:secret@localhost:5432/mydb

# Environment variables (same as psql)
export PGHOST=localhost
export PGUSER=postgres
export PGDATABASE=mydb
export PGPASSWORD=secret
./pgcli

# .pgpass file (auto-detected from ~/.pgpass)
# Format: hostname:port:database:username:password
echo "localhost:5432:mydb:postgres:secret" >> ~/.pgpass
chmod 600 ~/.pgpass
./pgcli -U postgres mydb
```

### Testing with a local MySQL database

```bash
go build -o mycli ./cmd/mycli

# Connect
./mycli -h localhost -u root mydb

# Or with password prompt
./mycli -h localhost -u root -p mydb
```

## Configuration

Config files are compatible with pgcli/mycli format. Default locations:

- pgcli: `~/.config/pgcli/config` (or `$PGCLIRC`)
- mycli: `~/.myclirc`

Example config:

```ini
[main]
multi_line = False
smart_completion = True
keyword_casing = auto
syntax_style = default
table_format = ascii
prompt = \u@\h:\d>
less_chatty = False
destructive_warning = True
row_limit = 1000

[favorite_queries]
show_slow = SELECT * FROM pg_stat_activity WHERE state != 'idle'

[alias_dsn]
production = postgres://prod-host:5432/myapp
staging = postgres://staging-host:5432/myapp

[colors]
keyword = blue
string = green
number = yellow
```

### Prompt tokens

| Token | Meaning |
|-------|---------|
| `\u` | Username |
| `\h` | Hostname |
| `\d` | Database name |
| `\p` | Port |
| `\n` | Newline |
| `\#` | `#` if superuser, `>` otherwise |

## Architecture

```
cmd/
  pgcli/main.go       # PostgreSQL entry point
  mycli/main.go        # MySQL entry point
internal/
  cli/app.go           # REPL loop, input handling, prompt
  completion/           # Context-aware SQL auto-completion
  config/               # INI config parser, prompt formatting
  format/               # Output formatting (table, CSV, JSON, etc.)
  highlight/            # SQL syntax highlighting and tokenization
  mysql/                # MySQL executor and connection handling
  pg/                   # PostgreSQL executor and connection handling
  special/              # Backslash command registry (pg.go, mysql.go)
```

## License

MIT
