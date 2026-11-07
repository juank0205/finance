# Lets GitHub Actions get short-lived AWS credentials without stored access keys. The OIDC provider
# is account-wide (one per account); modules like web add their own narrowly scoped roles on top.
# The plan role can only READ (terraform plan). Applying stays a deliberate, local action.

data "aws_partition" "current" {}

locals {
  partition = data.aws_partition.current.partition
}

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_policy_document" "plan_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        "repo:${var.github_repo}:ref:refs/heads/main",
        "repo:${var.github_repo}:pull_request",
      ]
    }
  }
}

resource "aws_iam_role" "plan" {
  name                 = "${var.name}-github-plan"
  assume_role_policy   = data.aws_iam_policy_document.plan_trust.json
  max_session_duration = 3600
}

# ReadOnlyAccess includes reading S3 objects. That is fine while S3 holds only the state and the
# public dashboard build; buckets with financial data must deny this role in their bucket policy.
resource "aws_iam_role_policy_attachment" "plan_readonly" {
  role       = aws_iam_role.plan.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/ReadOnlyAccess"
}
