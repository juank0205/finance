output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.github.arn
}

output "plan_role_arn" {
  description = "GitHub variable AWS_PLAN_ROLE_ARN"
  value       = aws_iam_role.plan.arn
}
