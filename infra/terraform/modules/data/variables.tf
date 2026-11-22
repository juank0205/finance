variable "name" {
  description = "Prefix for resource names"
  type        = string
}

# One person's reads and writes fit easily. Keep the sum of every table's units at 25 or less to
# stay in the free tier. Short bursts above these use DynamoDB's burst capacity.
variable "read_capacity" {
  description = "Provisioned read capacity units of the ledger table"
  type        = number
  default     = 10
}

variable "write_capacity" {
  description = "Provisioned write capacity units of the ledger table"
  type        = number
  default     = 5
}
