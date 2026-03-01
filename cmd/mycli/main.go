// mycli is a Go reimplementation of mycli - a MySQL CLI with auto-completion and syntax highlighting.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/tomblomfield/gocli/internal/cli"
	"github.com/tomblomfield/gocli/internal/config"
	"github.com/tomblomfield/gocli/internal/mysql"
)

var (
	version = "0.1.0"

	host       = flag.String("h", "", "Host address of the MySQL server")
	port       = flag.Int("P", 3306, "Port number")
	username   = flag.String("u", "", "Username")
	password   = flag.Bool("p", false, "Prompt for password")
	dbname     = flag.String("D", "", "Database name")
	dsnAlias   = flag.String("d", "", "DSN alias from config")
	socket     = flag.String("S", "", "Socket file path")
	configPath = flag.String("myclirc", "", "Config file path")
	lessChatty = flag.Bool("less-chatty", false, "Skip intro/goodbye")
	promptFmt  = flag.String("R", "", "Prompt format")
	autoVert   = flag.Bool("auto-vertical-output", false, "Auto vertical for wide results")
	listDSN    = flag.Bool("list-dsn", false, "List DSN aliases and exit")
	showVer    = flag.Bool("V", false, "Print version")
	execute    = flag.String("e", "", "Execute command and exit")
	tableOut   = flag.Bool("t", false, "Force table output")
	csvOut     = flag.Bool("csv", false, "Force CSV output")
	logFile    = flag.String("l", "", "Audit log file")
	initCmd    = flag.String("init-command", "", "SQL to execute after connecting")
	sslMode    = flag.String("ssl-mode", "auto", "SSL mode: auto, on, off")
	sslCa      = flag.String("ssl-ca", "", "CA file in PEM format")
	sslCert    = flag.String("ssl-cert", "", "Client X509 cert")
	sslKey     = flag.String("ssl-key", "", "Client X509 key")
	charset    = flag.String("charset", "", "Character set")
	warn       = flag.Bool("warn", true, "Warn before destructive commands")
	verbose    = flag.Bool("v", false, "Verbose output")
	loginPath  = flag.String("g", "", "MySQL login path")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mycli [OPTIONS] [DATABASE]\n\n")
		fmt.Fprintf(os.Stderr, "A Go reimplementation of mycli - MySQL CLI with auto-completion.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("mycli (gocli) %s\n", version)
		os.Exit(0)
	}

	// Load config
	cfg := config.DefaultMySQLConfig()
	cfgPath := *configPath
	if err := cfg.Load(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %s\n", err)
	}

	// Apply CLI flags
	if *lessChatty {
		cfg.LessChatty = true
	}
	if *promptFmt != "" {
		cfg.Prompt = *promptFmt
	}
	if *autoVert {
		cfg.AutoExpand = true
	}
	if !*warn {
		cfg.DestructiveWarning = false
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
	connCfg := buildMySQLConfig(cfg)

	// Prompt for password if -p flag
	if *password {
		fmt.Fprint(os.Stderr, "Password: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			connCfg.Password = scanner.Text()
		}
	}

	// Connect
	executor, err := mysql.NewExecutor(connCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection failed: %s\n", err)
		os.Exit(1)
	}
	defer executor.Close()

	// Init command
	if *initCmd != "" {
		if _, err := executor.Execute(context.Background(), *initCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Init command error: %s\n", err)
		}
	}

	// Create app (used by both -e mode and interactive mode)
	app := cli.NewApp(cli.MySQL, executor, executor, cfg)

	// Execute mode
	if *execute != "" {
		if app.ExecuteNonInteractive(*execute) {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if !cfg.LessChatty {
		ver, _ := executor.ServerVersion()
		fmt.Printf("gocli (mycli) %s\n", version)
		if ver != "" {
			fmt.Printf("Server: MySQL %s\n", ver)
		}
		fmt.Printf("Database: %s\n", executor.Database())
		fmt.Println("Type \\? for help.")
		fmt.Println()
	}

	// Refresh completions in background
	go app.RefreshCompletions()

	// Run interactive loop
	runREPL(app, cfg)

	if !cfg.LessChatty {
		fmt.Println("Goodbye!")
	}

	_ = logFile
	_ = tableOut
	_ = csvOut
	_ = verbose
	_ = loginPath
}

func runREPL(app *cli.App, cfg *config.Config) {
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		runBasicREPL(app, cfg)
		return
	}

	quit := false

	executor := func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			return
		}
		if app.HandleInput(input) {
			quit = true
		}
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
		goprompt.OptionAddKeyBind(goprompt.KeyBind{
			Key: goprompt.ControlC,
			Fn:  func(*goprompt.Buffer) { quit = true },
		}),
	)

	for !quit {
		p.Run()
		break
	}
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

func buildMySQLConfig(cfg *config.Config) mysql.ConnectionConfig {
	// Start with my.cnf defaults
	connCfg := mysql.ParseMyCnf()

	// Environment variables
	if h := os.Getenv("MYSQL_HOST"); h != "" {
		connCfg.Host = h
	}
	if p := os.Getenv("MYSQL_TCP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			connCfg.Port = n
		}
	}
	if s := os.Getenv("MYSQL_UNIX_PORT"); s != "" {
		connCfg.Socket = s
	}
	if pw := os.Getenv("MYSQL_PWD"); pw != "" {
		connCfg.Password = pw
	}

	// Positional arguments
	args := flag.Args()
	if len(args) >= 1 {
		if strings.HasPrefix(args[0], "mysql://") {
			if parsed, err := mysql.ParseDSN(args[0]); err == nil {
				return parsed
			}
		}
		connCfg.Database = args[0]
	}

	// DSN alias
	if *dsnAlias != "" {
		if dsn, ok := cfg.DSNAliases[*dsnAlias]; ok {
			if parsed, err := mysql.ParseDSN(dsn); err == nil {
				return parsed
			}
		}
	}

	// CLI flags override
	if *host != "" {
		connCfg.Host = *host
	}
	if *port != 3306 {
		connCfg.Port = *port
	}
	if *username != "" {
		connCfg.User = *username
	}
	if *dbname != "" {
		connCfg.Database = *dbname
	}
	if *socket != "" {
		connCfg.Socket = *socket
	}
	if *charset != "" {
		connCfg.Charset = *charset
	}

	// SSL
	if *sslMode == "on" {
		connCfg.SSL = true
	}
	if *sslCa != "" {
		connCfg.SSLCa = *sslCa
	}
	if *sslCert != "" {
		connCfg.SSLCert = *sslCert
	}
	if *sslKey != "" {
		connCfg.SSLKey = *sslKey
	}

	return connCfg
}
