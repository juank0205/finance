output "url" {
  description = "Where the dashboard is served"
  value       = "https://${aws_cloudfront_distribution.site.domain_name}"
}

output "bucket" {
  description = "GitHub variable WEB_BUCKET"
  value       = aws_s3_bucket.site.bucket
}

output "distribution_id" {
  description = "GitHub variable WEB_DISTRIBUTION_ID"
  value       = aws_cloudfront_distribution.site.id
}

output "deploy_role_arn" {
  description = "GitHub variable AWS_DEPLOY_ROLE_ARN"
  value       = aws_iam_role.deploy.arn
}
