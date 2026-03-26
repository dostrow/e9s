terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project = "e9s-demo"
    }
  }
}

variable "region" {
  default = "us-east-1"
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# ---------- VPC (minimal, public-only for Fargate) ----------

resource "aws_vpc" "demo" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = "e9s-demo" }
}

resource "aws_internet_gateway" "demo" {
  vpc_id = aws_vpc.demo.id
}

resource "aws_subnet" "public_a" {
  vpc_id                  = aws_vpc.demo.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "${var.region}a"
  map_public_ip_on_launch = true
  tags                    = { Name = "e9s-demo-public-a" }
}

resource "aws_subnet" "public_b" {
  vpc_id                  = aws_vpc.demo.id
  cidr_block              = "10.0.2.0/24"
  availability_zone       = "${var.region}b"
  map_public_ip_on_launch = true
  tags                    = { Name = "e9s-demo-public-b" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.demo.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.demo.id
  }
}

resource "aws_route_table_association" "a" {
  subnet_id      = aws_subnet.public_a.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "b" {
  subnet_id      = aws_subnet.public_b.id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "ecs" {
  name_prefix = "e9s-demo-ecs-"
  vpc_id      = aws_vpc.demo.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ---------- ECS Cluster + Services ----------

resource "aws_ecs_cluster" "demo" {
  name = "e9s-demo"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_iam_role" "ecs_task_execution" {
  name = "e9s-demo-ecs-execution"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "ecs_task" {
  name = "e9s-demo-ecs-task"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "ecs_exec" {
  name = "ecs-exec"
  role = aws_iam_role.ecs_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ssmmessages:CreateControlChannel",
        "ssmmessages:CreateDataChannel",
        "ssmmessages:OpenControlChannel",
        "ssmmessages:OpenDataChannel",
      ]
      Resource = "*"
    }]
  })
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/e9s-demo/api"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/e9s-demo/worker"
  retention_in_days = 7
}

resource "aws_ecs_task_definition" "api" {
  family                   = "e9s-demo-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "api"
    image     = "nginx:alpine"
    essential = true
    portMappings = [{ containerPort = 80, protocol = "tcp" }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.api.name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "api"
      }
    }
    environment = [
      { name = "APP_NAME", value = "e9s-demo-api" },
      { name = "ENVIRONMENT", value = "demo" },
      { name = "LOG_LEVEL", value = "info" },
    ]
    secrets = [
      { name = "DB_PASSWORD", valueFrom = aws_ssm_parameter.db_password.arn },
      { name = "API_KEY", valueFrom = aws_secretsmanager_secret.api_key.arn },
    ]
  }])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "e9s-demo-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "worker"
    image     = "busybox:latest"
    essential = true
    command   = ["sh", "-c", "while true; do echo '{\"level\":\"info\",\"msg\":\"processing job\",\"jobId\":\"'$RANDOM'\",\"queue\":\"default\",\"duration_ms\":'$((RANDOM % 500))'}'; sleep $((RANDOM % 5 + 1)); done"]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.worker.name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "worker"
      }
    }
    environment = [
      { name = "APP_NAME", value = "e9s-demo-worker" },
      { name = "ENVIRONMENT", value = "demo" },
      { name = "QUEUE_URL", value = aws_sqs_queue.tasks.url },
    ]
  }])
}

resource "aws_ecs_service" "api" {
  name            = "api"
  cluster         = aws_ecs_cluster.demo.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = 2
  launch_type     = "FARGATE"

  enable_execute_command = true

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }
}

resource "aws_ecs_service" "worker" {
  name            = "worker"
  cluster         = aws_ecs_cluster.demo.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }
}

# ---------- SSM Parameters ----------

resource "aws_ssm_parameter" "db_host" {
  name  = "/e9s-demo/database/host"
  type  = "String"
  value = "demo-db.cluster-abc123.us-east-1.rds.amazonaws.com"
}

resource "aws_ssm_parameter" "db_port" {
  name  = "/e9s-demo/database/port"
  type  = "String"
  value = "5432"
}

