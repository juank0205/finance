variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "alert_email" {
  description = "Receives the budget emails"
  type        = string
}

variable "limit_usd" {
  description = "Monthly limit. Emails when actual or forecasted spend goes above it"
  type        = number
  default     = 1
}
