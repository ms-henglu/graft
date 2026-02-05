# Tests deep merge behavior for nested blocks
# The override should merge into all subnet blocks preserving their original attributes

command = "build"

expected ".graft/build/network/_graft_override.tf" {
  # Verify that static subnets have their original attributes preserved and override merged
  content = <<-EOF
resource "azurerm_virtual_network" "main" {
  subnet {
    address_prefixes                = ["10.0.1.0/24"]
    name                            = "subnet1"
    default_outbound_access_enabled = false
  }
  subnet {
    address_prefixes                = ["10.0.2.0/24"]
    name                            = "subnet2"
    default_outbound_access_enabled = false
  }
  dynamic "subnet" {
    for_each = var.extra_subnets
    content {
      address_prefixes                = subnet.value.address_prefixes
      name                            = subnet.value.name
      default_outbound_access_enabled = false
    }
  }
}
EOF
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/network"]
}