resource "aws_ssm_parameter" "db_name" {
  name  = "/e9s-demo/database/name"
  type  = "String"
  value = "e9s_demo"
}

resource "aws_ssm_parameter" "db_password" {
  name  = "/e9s-demo/database/password"
  type  = "SecureString"
  value = "demo-secret-password-12345"
}

resource "aws_ssm_parameter" "api_url" {
  name  = "/e9s-demo/api/base-url"
  type  = "String"
  value = "https://api.e9s-demo.example.com"
}

resource "aws_ssm_parameter" "feature_flags" {
  name  = "/e9s-demo/features/dark-mode"
  type  = "String"
  value = "enabled"
}

# ---------- Secrets Manager ----------

resource "aws_secretsmanager_secret" "api_key" {
  name = "e9s-demo/api-key"
}

resource "aws_secretsmanager_secret_version" "api_key" {
  secret_id     = aws_secretsmanager_secret.api_key.id
  secret_string = "sk-demo-abc123def456ghi789"
}

resource "aws_secretsmanager_secret" "db_credentials" {
  name = "e9s-demo/db-credentials"
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = aws_secretsmanager_secret.db_credentials.id
  secret_string = jsonencode({
    username = "demo_admin"
    password = "demo-secret-password-12345"
    host     = "demo-db.cluster-abc123.us-east-1.rds.amazonaws.com"
    port     = 5432
    dbname   = "e9s_demo"
  })
}

resource "aws_secretsmanager_secret" "oauth_config" {
  name = "e9s-demo/oauth"
}

resource "aws_secretsmanager_secret_version" "oauth_config" {
  secret_id = aws_secretsmanager_secret.oauth_config.id
  secret_string = jsonencode({
    client_id     = "demo-client-id"
    client_secret = "demo-client-secret"
    redirect_uri  = "https://e9s-demo.example.com/auth/callback"
    scopes        = ["openid", "profile", "email"]
  })
}

# ---------- S3 ----------

resource "aws_s3_bucket" "data" {
  bucket_prefix = "e9s-demo-data-"
  force_destroy = true
}

resource "aws_s3_object" "config" {
  bucket  = aws_s3_bucket.data.id
  key     = "config/app.json"
  content = jsonencode({ environment = "demo", version = "1.0.0", features = { dark_mode = true } })
}

resource "aws_s3_object" "readme" {
  bucket  = aws_s3_bucket.data.id
  key     = "docs/README.md"
  content = "# e9s Demo\n\nThis bucket contains sample data for the e9s demo.\n"
}

resource "aws_s3_object" "logs_1" {
  bucket  = aws_s3_bucket.data.id
  key     = "logs/2026/03/25/access.log"
  content = "192.168.1.1 - - [25/Mar/2026:10:00:00 +0000] \"GET /api/health HTTP/1.1\" 200 15\n192.168.1.2 - - [25/Mar/2026:10:00:01 +0000] \"POST /api/users HTTP/1.1\" 201 234\n"
}

resource "aws_s3_object" "logs_2" {
  bucket  = aws_s3_bucket.data.id
  key     = "logs/2026/03/25/error.log"
  content = "2026-03-25T10:05:00Z ERROR connection timeout to database after 30s\n2026-03-25T10:05:05Z WARN retrying database connection (attempt 2/3)\n"
}

resource "aws_s3_bucket" "artifacts" {
  bucket_prefix = "e9s-demo-artifacts-"
  force_destroy = true
}

resource "aws_s3_object" "build_artifact" {
  bucket  = aws_s3_bucket.artifacts.id
  key     = "builds/v1.0.0/app.tar.gz"
  content = "placeholder"
}

# ---------- Lambda ----------

