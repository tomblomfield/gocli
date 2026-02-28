// deploy-watch monitors deployment status across cloud providers and exits
// when the deployment reaches a terminal state (success or failure).
//
// Usage:
//
//	deploy-watch [flags] <provider> [deployment-id]
//
// Providers: railway, vercel, heroku, fly
//
// Exit codes:
//
//	0 - deployment succeeded
//	1 - deployment failed, crashed, or cancelled
//	2 - usage error or misconfiguration
//	3 - timeout
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tomblomfield/gocli/internal/deploy"
)

var version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	// Flags
	token := flag.String("token", "", "API token (overrides env var)")
	project := flag.String("project", "", "Project/app name or ID")
	service := flag.String("service", "", "Service ID (Railway)")
	environment := flag.String("environment", "", "Environment ID (Railway)")
	team := flag.String("team", "", "Team ID (Vercel)")
	interval := flag.Duration("interval", 5*time.Second, "Poll interval")
	timeout := flag.Duration("timeout", 30*time.Minute, "Max wait time")
	jsonOutput := flag.Bool("json", false, "Output as JSON")
	showVer := flag.Bool("version", false, "Print version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `deploy-watch - Deployment status monitor for coding agents

Usage:
  deploy-watch [flags] <provider> [deployment-id]

Providers:
  railway    Railway deployments (env: RAILWAY_TOKEN)
  vercel     Vercel deployments (env: VERCEL_TOKEN)
  heroku     Heroku builds (env: HEROKU_API_KEY)
  fly        Fly.io machines (env: FLY_API_TOKEN)

Examples:
  deploy-watch railway                     Watch latest Railway deployment
  deploy-watch -project myapp vercel       Watch latest Vercel deployment
  deploy-watch heroku abc-123-def          Watch specific Heroku build
  deploy-watch -json fly                   Output status as JSON

Environment Variables:
  RAILWAY_TOKEN   Railway API token
  VERCEL_TOKEN    Vercel API token
  HEROKU_API_KEY  Heroku API key
  FLY_API_TOKEN   Fly.io API token

Exit Codes:
  0  Deployment succeeded
  1  Deployment failed, crashed, or cancelled
  2  Usage error or misconfiguration
  3  Timeout

Flags:
`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVer {
		fmt.Printf("deploy-watch %s\n", version)
		return 0
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: provider argument required (railway, vercel, heroku, fly)")
		fmt.Fprintln(os.Stderr, "Run 'deploy-watch -help' for usage.")
		return 2
	}

	providerName := strings.ToLower(args[0])
	deploymentID := ""
	if len(args) >= 2 {
		deploymentID = args[1]
	}

	// Resolve token from flag or environment
	apiToken := *token
	if apiToken == "" {
		apiToken = tokenFromEnv(providerName)
	}

	cfg := deploy.ProviderConfig{
		Token:       apiToken,
		Project:     *project,
		Service:     *service,
		Environment: *environment,
		Team:        *team,
	}

	// Also check provider-specific env vars for project
	if cfg.Project == "" {
		cfg.Project = projectFromEnv(providerName)
	}

	provider, err := newProvider(providerName, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 2
	}

	// Setup context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	watchCfg := deploy.WatchConfig{
		Interval:     *interval,
		Timeout:      *timeout,
		DeploymentID: deploymentID,
		JSONOutput:   *jsonOutput,
		Writer:       os.Stderr,
	}

	if !*jsonOutput {
		fmt.Fprintf(os.Stderr, "Watching %s deployment", providerName)
		if deploymentID != "" {
			fmt.Fprintf(os.Stderr, " %s", deploymentID)
		}
		if cfg.Project != "" {
			fmt.Fprintf(os.Stderr, " (project: %s)", cfg.Project)
		}
		fmt.Fprintf(os.Stderr, "...\n")
	}

	result, err := deploy.Watch(ctx, provider, watchCfg)
	if err != nil {
		if *jsonOutput {
			writeJSON(os.Stdout, map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		if strings.Contains(err.Error(), "timed out") {
			return 3
		}
		return 1
	}

	// Output final result
	if *jsonOutput {
		writeJSON(os.Stdout, result)
	} else {
		printResult(result)
	}

	if result.Deployment.Status.Success() {
		return 0
	}
	return 1
}

func newProvider(name string, cfg deploy.ProviderConfig) (deploy.Provider, error) {
	switch name {
	case "railway":
		return deploy.NewRailway(cfg)
	case "vercel":
		return deploy.NewVercel(cfg)
	case "heroku":
		return deploy.NewHeroku(cfg)
	case "fly":
		return deploy.NewFly(cfg)
	default:
		return nil, fmt.Errorf("unknown provider %q (supported: railway, vercel, heroku, fly)", name)
	}
}

func tokenFromEnv(provider string) string {
	switch provider {
	case "railway":
		return os.Getenv("RAILWAY_TOKEN")
	case "vercel":
		return os.Getenv("VERCEL_TOKEN")
	case "heroku":
		return os.Getenv("HEROKU_API_KEY")
	case "fly":
		return os.Getenv("FLY_API_TOKEN")
	default:
		return ""
	}
}

func projectFromEnv(provider string) string {
	switch provider {
	case "railway":
		return os.Getenv("RAILWAY_PROJECT_ID")
	case "vercel":
		return os.Getenv("VERCEL_PROJECT_ID")
	case "heroku":
		return os.Getenv("HEROKU_APP")
	case "fly":
		return os.Getenv("FLY_APP")
	default:
		return ""
	}
}

func printResult(r *deploy.WatchResult) {
	d := r.Deployment
	fmt.Fprintf(os.Stdout, "\nDeployment %s\n", d.Status)
	fmt.Fprintf(os.Stdout, "  Provider:    %s\n", d.Provider)
	fmt.Fprintf(os.Stdout, "  ID:          %s\n", d.ID)
	if d.Project != "" {
		fmt.Fprintf(os.Stdout, "  Project:     %s\n", d.Project)
	}
	if d.Environment != "" {
		fmt.Fprintf(os.Stdout, "  Environment: %s\n", d.Environment)
	}
	if d.URL != "" {
		fmt.Fprintf(os.Stdout, "  URL:         %s\n", d.URL)
	}
	if d.CommitSHA != "" {
		sha := d.CommitSHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Fprintf(os.Stdout, "  Commit:      %s\n", sha)
	}
	if d.CommitMsg != "" {
		msg := d.CommitMsg
		if len(msg) > 60 {
			msg = msg[:60] + "..."
		}
		fmt.Fprintf(os.Stdout, "  Message:     %s\n", msg)
	}
	fmt.Fprintf(os.Stdout, "  Duration:    %s\n", r.Duration.Truncate(time.Second))
	fmt.Fprintf(os.Stdout, "  Polls:       %d\n", r.Polls)
}

func writeJSON(w *os.File, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
