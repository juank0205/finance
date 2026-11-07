# Phase 0: account-wide guardrails, built to fit the free tier. Encryption at rest uses S3-managed keys
# (no $1/month KMS key) and GuardDuty is opt-in (it bills after a 30-day trial). The only non-zero
# item is CloudTrail writing log files to S3: a few thousand requests a month, about $0.10.

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}
data "aws_partition" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.region
  partition  = data.aws_partition.current.partition
  trail_name = "${var.name}-trail"
  # Built by hand so the bucket policy can reference the trail without a dependency cycle.
  trail_arn = "arn:${local.partition}:cloudtrail:${local.region}:${local.account_id}:trail/${local.trail_name}"

  forward_global_events = local.region != "us-east-1"
  home_bus_arn          = "arn:${local.partition}:events:${local.region}:${local.account_id}:event-bus/default"
}

# --- Account-level hardening (all free) ---------------------------------------------------------

# No bucket in the account can ever be made public. CloudFront's access to the dashboard bucket is
# not "public" (it is limited to one distribution), so it is unaffected.
resource "aws_s3_account_public_access_block" "this" {
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_ebs_encryption_by_default" "this" {
  enabled = true
}

# Flags any resource policy that grants access to someone outside this account.
resource "aws_accessanalyzer_analyzer" "account" {
  analyzer_name = "${var.name}-account"
  type          = "ACCOUNT"
}

# Humans sign in through Identity Center, so IAM users shouldn't exist; if one ever does, it gets this.
resource "aws_iam_account_password_policy" "strict" {
  minimum_password_length        = 16
  require_lowercase_characters   = true
  require_uppercase_characters   = true
  require_numbers                = true
  require_symbols                = true
  allow_users_to_change_password = true
  password_reuse_prevention      = 24
}
