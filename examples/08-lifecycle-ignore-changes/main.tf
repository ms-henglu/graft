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

module "example" {
  source              = "Azure/avm-res-resources-resourcegroup/azurerm"
  version             = "0.1.0"
  location            = "East US"
  name                = "example-rg"
}
