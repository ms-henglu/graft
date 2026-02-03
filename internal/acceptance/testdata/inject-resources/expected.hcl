# Tests that Graft can inject new resources and outputs into a module.

command = "build"

expected ".graft/build/app/_graft_add.tf" {
  content {
    # Inject a new resource into the module
    resource "random_id" "injected" {
      byte_length = 4
    }
    # Inject a new output to expose the value
    output "injected_id" {
      value = random_id.injected.hex
    }
  }
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/app"]
}
