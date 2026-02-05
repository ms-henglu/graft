terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
}

variable "vnet_name" {
  type        = string
  description = "Name of the virtual network"
}

variable "location" {
  type        = string
  description = "Azure region"
}

variable "extra_subnets" {
  type = list(object({
    name             = string
    address_prefixes = list(string)
  }))
  description = "Additional subnets to create via dynamic block"
  default     = []
}

resource "azurerm_virtual_network" "main" {
  name                = var.vnet_name
  location            = var.location
  resource_group_name = "my-rg"
  address_space       = ["10.0.0.0/16"]

  # Static subnet block 1
  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  # Static subnet block 2
  subnet {
    name             = "subnet2"
    address_prefixes = ["10.0.2.0/24"]
  }

  # Dynamic subnet block for additional subnets
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
