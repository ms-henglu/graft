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

resource "azurerm_resource_group" "test" {
  name     = "example-rg-logic"
  location = "westus"
}

# Example of a module that calls other modules (Child Modules)
# This module (Azure/compute/azurerm) internally calls a module named "os"
module "linux_servers" {
  source              = "Azure/compute/azurerm"
  version             = "5.3.0"
  resource_group_name = "example-rg"
  vm_os_simple        = "UbuntuServer"
  public_ip_dns       = ["linsimple"]
  vnet_subnet_id      = "subnet_id_placeholder"
  
  nb_instances = 1
  vm_hostname  = "myvm"
  
  tags = {
    environment = "dev"
  }
}

# Example of a standard module
module "network" {
  source              = "Azure/network/azurerm"
  version             = "5.3.0"
  resource_group_name = "example-rg"
  vnet_name           = "test-vnet"
  address_spaces      = ["10.0.0.0/16"]
  subnet_prefixes     = ["10.0.1.0/24"]
  subnet_names        = ["subnet1"]
  use_for_each        = true

  tags = {
    environment = "dev"
  }
}
