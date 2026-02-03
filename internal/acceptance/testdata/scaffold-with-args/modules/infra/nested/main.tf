resource "local_file" "nested_config" {
  content  = "nested content"
  filename = "${path.module}/nested.txt"
}
