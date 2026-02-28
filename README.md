# deploy-watch

A CLI tool that monitors deployment status across cloud providers and exits when the deployment reaches a terminal state. Designed for coding agents that need to wait for deployments to complete before proceeding.

## Install

```bash
go install github.com/tomblomfield/gocli/cmd/deploy-watch@latest
```

Or build from source:

```bash
git clone https://github.com/tomblomfield/gocli.git
cd gocli
go build -o deploy-watch ./cmd/deploy-watch/
```

## Supported Providers

| Provider | API | Auth Env Var | Project Env Var |
|----------|-----|-------------|-----------------|
| Railway | GraphQL (`backboard.railway.app`) | `RAILWAY_TOKEN` | `RAILWAY_PROJECT_ID` |
| Vercel | REST (`api.vercel.com`) | `VERCEL_TOKEN` | `VERCEL_PROJECT_ID` |
| Heroku | REST (`api.heroku.com`) | `HEROKU_API_KEY` | `HEROKU_APP` |
| Fly.io | Machines API (`api.machines.dev`) | `FLY_API_TOKEN` | `FLY_APP` |

## Usage

```
deploy-watch [flags] <provider> [deployment-id]
```

### Watch the latest deployment

```bash
# Railway
export RAILWAY_TOKEN=your-token
deploy-watch -project your-project-id railway

# Vercel
export VERCEL_TOKEN=your-token
deploy-watch vercel

# Heroku
export HEROKU_API_KEY=your-key
deploy-watch -project myapp heroku

# Fly.io
export FLY_API_TOKEN=your-token
deploy-watch -project myapp fly
```

### Watch a specific deployment

```bash
deploy-watch railway deploy-abc-123
deploy-watch vercel dpl_xyz789
deploy-watch heroku b9a1c3e4-5678-90ab-cdef
deploy-watch fly machine-12345
```

### JSON output for programmatic use

```bash
deploy-watch -json railway
```

Status updates stream to stderr as JSON:

```json
{"time":"2025-01-15T10:30:00Z","provider":"railway","status":"BUILDING","id":"deploy-abc12","message":"status=BUILDING id=deploy-abc12 elapsed=5s"}
{"time":"2025-01-15T10:31:00Z","provider":"railway","status":"SUCCEEDED","id":"deploy-abc12","message":"status=SUCCEEDED id=deploy-abc12 elapsed=1m5s url=https://myapp.up.railway.app"}
```

Final result prints to stdout:

```json
{
  "deployment": {
    "ID": "deploy-abc12",
    "Status": 4,
    "Provider": "railway",
    "Project": "my-service",
    "URL": "https://myapp.up.railway.app"
  },
  "duration": 65000000000,
  "polls": 13
}
```

### Railway-specific flags

```bash
deploy-watch \
  -project proj_abc123 \
  -service svc_def456 \
  -environment env_prod789 \
  railway
```

### Vercel team deployments

```bash
deploy-watch -team team_xyz vercel
```

### Custom poll interval and timeout

```bash
deploy-watch -interval 3s -timeout 10m railway
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-token` | | API token (overrides env var) |
| `-project` | | Project/app name or ID |
| `-service` | | Service ID (Railway only) |
| `-environment` | | Environment ID (Railway only) |
| `-team` | | Team ID (Vercel only) |
| `-interval` | `5s` | Poll interval |
| `-timeout` | `30m` | Max wait time |
| `-json` | `false` | Output as JSON |
| `-version` | | Print version |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Deployment succeeded |
| `1` | Deployment failed, crashed, or cancelled |
| `2` | Usage error or misconfiguration |
| `3` | Timeout |

Exit codes make it easy to use in scripts and agent workflows:

```bash
deploy-watch railway && echo "Deploy succeeded, running smoke tests..."
```

```bash
deploy-watch -timeout 5m vercel
case $? in
  0) echo "Success" ;;
  1) echo "Deploy failed" ;;
  3) echo "Timed out" ;;
esac
```

## Status Lifecycle

Each provider's native statuses are mapped to a common set:

```
PENDING -> BUILDING -> DEPLOYING -> SUCCEEDED
                                 -> FAILED
                                 -> CANCELLED
                                 -> CRASHED
```

The watcher polls until any terminal status is reached (`SUCCEEDED`, `FAILED`, `CANCELLED`, `CRASHED`), then exits with the appropriate code.

## Typical Agent Workflow

```bash
# 1. Trigger a deployment (provider-specific)
railway up

# 2. Wait for it to complete
deploy-watch -timeout 10m railway

# 3. Continue based on exit code
if [ $? -eq 0 ]; then
  echo "Running post-deploy checks..."
  curl -f https://myapp.up.railway.app/health
fi
```

## Architecture

```
cmd/deploy-watch/main.go       CLI entry point, flag parsing, signal handling
internal/deploy/
  provider.go                  Provider interface, Status type, Deployment struct
  watcher.go                   Core polling loop with timeout and cancellation
  railway.go                   Railway GraphQL API client
  vercel.go                    Vercel REST API client
  heroku.go                    Heroku REST API client
  fly.go                       Fly.io Machines API client
```

The `Provider` interface has two methods:

```go
type Provider interface {
    Name() string
    GetDeployment(ctx context.Context, id string) (*Deployment, error)
    LatestDeployment(ctx context.Context) (*Deployment, error)
}
```

Adding a new provider means implementing this interface and adding a case to the `newProvider` switch in `main.go`.

## License

MIT
