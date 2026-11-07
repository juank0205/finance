output "kms_key_arn" {
  description = "The account's customer-managed key; pass to later modules"
  value       = aws_kms_key.main.arn
}

output "alerts_topic_arn" {
  value = aws_sns_topic.alerts.arn
}
