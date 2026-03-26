# e9s - ElasticMS The Elastic Management System

An interactive terminal UI for managing AWS infrastructure from a single tool. Browse and operate on ECS, EC2, ECR, CloudWatch Logs, CloudWatch Alarms, SSM Parameter Store, Secrets Manager, S3, Lambda, DynamoDB, SQS, CodeBuild, and Route53.

Inspired by [k9s](https://k9scli.io/) for Kubernetes. Built in Go with [bubbletea](https://github.com/charmbracelet/bubbletea) and [lipgloss](https://github.com/charmbracelet/lipgloss). Colors adapt to your terminal's color scheme via ANSI color indices.

## Features

- **Full-screen framed UI** with bordered layout, scrollbar, info bar, and mode label
- **Mode switcher** — press `` ` `` to pick a module, or `ctrl+p` to reopen the current mode's entry picker
- **Context-sensitive help** — press `?` for a full keybinding overlay
- **Region switching** — `ctrl+r` to change AWS region on the fly
- **Config hot-reload** — edit `~/.config/e9s/config.yaml` (or `ctrl+e` to open in `$EDITOR`) and changes apply automatically
- **Saved bookmarks** — save frequently used paths, filters, queries, and multi-group selections with `W`
- **Terminal-adaptive colors** — uses ANSI indices 0–15 so colors match your terminal theme

## Modules

e9s is organized into modules accessible via the mode switcher (`` ` ``). Modules can be individually enabled or disabled in the config file.

### ECS

Navigate clusters → services → tasks → containers. Force new deployments, scale services, stop tasks, view deployment rollout progress, inspect service events, and monitor CPU/memory metrics with CloudWatch alarm status.

- **ECS Exec** — shell into running containers via Session Manager Plugin
- **Environment Variables** — view resolved env vars including SSM/Secrets Manager references with source labels and ARN toggle
- **Task Definition Diff** — compare task definitions between deployments
- **Standalone Tasks** — browse non-service tasks (Lambda-launched workers, one-off jobs)
- **Log Streaming** — tail CloudWatch logs per task, container, or entire service

### EC2 (EC2i)

Browse EC2 instances with name, state (color-coded), type, availability zone, private IP, and age. Filter by name, instance ID, IP, type, or state. Running instances sort first.

- **Instance Detail** — full metadata, networking (private/public IP, VPC, subnet), IAM role, AMI, key name, architecture
- **Security Groups** — inbound and outbound rules table with protocol, ports, and source/destination
- **EBS Volumes** — attached volumes with size, type, and state
- **Tags** — all instance tags
- **SSM Session** — shell into a running instance via Session Manager Plugin (`e`)
- **Console Output** — view the instance's serial console log for debugging boot issues (`c`)
- **State Management** — start (`S`), stop (`X`), reboot (`r`), and terminate (`T`) with confirmation

### CloudWatch Logs (CWL)

Browse log groups (by prefix or substring search), drill into log streams, and interact with logs.

- **Live Tail** — stream logs at the group or stream level with follow mode, search highlighting, and `n`/`N` match navigation
- **Log Search** — search across a time range (relative presets or custom UTC timestamps) using CloudWatch filter syntax; plain text is auto-quoted for literal matching
- **Multi-Group Search** — select multiple log groups with `space`, search across all of them, and save the selection for future use
- **Backward/Forward Fetch** — press `[`/`]` to load older or newer log chunks
- **Timestamp Modes** — cycle through relative, local, and UTC timestamps with `t`
- **Copy/Edit** — copy log buffer to clipboard (`y`) or open in `$EDITOR` (`o`)
- **Save to File** — export the current log buffer with `w`
- **Save/Delete Log Paths** — bookmark frequently used log groups, streams, and multi-group selections

### CloudWatch Alarms (CWA)

Browse all CloudWatch metric alarms with color-coded state (OK, ALARM, INSUFFICIENT_DATA). Filter by state on entry, filter by name/metric/namespace in the list.

- **Alarm Detail** — view configuration, dimensions, actions, and recent history
- **Toggle Actions** — enable or disable alarm actions with `a`
- **Set State** — manually set alarm state for testing with `S`
- **Timestamps** — toggle between local and UTC with `t`

### SSM Parameter Store

Browse SSM parameters by path prefix. View values (with decryption for SecureString), edit parameters with confirmation, and save prefixes for quick access.

### Secrets Manager

Browse secrets by name filter (true substring matching). View secret values as pretty-printed JSON with syntax coloring, inspect tags, and edit values with confirmation. Save filters for quick access.

