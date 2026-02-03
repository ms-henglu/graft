# Tests that Graft can apply overrides to root module resources.

command = "build"

expected "_graft_override.tf" {
  content {
    resource "local_file" "root_config" {
      content = "overridden root content"
    }
  }
}
