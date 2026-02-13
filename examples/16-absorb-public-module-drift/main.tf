terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
}

provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "example" {
  name     = "graft-absorb-module-rg"
  location = "eastus"

  tags = {
    environment = "dev"
    project     = "graft"
  }
}

# Public module from the Terraform Registry
module "network" {
  source  = "Azure/network/azurerm"
  version = "5.3.0"

  resource_group_name = azurerm_resource_group.example.name
  use_for_each        = true
  vnet_name           = "graft-example-vnet"
  vnet_location       = azurerm_resource_group.example.location
  address_space       = ["10.0.0.0/16"]

  subnet_names    = ["web-subnet", "app-subnet"]
  subnet_prefixes = ["10.0.1.0/24", "10.0.2.0/24"]

  tags = {
    environment = "dev"
    managed_by  = "terraform"
  }
}
