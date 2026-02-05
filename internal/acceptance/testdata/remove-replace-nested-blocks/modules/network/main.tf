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

resource "azurerm_virtual_network" "main" {
  name                = var.vnet_name
  location            = var.location
  resource_group_name = "my-rg"
  address_space       = ["10.0.0.0/16"]

  # Original subnet blocks that will be removed and replaced
  subnet {
    name             = "old-subnet-1"
    address_prefixes = ["10.0.1.0/24"]
  }

  subnet {
    name             = "old-subnet-2"
    address_prefixes = ["10.0.2.0/24"]
  }

  subnet {
    name             = "old-subnet-3"
    address_prefixes = ["10.0.3.0/24"]
  }
}
