# Tests removing all nested blocks (both static and dynamic) of a type

command = "build"

expected ".graft/build/network/main.tf" {
  # Verify both static and dynamic subnet blocks are removed
  excludes = [
    "static-subnet-1",
    "static-subnet-2",
    "dynamic \"subnet\"",
    "for_each = var.extra_subnets"
  ]
  # Verify the resource still exists with other attributes
  contains = [
    "azurerm_virtual_network",
    "address_space"
  ]
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/network"]
}
