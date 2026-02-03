# Tests that Graft can override local values in a module.

expected ".graft/build/app/_graft_override.tf" {
  content {
    locals {
      environment = "production"
    }
  }
}

expected ".graft/build/app/_graft_add.tf" {
  content {
    locals {
      extra_tags = {
        ManagedBy = "Graft"
      }
    }
  }
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/app"]
}