data "archive_file" "hello" {
  type        = "zip"
  output_path = "${path.module}/.terraform/hello.zip"

  source {
    content  = <<-PYTHON
    import json
    import time
    import random

    def handler(event, context):
        duration = random.randint(10, 200)
        time.sleep(duration / 1000)
        print(json.dumps({"level": "info", "msg": "request processed", "duration_ms": duration, "path": event.get("path", "/")}))
        return {
            "statusCode": 200,
            "body": json.dumps({"message": "Hello from e9s demo!", "version": "1.0.0"})
        }
    PYTHON
    filename = "index.py"
  }
}

resource "aws_iam_role" "lambda" {
  name = "e9s-demo-lambda"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "hello" {
  function_name    = "e9s-demo-hello"
  role             = aws_iam_role.lambda.arn
  handler          = "index.handler"
  runtime          = "python3.12"
  filename         = data.archive_file.hello.output_path
  source_code_hash = data.archive_file.hello.output_base64sha256
  timeout          = 10
  memory_size      = 128

  environment {
    variables = {
      APP_NAME    = "e9s-demo"
      ENVIRONMENT = "demo"
      DB_HOST     = aws_ssm_parameter.db_host.value
    }
  }
}

data "archive_file" "cron" {
  type        = "zip"
  output_path = "${path.module}/.terraform/cron.zip"

  source {
    content  = <<-PYTHON
    import json
    import random

    def handler(event, context):
        items = random.randint(5, 50)
        print(json.dumps({"level": "info", "msg": "cron job completed", "items_processed": items, "job": "cleanup"}))
        if random.random() < 0.2:
            print(json.dumps({"level": "warn", "msg": "slow processing detected", "items_processed": items, "threshold_ms": 1000}))
        return {"processed": items}
    PYTHON
    filename = "index.py"
  }
}

resource "aws_lambda_function" "cron" {
  function_name    = "e9s-demo-cron-cleanup"
  role             = aws_iam_role.lambda.arn
  handler          = "index.handler"
  runtime          = "python3.12"
  filename         = data.archive_file.cron.output_path
  source_code_hash = data.archive_file.cron.output_base64sha256
  timeout          = 60
  memory_size      = 256

  environment {
    variables = {
      ENVIRONMENT = "demo"
      TABLE_NAME  = aws_dynamodb_table.orders.name
    }
  }
}

# ---------- DynamoDB ----------

resource "aws_dynamodb_table" "orders" {
  name         = "e9s-demo-orders"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "orderId"
  range_key    = "createdAt"

  attribute {
    name = "orderId"
    type = "S"
  }

  attribute {
    name = "createdAt"
    type = "S"
  }
}

resource "aws_dynamodb_table_item" "orders" {
  for_each   = toset(["1", "2", "3", "4", "5", "6", "7", "8"])
  table_name = aws_dynamodb_table.orders.name
  hash_key   = aws_dynamodb_table.orders.hash_key
  range_key  = aws_dynamodb_table.orders.range_key

  item = jsonencode({
    orderId   = { S = "ORD-${format("%04d", tonumber(each.key))}" }
    createdAt = { S = "2026-03-${format("%02d", 18 + tonumber(each.key))}T${format("%02d", 8 + tonumber(each.key))}:00:00Z" }
    customer  = { S = element(["Alice Johnson", "Bob Smith", "Carol Williams", "Dave Brown", "Eve Davis", "Frank Miller", "Grace Wilson", "Henry Taylor"], tonumber(each.key) - 1) }
    status    = { S = element(["completed", "processing", "completed", "shipped", "completed", "pending", "completed", "cancelled"], tonumber(each.key) - 1) }
    total     = { N = tostring(element([49.99, 129.50, 24.00, 89.95, 199.99, 15.00, 74.50, 39.99], tonumber(each.key) - 1)) }
    items     = { L = [{ M = { name = { S = "Widget ${each.key}" }, qty = { N = each.key } } }] }
  })
}

resource "aws_dynamodb_table" "users" {
  name         = "e9s-demo-users"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "userId"

  attribute {
    name = "userId"
    type = "S"
  }
}

# ---------- SQS ----------

resource "aws_sqs_queue" "tasks_dlq" {
  name                      = "e9s-demo-tasks-dlq"
  message_retention_seconds = 1209600 # 14 days
}

resource "aws_sqs_queue" "tasks" {
  name                       = "e9s-demo-tasks"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600 # 4 days
  delay_seconds              = 0

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.tasks_dlq.arn
    maxReceiveCount     = 3
  })
}

