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

# Root-level resource — will have tag drift
resource "azurerm_resource_group" "test" {
  name     = "graft-absorb-nested-test-rg"
  location = "eastus"

  tags = {
    environment = "test"
    project     = "graft"
  }
}

# Child module — contains VNet (with inline subnets) and calls a grandchild module
module "network" {
  source = "./modules/network"

  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
}
