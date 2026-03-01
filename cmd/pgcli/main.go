// pgcli is a Go reimplementation of pgcli - a PostgreSQL CLI with auto-completion and syntax highlighting.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/tomblomfield/gocli/internal/cli"
	"github.com/tomblomfield/gocli/internal/config"
	"github.com/tomblomfield/gocli/internal/pg"
)

var (
	version = "0.1.0"

	host       = flag.String("h", "", "Host address of the postgres database")
	port       = flag.Int("p", 5432, "Port number")
	username   = flag.String("U", "", "Username")
	password   = flag.Bool("W", false, "Force password prompt")
	noPassword = flag.Bool("w", false, "Never prompt for password")
	dbname     = flag.String("d", "", "Database name")
	dsnAlias   = flag.String("D", "", "DSN alias from config")
	singleConn = flag.Bool("single-connection", false, "Use single connection")
	configPath = flag.String("pgclirc", "", "Config file path")
	rowLimit   = flag.Int("row-limit", 0, "Row limit (0=from config)")
	appName    = flag.String("application-name", "gocli", "Application name")
	lessChatty = flag.Bool("less-chatty", false, "Skip intro/goodbye")
	prompt     = flag.String("prompt", "", "Prompt format")
	autoVert   = flag.Bool("auto-vertical-output", false, "Auto vertical for wide results")
	listDBs    = flag.Bool("l", false, "List databases and exit")
	listDSN    = flag.Bool("list-dsn", false, "List DSN aliases and exit")
	showVer    = flag.Bool("v", false, "Print version")
	sslMode    = flag.String("sslmode", "", "SSL mode")
	logFile    = flag.String("log-file", "", "Log queries and results to file")
	initCmd    = flag.String("init-command", "", "SQL to execute after connecting")
	execute    = flag.String("e", "", "Execute command and exit")
	pingOnly   = flag.Bool("ping", false, "Check connectivity and exit")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pgcli [OPTIONS] [DBNAME [USERNAME]]\n\n")
		fmt.Fprintf(os.Stderr, "A Go reimplementation of pgcli - PostgreSQL CLI with auto-completion.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("pgcli (gocli) %s\n", version)
		os.Exit(0)
	}

	// Load config
	cfg := config.DefaultPGConfig()
	cfgPath := *configPath
	if cfgPath == "" {
		if env := os.Getenv("PGCLIRC"); env != "" {
			cfgPath = env
		}
	}
	if err := cfg.Load(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %s\n", err)
	}

	// Apply CLI flags
	if *lessChatty {
		cfg.LessChatty = true
	}
	if *prompt != "" {
		cfg.Prompt = *prompt
	}
	if *autoVert {
		cfg.AutoExpand = true
	}
	if *rowLimit > 0 {
		cfg.RowLimit = *rowLimit
	}

	// List DSN aliases
	if *listDSN {
		if len(cfg.DSNAliases) == 0 {
			fmt.Println("No DSN aliases configured.")
		} else {
			for name, dsn := range cfg.DSNAliases {
				fmt.Printf("  %s = %s\n", name, dsn)
			}
		}
		os.Exit(0)
	}

	// Build connection config
	connCfg := buildPGConfig(cfg)

	// Handle .pgpass
	if connCfg.Password == "" && !*noPassword {
		pw := pg.ParsePgpass(connCfg.Host, connCfg.Port, connCfg.Database, connCfg.User)
		if pw != "" {
			connCfg.Password = pw
		}
	}

	// Prompt for password if -W flag
	if *password {
		fmt.Fprint(os.Stderr, "Password: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			connCfg.Password = scanner.Text()
		}
	}

	// Ping mode
	if *pingOnly {
		executor, err := pg.NewExecutor(connCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %s\n", err)
			os.Exit(1)
		}
		executor.Close()
		fmt.Println("Connection successful.")
		os.Exit(0)
	}

	// Connect
	executor, err := pg.NewExecutor(connCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection failed: %s\n", err)
		os.Exit(1)
	}
	defer executor.Close()

	// List databases mode
	if *listDBs {
		dbs, err := executor.Databases(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		for _, db := range dbs {
			fmt.Println(db)
		}
		os.Exit(0)
	}

	// Init command
	if *initCmd != "" {
		if _, err := executor.Execute(context.Background(), *initCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Init command error: %s\n", err)
		}
	}

	// Create app (used by both -e mode and interactive mode)
	app := cli.NewApp(cli.PostgreSQL, executor, executor, cfg)

	// Execute mode
	if *execute != "" {
		if app.ExecuteNonInteractive(*execute) {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if !cfg.LessChatty {
		ver, _ := executor.ServerVersion()
		fmt.Printf("gocli (pgcli) %s\n", version)
		if ver != "" {
			fmt.Printf("Server: PostgreSQL %s\n", ver)
		}
		fmt.Printf("Database: %s\n", executor.Database())
		fmt.Println("Type \\? for help.")
		fmt.Println()
	}

	// Refresh completions in background
	go app.RefreshCompletions()

	// Log file
	if *logFile != "" {
		lf, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open log file: %s\n", err)
		} else {
			defer lf.Close()
		}
	}

	// Run interactive loop
	runREPL(app, cfg)

	if !cfg.LessChatty {
		fmt.Println("Goodbye!")
	}
}

func runREPL(app *cli.App, cfg *config.Config) {
	// Detect if stdin is a terminal; fall back to basic REPL for piped input
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		runBasicREPL(app, cfg)
		return
	}

	completer := func(d goprompt.Document) []goprompt.Suggest {
		text := d.TextBeforeCursor()
		suggestions := app.Complete(text, len(text))
		s := make([]goprompt.Suggest, 0, len(suggestions))
		for _, sg := range suggestions {
			s = append(s, goprompt.Suggest{
				Text:        sg.Text,
				Description: sg.Description,
			})
		}
		return s
	}

	shouldQuit := false

	executor := func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			return
		}
		if app.HandleInput(input) {
			shouldQuit = true
		}
	}

	p := goprompt.New(
		executor,
		completer,
		goprompt.OptionPrefix(app.GetPrompt()),
		goprompt.OptionLivePrefix(func() (string, bool) {
			return app.GetPrompt(), true
		}),
		goprompt.OptionTitle("gocli"),
		goprompt.OptionPrefixTextColor(goprompt.Cyan),
		goprompt.OptionSuggestionBGColor(goprompt.DarkGray),
		goprompt.OptionSuggestionTextColor(goprompt.White),
		goprompt.OptionSelectedSuggestionBGColor(goprompt.Blue),
		goprompt.OptionSelectedSuggestionTextColor(goprompt.White),
		goprompt.OptionDescriptionBGColor(goprompt.DarkGray),
		goprompt.OptionDescriptionTextColor(goprompt.LightGray),
		goprompt.OptionSelectedDescriptionBGColor(goprompt.Blue),
		goprompt.OptionSelectedDescriptionTextColor(goprompt.White),
		goprompt.OptionMaxSuggestion(10),
		goprompt.OptionCompletionOnDown(),
		goprompt.OptionSetExitCheckerOnInput(func(in string, breakline bool) bool {
			return shouldQuit
		}),
	)

	p.Run()
}

