# Test Case: Public Registry Module
# Tests that Graft can vendor and patch a module from the public Terraform registry.

terraform {
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

module "labels" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  namespace   = "eg"
  environment = "dev"
  name        = "app"
}
