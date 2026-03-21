# e9s

An interactive terminal UI for managing AWS infrastructure — ECS, CloudWatch Logs, SSM Parameter Store, Secrets Manager, S3, and Lambda — from a single tool.

Inspired by [k9s](https://k9scli.io/) for Kubernetes. Built in Go with [bubbletea](https://github.com/charmbracelet/bubbletea) and [lipgloss](https://github.com/charmbracelet/lipgloss). Colors adapt to your terminal's color scheme.

```
e9s [1:ECS] [2:CW] [3:SSM] [4:SM] [5:S3] [6:λ] ── sample-prod ── region: us-east-1 ── ↻ 2s ago

  Services (4)

  ╭─────────────────────────┬────────┬─────────┬─────────┬─────────┬────────────────────────────┬─────╮
  │ NAME                    │ STATUS │ DESIRED │ RUNNING │ PENDING │ TASK DEF                   │ AGE │
  ├─────────────────────────┼────────┼─────────┼─────────┼─────────┼────────────────────────────┼─────┤
► │ backend                 │ ACTIVE │       5 │       5 │       0 │ backend:42                 │ 14d │
  │ upload                  │ ACTIVE │       3 │       3 │       0 │ upload:41                  │ 14d │
  │ api                     │ ACTIVE │       2 │       2 │       0 │ api:40                     │ 14d │
  │ ui                      │ ACTIVE │       2 │       2 │       0 │ ui:15                      │  7d │
  ╰─────────────────────────┴────────┴─────────┴─────────┴─────────┴────────────────────────────┴─────╯

  [enter] tasks  [d] detail  [L] logs  [r] redeploy  [s] scale  [m] metrics  [S] standalone
  [/] filter  [ctrl+r] region  [esc] back  [q] quit  [?] help
```

## Modules

e9s is organized into modules, each accessible as a numbered tab in the status bar. Press `1`–`6` to switch between them. Modules can be individually enabled or disabled in the config file.

### ECS (tab 1)

Navigate clusters → services → tasks → containers. Force new deployments, scale services, stop tasks, view deployment rollout progress, inspect service events, and monitor CPU/memory metrics with CloudWatch alarm status.

- **ECS Exec** — shell into running containers via Session Manager Plugin with command prompt
- **Environment Variables** — view resolved env vars including SSM/Secrets Manager references with source labels and ARN toggle
- **Task Definition Diff** — compare task definitions between deployments
- **Standalone Tasks** — browse non-service tasks (Lambda-launched workers, one-off jobs)
- **Log Streaming** — tail CloudWatch logs per task, container, or entire service

### CloudWatch Logs (tab 2)

Browse log groups (by prefix or substring search), drill into log streams, and interact with logs.

- **Live Tail** — stream logs at the group or stream level with follow mode, search highlighting, and `n`/`N` match navigation
- **Log Search** — search across a time range using CloudWatch filter syntax, then jump from a result directly into the full log context
- **Backward/Forward Fetch** — press `[`/`]` to load older or newer log chunks without enabling follow
- **Timestamp Modes** — cycle through relative, local, and UTC timestamps with `t`
- **Save to File** — export the current log buffer with `w`
- **Save/Delete Log Paths** — bookmark frequently used log groups and streams

### SSM Parameter Store (tab 3)

Browse SSM parameters by path prefix. View values (with decryption for SecureString), edit parameters with confirmation, and save prefixes for quick access.

### Secrets Manager (tab 4)

Browse secrets by name filter. View secret values as pretty-printed JSON with syntax coloring, inspect tags, and edit values with confirmation. Saved filters for quick access.

### S3 (tab 5)

Browse S3 buckets (search by name), navigate object keys as a file browser with folder-level navigation. View object metadata and tags, download individual objects or recursively download entire prefixes.

### Lambda (tab 6)

Browse Lambda functions with runtime, state, memory, and timeout info. View environment variables with SSM/Secrets Manager resolution (same as ECS), tail CloudWatch logs, and search logs with the full CW search flow.

## Installation

### Prerequisites

- **Go 1.24+** — [install](https://go.dev/doc/install)
- **AWS credentials** configured via any standard method (`~/.aws/credentials`, environment variables, IAM role, SSO)
- **session-manager-plugin** (required only for ECS Exec)

### Build from source

```bash
git clone https://github.com/dostrow/e9s.git
cd e9s
go build -o e9s .
```

Or install directly to your `$GOPATH/bin`:

```bash
go install github.com/dostrow/e9s@latest
```

### Cross-compile

```bash
GOOS=linux GOARCH=amd64 go build -o e9s-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o e9s-linux-arm64 .
GOOS=darwin GOARCH=arm64 go build -o e9s-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o e9s.exe .
```

## Dependencies

### Session Manager Plugin (for ECS Exec)

The `e` keybinding (shell into container) requires the AWS Session Manager Plugin binary in your `PATH`.

**Ubuntu/Debian:**

```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o session-manager-plugin.deb
sudo dpkg -i session-manager-plugin.deb
```

**macOS (Homebrew):**

```bash
brew install --cask session-manager-plugin
```

**Amazon Linux / RPM:**

```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/64bit/session-manager-plugin.rpm" -o session-manager-plugin.rpm
sudo yum install -y session-manager-plugin.rpm
```

**Windows:**

Download and run the installer from:

```
https://s3.amazonaws.com/session-manager-downloads/plugin/latest/windows/SessionManagerPluginSetup.exe
```

Verify: `session-manager-plugin --version`

> **Note:** ECS Exec also requires `enableExecuteCommand: true` on the ECS service and appropriate SSM permissions on the task IAM role. See the [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html) for setup details.

## Usage

```
e9s [flags]

Flags:
  -c, --cluster string   Default cluster name (skips cluster selection)
  -r, --region string    AWS region (default: from AWS config)
  -p, --profile string   AWS profile name
      --refresh int      Refresh interval in seconds (default: 5)
  -h, --help             Help
  -v, --version          Version
```

### Examples

```bash
# Interactive — start at cluster list
e9s

# Jump directly to a cluster's services
e9s -c my-cluster

# Use a specific AWS profile and region
e9s -p production -r us-east-2 -c my-cluster
```

## Key Bindings

### Global

| Key | Action |
|-----|--------|
| `1`–`6` | Switch module (ECS, CW, SSM, SM, S3, λ) |
| `j`/`k` or `↑`/`↓` | Navigate up/down |
| `Enter` | Drill into selected item |
| `Esc` | Go back to parent view |
| `q` | Quit from anywhere |
| `/` | Filter/search |
| `R` | Refresh data |
| `Ctrl+R` | Switch AWS region |
| `?` | Help overlay |

### ECS — Service List

| Key | Action |
|-----|--------|
| `r` | Force new deployment (with confirmation) |
| `s` | Scale — prompt for desired count |
| `d` | Service detail (deployments + events) |
| `L` | Tail logs for entire service |
| `m` | CPU/memory metrics + alarms |
| `S` | Standalone tasks (non-service) |

### ECS — Task List

| Key | Action |
|-----|--------|
| `l` | Tail logs for selected task |
| `x` | Stop task (with confirmation) |
| `e` | ECS Exec (shell into container) |

### ECS — Task/Lambda Detail

| Key | Action |
|-----|--------|
| `E` | View environment variables (with SSM/SM resolution) |

### Log Viewer

| Key | Action |
|-----|--------|
| `f` | Toggle follow mode (auto-scroll) |
| `t` | Cycle timestamps: relative → local → UTC |
| `/` | Search — jump to matches with `n`/`N` |
| `[`/`]` | Load older/newer log chunks |
| `w` | Save buffer to file |
| `g`/`G` | Jump to top/bottom |
| `PgUp`/`PgDn` | Scroll by page |

### CloudWatch — Log Groups/Streams

| Key | Action |
|-----|--------|
| `l` | Tail selected stream/group |
| `L` | Tail entire log group |
| `s` | Search logs (time range + pattern) |
| `W` | Save log path |

### SSM / Secrets Manager

| Key | Action |
|-----|--------|
| `Enter` | View value (SM shows pretty-printed JSON with tags) |
| `e` | Edit value (with confirmation) |
| `W` | Save prefix/filter |

### S3

| Key | Action |
|-----|--------|
| `Enter` | Browse into folder / view object detail |
| `i` | View object detail + tags |
| `D` | Download object or folder |
| `W` | Save bucket search |

### Lambda

| Key | Action |
|-----|--------|
| `Enter` | View function detail |
| `l` | Tail function logs |
| `s` | Search function logs |
| `E` | View environment variables (from detail) |
| `W` | Save search |

### All Pickers (saved items)

| Key | Action |
|-----|--------|
| `d` | Delete selected saved entry |

## Configuration

Create `~/.e9s.yaml` to set defaults:

```yaml
defaults:
  cluster: my-cluster
  region: us-east-2
  profile: ""
  refresh_interval: 5

display:
  timestamp_format: relative  # "relative" or "absolute"
  max_events: 50
  max_log_lines: 1000

# Enable/disable modules (all enabled by default)
modules:
  ecs: true
  cloudwatch: true
  ssm: true
  sm: true
  s3: true
  lambda: true

# Saved bookmarks (managed via W/d keys in the TUI)
ssm_prefixes: []
sm_filters: []
s3_searches: []
lambda_searches: []
log_paths: []

exclude_services: []
```

CLI flags override config file values.

## AWS Permissions

Your IAM identity needs permissions for whichever features you use:

| Feature | API Calls |
|---------|-----------|
| ECS browse | `ecs:ListClusters`, `ecs:DescribeClusters`, `ecs:ListServices`, `ecs:DescribeServices`, `ecs:ListTasks`, `ecs:DescribeTasks` |
| ECS operations | `ecs:UpdateService`, `ecs:StopTask` |
| ECS Exec | `ecs:ExecuteCommand`, `ssmmessages:*` |
| Task definitions | `ecs:DescribeTaskDefinition` |
| CloudWatch Logs | `logs:DescribeLogGroups`, `logs:DescribeLogStreams`, `logs:FilterLogEvents` |
| CloudWatch Metrics | `cloudwatch:GetMetricData`, `cloudwatch:DescribeAlarms` |
| SSM parameters | `ssm:GetParametersByPath`, `ssm:GetParameter`, `ssm:GetParameters`, `ssm:PutParameter` |
| Secrets Manager | `secretsmanager:ListSecrets`, `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue` |
| S3 | `s3:ListBuckets`, `s3:ListObjectsV2`, `s3:HeadObject`, `s3:GetObject`, `s3:GetObjectTagging` |
| Lambda | `lambda:ListFunctions`, `lambda:GetFunction` |

## Project Structure

```
e9s/
├── main.go
├── Makefile
├── internal/
│   ├── aws/
│   │   ├── client.go          # AWS SDK client + region switching
│   │   ├── ecs.go             # ECS CRUD operations
│   │   ├── exec.go            # ECS Exec + session-manager-plugin
│   │   ├── logs.go            # CloudWatch log streaming + search
│   │   ├── metrics.go         # CloudWatch metrics + alarms
│   │   ├── ssm.go             # SSM Parameter Store
│   │   ├── secrets.go         # Secrets Manager
│   │   ├── s3.go              # S3 bucket/object operations
│   │   ├── lambda.go          # Lambda function listing
│   │   └── taskdef.go         # Task definition fetch + diff + env var resolution
│   ├── config/
│   │   └── config.go          # ~/.e9s.yaml loader + saved bookmarks
│   ├── model/
│   │   ├── types.go           # ECS data types
│   │   └── transform.go       # AWS SDK types → internal types
│   └── ui/
│       ├── app.go             # Core: types, Update routing, View, navigation
│       ├── app_ecs.go         # ECS operations, exec, env vars, data loading
│       ├── app_cloudwatch.go  # CloudWatch log browser + search
│       ├── app_ssm.go         # SSM parameter operations
│       ├── app_sm.go          # Secrets Manager operations
│       ├── app_s3.go          # S3 browser + download
│       ├── app_lambda.go      # Lambda browser + log integration
│       ├── app_messages.go    # All message type definitions
│       ├── app_shared.go      # Region switching, saved entry deletion
│       ├── confirm.go         # Confirmation dialog
│       ├── help.go            # Help overlay
│       ├── input.go           # Text input dialog
│       ├── picker.go          # List picker with delete support
│       ├── execwrap.go        # Exec subprocess wrapper
│       ├── statusbar.go       # Status bar with mode tabs
│       ├── theme/
│       │   ├── styles.go      # ANSI-adaptive colors + styles
│       │   └── keys.go        # Key bindings
│       ├── components/
│       │   └── table.go       # Auto-sizing nushell-style grid table
│       └── views/
│           ├── clusters.go
│           ├── services.go
│           ├── service_detail.go
│           ├── tasks.go
│           ├── standalone_tasks.go
│           ├── task_detail.go
│           ├── taskdef_diff.go
│           ├── logs.go
│           ├── log_groups.go
│           ├── log_streams.go
│           ├── log_search.go
│           ├── metrics.go
│           ├── env_vars.go
│           ├── ssm.go
│           ├── secrets.go
│           ├── secret_value.go
│           ├── s3_buckets.go
│           ├── s3_objects.go
│           ├── s3_detail.go
│           ├── lambda_list.go
│           ├── lambda_detail.go
│           └── region_picker.go
```

## License

MIT
