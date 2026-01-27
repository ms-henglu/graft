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

module "network" {
  source              = "Azure/network/azurerm"
  version             = "5.3.0"
  resource_group_name = "example-rg-removal"
  vnet_name           = "example-vnet-removal"
  address_spaces      = ["10.0.0.0/16"]
  subnet_prefixes     = ["10.0.1.0/24"]
  subnet_names        = ["subnet1"]
  use_for_each        = true
  # Suppose the module defaults service_endpoints to something, or we passed it and want to strip it
  # internally (though passing it in main.tf is easier to change, assumes module hardcodes it)
}
