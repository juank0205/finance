output "alerts_topic_arn" {
  value = aws_sns_topic.alerts.arn
}

output "trail_bucket" {
  description = "Where CloudTrail log files are kept"
  value       = aws_s3_bucket.trail.bucket
}
