variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "image_digest" {
  description = "sha256:... digest of the cloud-api image in the repository; empty = repository only"
  type        = string
  default     = ""
  validation {
    condition     = var.image_digest == "" || can(regex("^sha256:[0-9a-f]{64}$", var.image_digest))
    error_message = "image_digest must look like sha256:<64 hex characters>."
  }
}

variable "table_name" {
  description = "Ledger table (modules/data)"
  type        = string
}

variable "table_arn" {
  description = "Ledger table ARN (modules/data)"
  type        = string
}

variable "cognito_issuer" {
  description = "Token issuer of the user pool (modules/auth)"
  type        = string
}

variable "cognito_client_id" {
  description = "The dashboard's app client (modules/auth)"
  type        = string
}

variable "allowed_origins" {
  description = "Browser origins allowed to call the API (CORS)"
  type        = list(string)
}

variable "reserved_concurrency" {
  description = "Cap on concurrent executions; null = no cap"
  type        = number
  default     = null
}
