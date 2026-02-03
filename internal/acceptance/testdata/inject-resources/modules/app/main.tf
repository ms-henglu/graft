resource "random_string" "name" {
  length  = 8
  special = false
}

output "name" {
  value = random_string.name.result
}
