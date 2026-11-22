output "table_name" {
  description = "Ledger table, for cloud-api's FINANCE_TABLE"
  value       = aws_dynamodb_table.ledger.name
}

output "table_arn" {
  description = "Ledger table, for cloud-api's IAM policy"
  value       = aws_dynamodb_table.ledger.arn
}
