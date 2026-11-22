# Sign-in for the dashboard: a Cognito user pool with no self sign-up (the owner's user is created by
# hand) and TOTP MFA required, plus the dashboard's public app client (authorization code + PKCE,
# no client secret, since a browser cannot keep one). cloud-api checks the access tokens it issues.
#
# Cost: $0. The Lite tier includes 10,000 monthly active users; TOTP is free (SMS MFA would not be).

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

resource "aws_cognito_user_pool" "this" {
  name                = "${var.name}-users"
  user_pool_tier      = "LITE"
  deletion_protection = "ACTIVE"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  username_configuration {
    case_sensitive = false
  }

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  mfa_configuration = "ON"
  software_token_mfa_configuration {
    enabled = true
  }

  password_policy {
    minimum_length                   = 14
    require_lowercase                = true
    require_uppercase                = true
    require_numbers                  = true
    require_symbols                  = false
    temporary_password_validity_days = 3
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }
}

# The hosted sign-in page at https://<prefix>.auth.<region>.amazoncognito.com. The prefix is public
# and global, so it is derived from the account id without showing it.
resource "aws_cognito_user_pool_domain" "this" {
  domain       = "${var.name}-${substr(sha256(data.aws_caller_identity.current.account_id), 0, 10)}"
  user_pool_id = aws_cognito_user_pool.this.id
}

resource "aws_cognito_user_pool_client" "dashboard" {
  name         = "${var.name}-dashboard"
  user_pool_id = aws_cognito_user_pool.this.id

  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = var.callback_urls
  logout_urls                          = var.callback_urls

  explicit_auth_flows           = ["ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]
  prevent_user_existence_errors = "ENABLED"
  enable_token_revocation       = true

  # The dashboard keeps tokens in memory only. A short refresh token bounds what a stolen one is worth.
  access_token_validity  = 60
  id_token_validity      = 60
  refresh_token_validity = 12
  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "hours"
  }
}
