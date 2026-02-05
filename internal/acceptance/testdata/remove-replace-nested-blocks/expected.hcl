# Tests the workaround for fully replacing nested blocks
# Remove all existing subnet blocks and replace with new ones

command = "build"

expected ".graft/build/network/_graft_override.tf" {
  content = <<-EOF
resource "azurerm_virtual_network" "main" {
  subnet {
    address_prefixes = ["10.0.10.0/24"]
    name             = "new-subnet-1"
  }
  subnet {
    address_prefixes = ["10.0.20.0/24"]
    name             = "new-subnet-2"
  }
}
EOF
}

expected ".graft/build/network/main.tf" {
  # Verify original subnet blocks are removed from source
  excludes = [
    "old-subnet-1",
    "old-subnet-2",
    "old-subnet-3"
  ]
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/network"]
}
