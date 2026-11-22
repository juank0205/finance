# The ledger: one DynamoDB table holding transactions, recurring rules and categories. Its layout is
# documented in backend/internal/ledger/dynamostore (CreateTable there must match this resource).
# The Pi's raw-event bucket and queue arrive with the ingestion part.
#
# Cost: $0 inside DynamoDB's always-free allowance (25 GB, 25 read + 25 write capacity units of
# provisioned capacity). On-demand mode is not covered by that allowance, hence provisioned. Point-in-
# time recovery is billed per GB stored; at a few MB this is far below $0.01/month.

resource "aws_dynamodb_table" "ledger" {
  name           = "${var.name}-ledger"
  billing_mode   = "PROVISIONED"
  read_capacity  = var.read_capacity
  write_capacity = var.write_capacity
  hash_key       = "pk"
  range_key      = "sk"

  attribute {
    name = "pk"
    type = "S"
  }
  attribute {
    name = "sk"
    type = "S"
  }
  attribute {
    name = "ts"
    type = "S"
  }

  # Transactions by time. A local index shares the table's capacity, so it costs no extra units.
  local_secondary_index {
    name            = "by_time"
    range_key       = "ts"
    projection_type = "ALL"
  }

  # Encryption at rest is always on. Without a server_side_encryption block it uses an AWS owned key,
  # which is free; a KMS key would add per-request charges.

  # Restore to any second of the last 35 days, e.g. after a bad deploy corrupts data.
  point_in_time_recovery {
    enabled = true
  }

  # This table is the source of truth: refuse deletion both in AWS and in Terraform.
  deletion_protection_enabled = true
  lifecycle {
    prevent_destroy = true
  }
}