### S3

Browse S3 buckets (search by name), navigate object keys as a file browser with folder-level navigation. View object metadata and tags, download individual objects or recursively download entire prefixes. Configurable default save directory.

### Lambda

Browse Lambda functions with runtime, state, memory, and timeout info. View environment variables with SSM/Secrets Manager resolution (same as ECS), tail CloudWatch logs, and search logs with the full CW Logs search flow.

### DynamoDB

Browse DynamoDB tables, scan items with pagination (press `]` to load more), and drill into item details.

- **Filter Scan** — filter by attribute with operators (=, <>, contains, begins_with, etc.)
- **PartiQL** — run arbitrary PartiQL queries with saved query support
- **Edit Fields** — edit individual field values via `$EDITOR` with type inference
- **Clone Items** — clone and modify items via `$EDITOR` for creating new entries

### SQS

Browse SQS queues (substring search), view queue stats and configuration, poll for messages, and inspect message details.

- **Send Messages** — compose messages via `$EDITOR` with FIFO support (group ID, deduplication ID)
- **Clone & Send** — clone an existing message for re-sending
- **DLQ Navigation** — jump to the dead letter queue from a queue's detail view
- **Delete Messages** — delete individual messages with confirmation

### CodeBuild

Browse CodeBuild projects, view build history per project, and inspect build details.

- **Build Detail** — view phases with durations and error contexts, source info, environment variables, and log location
- **View Logs** — open the build's CloudWatch log stream in the log viewer (full history for completed builds, follow mode for in-progress)
- **Search Logs** — server-side search across the full build log with `s`
- **Start Build** — trigger a new build with `b` (with confirmation)
- **Stop Build** — stop an in-progress build with `x`

### ECR

Browse ECR repositories, view images with vulnerability scan summaries, and drill into scan findings.

- **Repository List** — name, scan-on-push status, tag mutability, encryption type. Filterable.
- **Image List** — tags, digest, push date, size, scan status, vulnerability summary (C:2 H:5 M:12 format, color-coded). Newest first.
- **Scan Findings** — severity (color-coded CRITICAL/HIGH/MEDIUM/LOW), CVE ID, package, version, description. Sorted by severity.
- **Start Scan** — trigger on-demand image scan with `s`
- **Copy URI** — copy full image URI to clipboard with `y`
- **Delete Image** — remove by digest with `x` (with confirmation)

### Route53

Browse hosted zones, view and manage DNS record sets, and test DNS resolution.

- **Hosted Zones** — zone name, public/private type, record count, comment. Filterable.
- **Record Sets** — name, type (color-coded), TTL, values (with alias target display), routing policy. Filterable.
- **Record Detail** — full values, alias info, routing policy details (weighted, latency, failover, geolocation), health check ID
- **Test DNS** — resolve a record via the Route53 `TestDNSAnswer` API with `t` — shows response code, nameserver, and resolved data
- **Create/Edit/Delete Records** — create (`n`) or edit (`e`) records via `$EDITOR` with JSON templates, delete (`x`) with confirmation

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
  -c, --cluster string   ECS cluster (skips to service list)
  -m, --mode string      Start in module: ECS, EC2i, ECR, CWL, CWA, SSM, SM, S3, Lambda, DDB, SQS, CB, R53
  -r, --region string    AWS region (default: from AWS config)
  -p, --profile string   AWS profile name
      --refresh int      Auto-refresh interval in seconds (default: 5)
  -h, --help             Help
  -v, --version          Version
```

### Examples

```bash
# Interactive — start at mode picker
e9s

# Jump straight into DynamoDB
e9s -m DDB

# Jump directly to an ECS cluster's services
e9s -c my-cluster

# Use a specific AWS profile and region
e9s -p production -r us-east-2

# Start in SQS mode in a specific region
e9s -m SQS -r eu-west-1
```

## Key Bindings

### Global

| Key | Action |
| --- | --- |
| `` ` `` | Open mode switcher |
| `ctrl+p` | Reopen current mode's entry picker |
| `ctrl+e` | Edit config in `$EDITOR` |
| `ctrl+r` | Switch AWS region |
| `j`/`k` or `↑`/`↓` | Navigate up/down |
| `Enter` | Drill into selected item |
| `Esc` | Go back to parent view |
| `q` | Quit |
| `/` | Filter/search |
| `R` | Refresh data |
| `?` | Context-sensitive help overlay |
| `PgUp`/`PgDn` | Scroll by page |

### ECS — Service List

