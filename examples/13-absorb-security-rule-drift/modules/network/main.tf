variable "resource_group_name" {
  type = string
}

variable "location" {
  type = string
}

# VNet with inline subnet blocks — subnets will drift
resource "azurerm_virtual_network" "main" {
  name                = "graft-absorb-nested-test-vnet"
  location            = var.location
  resource_group_name = var.resource_group_name
  address_space       = ["10.0.0.0/16"]

  subnet {
    name                            = "web-subnet"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }

  subnet {
    name                            = "app-subnet"
    address_prefixes                = ["10.0.2.0/24"]
    default_outbound_access_enabled = false
  }

  tags = {
    environment = "test"
    layer       = "network"
  }
}

# Grandchild module — contains NSG with security_rule blocks
module "security" {
  source = "./security"

  resource_group_name = var.resource_group_name
  location            = var.location
}

output "vnet_id" {
  value = azurerm_virtual_network.main.id
}

output "nsg_id" {
  value = module.security.nsg_id
}
