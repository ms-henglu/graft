variable "resource_group_name" {
  type = string
}

variable "location" {
  type = string
}

# NSG with inline security_rule blocks — rules and tags will drift
resource "azurerm_network_security_group" "main" {
  name                = "graft-absorb-nested-test-nsg"
  location            = var.location
  resource_group_name = var.resource_group_name

  security_rule {
    name                       = "allow-ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "10.0.0.0/8"
    destination_address_prefix = "*"
  }

  tags = {
    environment = "test"
    layer       = "security"
  }
}

output "nsg_id" {
  value = azurerm_network_security_group.main.id
}
