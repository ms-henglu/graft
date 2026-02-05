terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
}

variable "vnet_name" {
  type    = string
  default = "my-vnet"
}

variable "location" {
  type    = string
  default = "eastus"
}

variable "extra_subnets" {
  type = list(object({
    name             = string
    address_prefixes = list(string)
  }))
  default = []
}

resource "azurerm_virtual_network" "main" {
  name                = var.vnet_name
  location            = var.location
  resource_group_name = "my-rg"
  address_space       = ["10.0.0.0/16"]

  # Static subnet block
  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  # Another static subnet block
  subnet {
    name             = "subnet2"
    address_prefixes = ["10.0.2.0/24"]
  }

  # Dynamic subnet block
  dynamic "subnet" {
    for_each = var.extra_subnets
    content {
      name             = subnet.value.name
      address_prefixes = subnet.value.address_prefixes
    }
  }
}

output "vnet_id" {
  value = azurerm_virtual_network.main.id
}