resource "aws_sqs_queue" "notifications_fifo" {
  name                        = "e9s-demo-notifications.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  visibility_timeout_seconds  = 30
}

# ---------- CloudWatch Alarms ----------

resource "aws_cloudwatch_metric_alarm" "api_cpu" {
  alarm_name          = "e9s-demo-api-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "API service CPU utilization above 80%"
  treat_missing_data  = "notBreaching"

  dimensions = {
    ClusterName = aws_ecs_cluster.demo.name
    ServiceName = aws_ecs_service.api.name
  }
}

resource "aws_cloudwatch_metric_alarm" "api_memory" {
  alarm_name          = "e9s-demo-api-memory-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "MemoryUtilization"
  namespace           = "AWS/ECS"
  period              = 300
  statistic           = "Average"
  threshold           = 85
  alarm_description   = "API service memory utilization above 85%"
  treat_missing_data  = "notBreaching"

  dimensions = {
    ClusterName = aws_ecs_cluster.demo.name
    ServiceName = aws_ecs_service.api.name
  }
}

resource "aws_cloudwatch_metric_alarm" "dlq_messages" {
  alarm_name          = "e9s-demo-dlq-not-empty"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "ApproximateNumberOfMessagesVisible"
  namespace           = "AWS/SQS"
  period              = 300
  statistic           = "Sum"
  threshold           = 0
  alarm_description   = "Dead letter queue has messages — investigate failed task processing"
  treat_missing_data  = "notBreaching"

  dimensions = {
    QueueName = aws_sqs_queue.tasks_dlq.name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  alarm_name          = "e9s-demo-lambda-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = 5
  alarm_description   = "Lambda function error rate exceeded threshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.hello.function_name
  }
}

# ---------- CodeBuild ----------

resource "aws_iam_role" "codebuild" {
  name = "e9s-demo-codebuild"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "codebuild.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "codebuild" {
  name = "codebuild-base"
  role = aws_iam_role.codebuild.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:*"
      },
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${aws_s3_bucket.artifacts.arn}/*"
      },
    ]
  })
}

resource "aws_codebuild_project" "app" {
  name         = "e9s-demo-app"
  description  = "Build and test the e9s demo application"
  service_role = aws_iam_role.codebuild.arn

  artifacts {
    type = "NO_ARTIFACTS"
  }

  environment {
    compute_type = "BUILD_GENERAL1_SMALL"
    image        = "aws/codebuild/amazonlinux2-x86_64-standard:5.0"
    type         = "LINUX_CONTAINER"

    environment_variable {
      name  = "APP_ENV"
      value = "demo"
    }
  }

  source {
    type            = "GITHUB"
    location        = "https://github.com/dostrow/e9s.git"
    git_clone_depth = 1
    buildspec       = <<-YAML
      version: 0.2
      phases:
        install:
          runtime-versions:
            golang: 1.22
        build:
          commands:
            - echo "Building e9s..."
            - go build -o e9s .
            - echo "Running tests..."
            - go test ./...
      artifacts:
        files:
          - e9s
    YAML
  }
}

# ---------- Outputs ----------

output "region" {
  value = var.region
}

output "ecs_cluster" {
  value = aws_ecs_cluster.demo.name
}

output "s3_data_bucket" {
  value = aws_s3_bucket.data.id
}

output "s3_artifacts_bucket" {
  value = aws_s3_bucket.artifacts.id
}

output "sqs_tasks_queue" {
  value = aws_sqs_queue.tasks.url
}

output "dynamodb_orders_table" {
  value = aws_dynamodb_table.orders.name
}

output "teardown" {
  value = "Run 'tofu destroy' to remove all demo resources"
}
