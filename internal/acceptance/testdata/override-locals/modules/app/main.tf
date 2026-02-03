locals {
  environment = "development"
  name        = "my-app"
}

output "environment" {
  value = local.environment
}

output "name" {
  value = local.name
}
