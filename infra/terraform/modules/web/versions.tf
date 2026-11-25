terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
      # us_east_1 holds the domain's certificate: CloudFront only uses certificates from there.
      configuration_aliases = [aws.us_east_1]
    }
  }
}
