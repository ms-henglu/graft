# Test Case: Root Override
# Tests that Graft can apply overrides to root module resources (not in submodules).

resource "local_file" "root_config" {
  content  = "original root content"
  filename = "${path.module}/root_output.txt"
}

output "root_file" {
  value = local_file.root_config.filename
}
