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

# Use a local network module
module "network" {
  source = "./modules/network"

  vnet_name = "my-vnet"
  location  = "eastus"

  # Additional subnets created via dynamic block
  extra_subnets = [
    {
      name             = "extra-subnet-1"
      address_prefixes = ["10.0.10.0/24"]
    }
  ]
}
