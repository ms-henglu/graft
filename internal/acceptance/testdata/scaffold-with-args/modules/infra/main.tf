resource "local_file" "infra_config" {
  content  = "infra content"
  filename = "${path.module}/infra.txt"
}

module "nested" {
  source = "./nested"
}
