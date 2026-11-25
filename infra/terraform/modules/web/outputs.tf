output "url" {
  description = "Where the dashboard is served"
  value       = "https://${local.custom_domain ? var.domain_name : aws_cloudfront_distribution.site.domain_name}"
}

output "cloudfront_domain" {
  description = "The distribution's own address: the target of the domain's CNAME record"
  value       = aws_cloudfront_distribution.site.domain_name
}

output "certificate_validation" {
  description = "The CNAME record ACM needs at the registrar; null without a domain"
  value = local.custom_domain ? one([
    for o in aws_acm_certificate.site[0].domain_validation_options : {
      name  = o.resource_record_name
      value = o.resource_record_value
    }
  ]) : null
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
