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
  name     = "graft-absorb-test-rg"
  location = "eastus"

  tags = {
    environment = "test"
    project     = "graft"
  }
}