| Key | Action |
| --- | --- |
| `r` | Force new deployment (with confirmation) |
| `s` | Scale — prompt for desired count |
| `d` | Service detail (deployments + events) |
| `L` | Tail logs for entire service |
| `m` | CPU/memory metrics + alarms |
| `S` | Standalone tasks (non-service) |

### ECS — Task List

| Key | Action |
| --- | --- |
| `l` | Tail logs for selected task |
| `x` | Stop task (with confirmation) |
| `e` | ECS Exec (shell into container) |

### ECS — Task/Lambda Detail

| Key | Action |
| --- | --- |
| `E` | View environment variables (with SSM/SM resolution) |

### Log Viewer

| Key | Action |
| --- | --- |
| `f` | Toggle follow mode (auto-scroll) |
| `t` | Cycle timestamps: relative → local → UTC |
| `/` | Search — jump to matches with `n`/`N` |
| `[`/`]` | Load older/newer log chunks |
| `y` | Copy log buffer to clipboard |
| `o` | Open log buffer in `$EDITOR` |
| `w` | Save buffer to file |
| `g`/`G` | Jump to top/bottom |

### CloudWatch Logs — Log Groups

| Key | Action |
| --- | --- |
| `space` | Multi-select groups for search |
| `l` | Tail selected group |
| `s` | Search selected groups |
| `W` | Save log path / group selection |

### CloudWatch Logs — Log Streams

| Key | Action |
| --- | --- |
| `l` | Tail selected stream |
| `L` | Tail entire log group |
| `s` | Search stream |
| `W` | Save log path |

### CloudWatch Alarms

| Key | Action |
| --- | --- |
| `t` | Toggle local/UTC timestamps |
| `/` | Filter alarms |

### CloudWatch Alarms — Detail

| Key | Action |
| --- | --- |
| `a` | Enable/disable alarm actions |
| `S` | Set alarm state (for testing) |

### SSM / Secrets Manager

| Key | Action |
| --- | --- |
| `Enter` | View value (SM shows pretty-printed JSON with tags) |
| `e` | Edit value (with confirmation) |
| `W` | Save prefix/filter |

### S3

| Key | Action |
| --- | --- |
| `Enter` | Browse into folder / view object detail |
| `D` | Download object or folder |
| `W` | Save bucket search |

### Lambda

| Key | Action |
| --- | --- |
| `Enter` | View function detail |
| `l` | Tail function logs |
| `s` | Search function logs |
| `E` | View environment variables (from detail) |
| `W` | Save search |

### DynamoDB — Items

| Key | Action |
| --- | --- |
| `Enter` | View item detail |
| `f` | Filter scan (attribute + operator + value) |
| `p` | PartiQL query |
| `]` | Load next page |
| `W` | Save PartiQL query |

### DynamoDB — Item Detail

| Key | Action |
| --- | --- |
| `e` | Edit field value via `$EDITOR` |
| `c` | Clone item via `$EDITOR` |

### SQS — Queue Detail

| Key | Action |
| --- | --- |
| `m` | View messages |
| `n` | Navigate to dead letter queue |

### SQS — Messages

| Key | Action |
| --- | --- |
| `p` | Poll for messages |
| `s` | Send message via `$EDITOR` |
| `c` | Clone & send message |
| `x` | Delete message |

### CodeBuild

| Key | Action |
| --- | --- |
| `b` | Start new build (with confirmation) |
| `t` | Toggle local/UTC timestamps (builds list) |

### CodeBuild — Build Detail

| Key | Action |
| --- | --- |
| `l` | View build logs |
| `s` | Search build logs (full, server-side) |
| `b` | Start new build |
| `x` | Stop build (if in progress) |

### EC2 — Instances

| Key | Action |
| --- | --- |
| `Enter` | View instance detail |
| `/` | Filter instances |

### EC2 — Instance Detail

| Key | Action |
| --- | --- |
| `e` | SSM session (shell into instance) |
| `c` | View console output |
| `S` | Start instance |
| `X` | Stop instance |
| `r` | Reboot instance |
| `T` | Terminate instance |

### ECR — Images

| Key | Action |
| --- | --- |
| `Enter` | View scan findings |
| `s` | Start on-demand scan |
| `y` | Copy image URI to clipboard |
| `x` | Delete image |

### Route53 — Records

| Key | Action |
| --- | --- |
| `Enter` | View record detail |
| `n` | Create new record |

### Route53 — Record Detail

| Key | Action |
| --- | --- |
| `t` | Test DNS resolution |
| `e` | Edit record |
| `x` | Delete record |

