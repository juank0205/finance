terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
      # us_east_1 hosts the few global pieces that only exist there (root sign-in events).
      configuration_aliases = [aws.us_east_1]
    }
  }
}
