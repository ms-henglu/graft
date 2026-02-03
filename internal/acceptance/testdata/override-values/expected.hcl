# Tests that Graft can override attribute values in a module's resources.

command = "build"

expected ".graft/build/app/_graft_override.tf" {
  content {
    resource "local_file" "config" {
      content = "overridden content"
    }
  }
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/app"]
}
