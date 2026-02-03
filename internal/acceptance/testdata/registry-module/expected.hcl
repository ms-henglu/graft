# Tests that Graft can vendor and patch a module from the public Terraform registry.

command = "build"

expected ".graft/build/labels/_graft_override.tf" {
  content {
    locals {
      delimiter = "_"
    }
  }
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/labels"]
}
