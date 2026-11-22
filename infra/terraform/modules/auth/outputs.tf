output "issuer" {
  description = "Token issuer, for cloud-api's COGNITO_ISSUER"
  value       = "https://cognito-idp.${data.aws_region.current.region}.amazonaws.com/${aws_cognito_user_pool.this.id}"
}

output "user_pool_id" {
  description = "For creating the owner's user (aws cognito-idp admin-create-user)"
  value       = aws_cognito_user_pool.this.id
}

output "client_id" {
  description = "The dashboard's app client, for COGNITO_CLIENT_ID and VITE_COGNITO_CLIENT_ID"
  value       = aws_cognito_user_pool_client.dashboard.id
}

output "hosted_ui_url" {
  description = "Base URL of the hosted sign-in page, for VITE_COGNITO_DOMAIN"
  value       = "https://${aws_cognito_user_pool_domain.this.domain}.auth.${data.aws_region.current.region}.amazoncognito.com"
}