func runBasicREPL(app *cli.App, cfg *config.Config) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		fmt.Fprint(os.Stdout, app.GetPrompt())

		if !scanner.Scan() {
			break
		}

		if shouldQuit := app.HandleInput(scanner.Text()); shouldQuit {
			break
		}
	}
}

func buildPGConfig(cfg *config.Config) pg.ConnectionConfig {
	connCfg := pg.DefaultConfig()

	// Environment variables
	if h := os.Getenv("PGHOST"); h != "" {
		connCfg.Host = h
	}
	if p := os.Getenv("PGPORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			connCfg.Port = n
		}
	}
	if u := os.Getenv("PGUSER"); u != "" {
		connCfg.User = u
	}
	if pw := os.Getenv("PGPASSWORD"); pw != "" {
		connCfg.Password = pw
	}
	if db := os.Getenv("PGDATABASE"); db != "" {
		connCfg.Database = db
	}

	// Positional arguments
	args := flag.Args()
	if len(args) >= 1 {
		// Could be a URI or database name
		if strings.HasPrefix(args[0], "postgres://") || strings.HasPrefix(args[0], "postgresql://") {
			if parsed, err := pg.ParseDSN(args[0]); err == nil {
				return parsed
			}
		}
		connCfg.Database = args[0]
	}
	if len(args) >= 2 {
		connCfg.User = args[1]
	}

	// DSN alias
	if *dsnAlias != "" {
		if dsn, ok := cfg.DSNAliases[*dsnAlias]; ok {
			if parsed, err := pg.ParseDSN(dsn); err == nil {
				return parsed
			}
		}
	}

	// CLI flags override
	if *host != "" {
		connCfg.Host = *host
	}
	if *port != 5432 {
		connCfg.Port = *port
	}
	if *username != "" {
		connCfg.User = *username
	}
	if *dbname != "" {
		connCfg.Database = *dbname
	}
	if *sslMode != "" {
		connCfg.SSLMode = *sslMode
	}

	// Default database to username
	if connCfg.Database == "" {
		connCfg.Database = connCfg.User
	}

	_ = singleConn
	_ = appName
	_ = time.Now // suppress unused import

	return connCfg
}
