# cloud-api: the dashboard's API as one Lambda function (arm64 container image from ECR, Lambda Web
# Adapter inside) reached through a Lambda Function URL.
#
# The Function URL is public (auth type NONE): it is how a browser reaches Lambda without API Gateway
# or CloudFront. Every /v1 request must carry a Cognito access token, which cloud-api checks itself
# and refuses to start without. CORS only lets the listed dashboard origins call it from a browser.
#
# Cost: $0 inside Lambda's always-free allowance (1M requests and 400,000 GB-seconds a month) and
# CloudWatch Logs' 5 GB. ECR storage is $0.10/GB-month past its 12-month free tier: the ~10 MB
# images kept here come to about $0.003/month.
#
# Deploying is two steps: push an image to the repository, then apply with its digest
# (cloud_api_image_digest). Until a digest is set only the repository exists. See docs/deploy-api.md.

data "aws_partition" "current" {}

locals {
  partition     = data.aws_partition.current.partition
  function_name = "${var.name}-cloud-api"
  deploy        = var.image_digest != ""
}

# --- Image repository -----------------------------------------------------------------------------

resource "aws_ecr_repository" "cloud_api" {
  name                 = "${var.name}/cloud-api"
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration {
    scan_on_push = true # basic scanning, free
  }
}

resource "aws_ecr_lifecycle_policy" "cloud_api" {
  repository = aws_ecr_repository.cloud_api.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep the last 3 images (current + rollbacks)"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 3
      }
      action = { type = "expire" }
    }]
  })
}

# --- Execution role -------------------------------------------------------------------------------

resource "aws_cloudwatch_log_group" "cloud_api" {
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = 30
}

data "aws_iam_policy_document" "trust" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cloud_api" {
  name               = local.function_name
  assume_role_policy = data.aws_iam_policy_document.trust.json
}

# The item operations dynamostore uses, on the ledger table and its index only. No Scan, no table
# management, no other table.
data "aws_iam_policy_document" "cloud_api" {
  statement {
    sid       = "Ledger"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:DeleteItem", "dynamodb:Query"]
    resources = [var.table_arn, "${var.table_arn}/index/*"]
  }
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.cloud_api.arn}:*"]
  }
}

resource "aws_iam_role_policy" "cloud_api" {
  role   = aws_iam_role.cloud_api.id
  policy = data.aws_iam_policy_document.cloud_api.json
}

# --- Function and URL -----------------------------------------------------------------------------

resource "aws_lambda_function" "cloud_api" {
  count         = local.deploy ? 1 : 0
  function_name = local.function_name
  role          = aws_iam_role.cloud_api.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.cloud_api.repository_url}@${var.image_digest}"
  architectures = ["arm64"]
  memory_size   = 256
  # A statement import books up to 50 lines per request against a table provisioned at 5 writes per
  # second; its burst capacity usually absorbs that, and the SDK retries when it does not.
  timeout = 30

  # null leaves the function on the account's shared pool. A cap needs an account concurrency
  # limit above 10, which new accounts do not have.
  reserved_concurrent_executions = var.reserved_concurrency

  environment {
    variables = {
      FINANCE_TABLE     = var.table_name
      COGNITO_ISSUER    = var.cognito_issuer
      COGNITO_CLIENT_ID = var.cognito_client_id
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.cloud_api.name
  }

  depends_on = [aws_iam_role_policy.cloud_api]
}

resource "aws_lambda_function_url" "cloud_api" {
  count              = local.deploy ? 1 : 0
  function_name      = aws_lambda_function.cloud_api[0].function_name
  authorization_type = "NONE"

  cors {
    allow_origins = var.allowed_origins
    allow_methods = ["GET", "POST", "PUT", "DELETE"]
    allow_headers = ["authorization", "content-type"]
    max_age       = 300
  }
}

# A public Function URL needs both statements. The second only allows invocations that arrive
# through the URL, so nobody can call the function directly with their own AWS credentials.
resource "aws_lambda_permission" "url" {
  count                  = local.deploy ? 1 : 0
  statement_id           = "FunctionUrlPublic"
  action                 = "lambda:InvokeFunctionUrl"
  function_name          = aws_lambda_function.cloud_api[0].function_name
  principal              = "*"
  function_url_auth_type = "NONE"
}

resource "aws_lambda_permission" "url_invoke" {
  count                    = local.deploy ? 1 : 0
  statement_id             = "FunctionUrlInvoke"
  action                   = "lambda:InvokeFunction"
  function_name            = aws_lambda_function.cloud_api[0].function_name
  principal                = "*"
  invoked_via_function_url = true
}
