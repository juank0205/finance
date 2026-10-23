# Lets GitHub Actions get short-lived AWS credentials without stored access keys.
# The CI role can only READ (terraform plan). Applying stays a deliberate, local action for now.

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_policy_document" "github_plan_trust" {
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

resource "aws_iam_role" "github_plan" {
  name                 = "${var.name}-github-plan"
  assume_role_policy   = data.aws_iam_policy_document.github_plan_trust.json
  max_session_duration = 3600
}

# ReadOnlyAccess has no kms:Decrypt, so the role still cannot read data encrypted with our key.
resource "aws_iam_role_policy_attachment" "github_plan_readonly" {
  role       = aws_iam_role.github_plan.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/ReadOnlyAccess"
}

data "aws_kms_alias" "tfstate" {
  name = var.state_kms_alias
}

# Exception to the rule above: plan must decrypt the Terraform state itself.
data "aws_iam_policy_document" "github_plan_state" {
  statement {
    sid       = "ListStateBucket"
    actions   = ["s3:ListBucket"]
    resources = ["arn:${local.partition}:s3:::${var.state_bucket}"]
  }
  statement {
    sid       = "ReadState"
    actions   = ["s3:GetObject"]
    resources = ["arn:${local.partition}:s3:::${var.state_bucket}/*"]
  }
  statement {
    sid       = "DecryptState"
    actions   = ["kms:Decrypt"]
    resources = [data.aws_kms_alias.tfstate.target_key_arn]
  }
}

resource "aws_iam_role_policy" "github_plan_state" {
  name   = "terraform-state-read"
  role   = aws_iam_role.github_plan.id
  policy = data.aws_iam_policy_document.github_plan_state.json
}
