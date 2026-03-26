# e9s Demo Environment

OpenTofu/Terraform configuration and VHS recording script for creating demo content.

## Infrastructure

`main.tf` creates a small set of AWS resources that exercise every e9s module:

| Module | Resources |
| --- | --- |
| ECS | Fargate cluster with `api` (nginx) and `worker` (busybox JSON logger) services |
| CloudWatch Logs | Log groups for both ECS services |
| CloudWatch Alarms | CPU, memory, DLQ, and Lambda error alarms |
| SSM | Parameters under `/e9s-demo/` (including SecureString) |
| Secrets Manager | API key, DB credentials, OAuth config (JSON) |
| S3 | Two buckets with sample config, docs, and log files |
| Lambda | `hello` (API handler) and `cron-cleanup` functions |
| DynamoDB | `orders` table with 8 sample items, `users` table |
| SQS | Standard queue with DLQ, FIFO notification queue |
| CodeBuild | Project pointed at the e9s repo |

### Deploy

```bash
cd demo
tofu init    # or: terraform init
tofu apply   # or: terraform apply
```

### Tear down

```bash
cd demo
tofu destroy   # or: terraform destroy
```

**Cost:** Most resources are free-tier eligible or cost pennies. The Fargate tasks (~$0.01/hr each) are the main cost. Tear down promptly after recording.

## Recording

Requires [vhs](https://github.com/charmbracelet/vhs):

```bash
# Install vhs
go install github.com/charmbracelet/vhs@latest

# Record the demo (from repo root)
vhs demo/demo.tape
```

The tape file navigates through ECS, CloudWatch Logs, DynamoDB, SQS, and CloudWatch Alarms. Edit `demo.tape` to adjust timing or add/remove sections.

Output: `demo/e9s-demo.gif` and `demo/e9s-demo.webm`.
