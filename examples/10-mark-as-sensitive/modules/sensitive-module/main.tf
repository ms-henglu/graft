
# For demonstration purposes only. It's recommended to use "random_password" in production.
resource "random_string" "db_password" {
  length           = 16
  special          = true
  override_special = "/@£$"
}

output "db_password" {
  value = random_string.db_password.result
}

output "connection_string" {
  value = "postgres://user:${random_string.db_password.result}@localhost:5432/db"
}
