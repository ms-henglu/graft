# Tests that Graft can override attribute values in a module's resources.

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