### All Pickers (saved items)

| Key | Action |
| --- | --- |
| `d` | Delete selected saved entry |

## Configuration

Config is stored at `~/.config/e9s/config.yaml` (XDG convention). Press `ctrl+e` to edit it in your `$EDITOR`, or it hot-reloads on file changes.

```yaml
defaults:
  cluster: my-cluster
  region: us-east-2
  profile: ""
  refresh_interval: 5
  default_mode: ""        # start in this mode (e.g. "ECS", "CWL", "SQS")
  save_dir: ~/Downloads   # default directory for file saves

display:
  timestamp_format: relative  # "relative" or "absolute"
  max_events: 50
  max_log_lines: 1000

# Enable/disable modules (all enabled by default)
modules:
  ecs: true
  cloudwatch_logs: true
  cloudwatch_alarms: true
  ssm: true
  sm: true
  s3: true
  lambda: true
  dynamodb: true
  sqs: true
  codebuild: true
  ec2_instances: true
  ecr: true
  route53: true

# Saved bookmarks (managed via W/d keys in the TUI)
ssm_prefixes: []
sm_filters: []
s3_searches: []
lambda_searches: []
log_paths: []
dynamo_tables: []
dynamo_queries: []
sqs_queues: []

exclude_services: []
```

CLI flags override config file values.

## AWS Permissions

Your IAM identity needs permissions for whichever modules you use:

| Module | API Calls |
| --- | --- |
| ECS browse | `ecs:ListClusters`, `ecs:DescribeClusters`, `ecs:ListServices`, `ecs:DescribeServices`, `ecs:ListTasks`, `ecs:DescribeTasks` |
| ECS operations | `ecs:UpdateService`, `ecs:StopTask` |
| ECS Exec | `ecs:ExecuteCommand`, `ssmmessages:*` |
| Task definitions | `ecs:DescribeTaskDefinition` |
| CloudWatch Logs | `logs:DescribeLogGroups`, `logs:DescribeLogStreams`, `logs:FilterLogEvents`, `logs:GetLogEvents` |
| CloudWatch Alarms | `cloudwatch:DescribeAlarms`, `cloudwatch:DescribeAlarmHistory`, `cloudwatch:EnableAlarmActions`, `cloudwatch:DisableAlarmActions`, `cloudwatch:SetAlarmState` |
| CloudWatch Metrics | `cloudwatch:GetMetricData` |
| SSM parameters | `ssm:GetParametersByPath`, `ssm:GetParameter`, `ssm:GetParameters`, `ssm:PutParameter` |
| Secrets Manager | `secretsmanager:ListSecrets`, `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue` |
| S3 | `s3:ListBuckets`, `s3:ListObjectsV2`, `s3:HeadObject`, `s3:GetObject`, `s3:GetObjectTagging` |
| Lambda | `lambda:ListFunctions`, `lambda:GetFunction` |
| DynamoDB | `dynamodb:ListTables`, `dynamodb:DescribeTable`, `dynamodb:Scan`, `dynamodb:GetItem`, `dynamodb:UpdateItem`, `dynamodb:PutItem`, `dynamodb:ExecuteStatement` |
| SQS | `sqs:ListQueues`, `sqs:GetQueueAttributes`, `sqs:ReceiveMessage`, `sqs:DeleteMessage`, `sqs:SendMessage` |
| CodeBuild | `codebuild:ListProjects`, `codebuild:BatchGetProjects`, `codebuild:ListBuildsForProject`, `codebuild:BatchGetBuilds`, `codebuild:StartBuild`, `codebuild:StopBuild` |
| EC2 browse | `ec2:DescribeInstances`, `ec2:DescribeVolumes`, `ec2:DescribeSecurityGroups`, `ec2:GetConsoleOutput` |
| EC2 operations | `ec2:StartInstances`, `ec2:StopInstances`, `ec2:RebootInstances`, `ec2:TerminateInstances` |
| EC2 SSM session | `ssm:StartSession`, `ssmmessages:*` |
| ECR browse | `ecr:DescribeRepositories`, `ecr:DescribeImages`, `ecr:DescribeImageScanFindings` |
| ECR operations | `ecr:StartImageScan`, `ecr:BatchDeleteImage` |
| Route53 browse | `route53:ListHostedZones`, `route53:ListResourceRecordSets`, `route53:TestDNSAnswer` |
| Route53 operations | `route53:ChangeResourceRecordSets` |

## License

[MIT](LICENSE)
