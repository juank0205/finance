output "repository_url" {
  description = "Where to push the cloud-api image"
  value       = aws_ecr_repository.cloud_api.repository_url
}

output "url" {
  description = "cloud-api's Function URL, for the dashboard's VITE_API_URL; null until an image is deployed"
  value       = one(aws_lambda_function_url.cloud_api[*].function_url)
}

output "deploy_role_arn" {
  description = "GitHub variable AWS_API_DEPLOY_ROLE_ARN"
  value       = aws_iam_role.deploy.arn
}
