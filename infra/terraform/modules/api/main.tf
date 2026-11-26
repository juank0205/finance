# cloud-api: the dashboard's API as one Lambda function (arm64 container image from ECR, Lambda Web
# Adapter inside) reached through a Lambda Function URL.
#
# The Function URL takes only SigV4-signed requests (auth type AWS_IAM). The dashboard's CloudFront
# distribution signs them with Origin Access Control and is the only caller allowed in
# (modules/web), so the API is https://<dashboard>/api/ and anything sent straight to the Function
# URL gets 403 before cloud-api runs. Every /v1 request must also carry a Cognito access token,
# which cloud-api checks itself and refuses to start without.
#
# Cost: $0 inside Lambda's always-free allowance (1M requests and 400,000 GB-seconds a month) and
# CloudWatch Logs' 5 GB. ECR storage is $0.10/GB-month past its 12-month free tier: the ~10 MB
# images kept here come to about $0.003/month.
#
# The first deploy is two steps: push an image to the repository, then apply with its digest
# (cloud_api_image_digest). Until a digest is set only the repository exists. After that, CI deploys
# every change on main (the deploy role below) and Terraform no longer tracks the image. See
# docs/deploy-api.md.

data "aws_partition" "current" {}
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  partition     = data.aws_partition.current.partition
  function_name = "${var.name}-cloud-api"
  function_arn  = "arn:${local.partition}:lambda:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:function:${local.function_name}"
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

  # CI moves the function to each new image (aws lambda update-function-code). image_digest only
  # picks the image the function is created with, so an apply never rolls back what CI deployed.
  lifecycle {
    ignore_changes = [image_uri]
  }
}

resource "aws_lambda_function_url" "cloud_api" {
  count              = local.deploy ? 1 : 0
  function_name      = aws_lambda_function.cloud_api[0].function_name
  authorization_type = "AWS_IAM"
}

# --- Deploy role for GitHub Actions ---------------------------------------------------------------

data "aws_iam_policy_document" "deploy_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [var.github_oidc_provider_arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    # Only workflows running on main may deploy; pull requests and other branches may not.
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_repo}:ref:refs/heads/main"]
    }
  }
}

resource "aws_iam_role" "deploy" {
  name                 = "${var.name}-github-deploy-api"
  assume_role_policy   = data.aws_iam_policy_document.deploy_trust.json
  max_session_duration = 3600
}

# Push to this one repository and move this one function to a new image. It cannot change the
# function's role, settings or URL, and cannot touch the table.
data "aws_iam_policy_document" "deploy" {
  statement {
    sid       = "EcrLogin"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"] # the action has no resource-level permissions
  }
  # A signed-in pull of the base images on public.ecr.aws. Anonymous pulls share a quota per IP,
  # which GitHub's shared runners use up ("429 Too Many Requests"). Pull only: pushing to ECR Public
  # needs other actions. Neither action has resource-level permissions.
  statement {
    sid       = "EcrPublicLogin"
    actions   = ["ecr-public:GetAuthorizationToken", "sts:GetServiceBearerToken"]
    resources = ["*"]
  }
  statement {
    sid = "PushImage"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage",
      "ecr:DescribeImages",
      # Lambda pulls the image with the caller's permissions when the code is updated.
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [aws_ecr_repository.cloud_api.arn]
  }
  statement {
    sid = "UpdateFunction"
    actions = [
      "lambda:UpdateFunctionCode",
      "lambda:GetFunction",
      "lambda:GetFunctionConfiguration",
      "lambda:GetFunctionUrlConfig",
    ]
    resources = [local.function_arn]
  }
}

resource "aws_iam_role_policy" "deploy" {
  name   = "deploy-cloud-api"
  role   = aws_iam_role.deploy.id
  policy = data.aws_iam_policy_document.deploy.json
}
