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

# count-indexed resource
resource "azurerm_resource_group" "env" {
  count    = 2
  name     = "graft-absorb-${count.index}-rg"
  location = "eastus"

  tags = {
    environment = "test"
    project     = "graft"
  }
}

# for_each-indexed resource
resource "azurerm_resource_group" "team" {
  for_each = toset(["web", "api"])
  name     = "graft-absorb-${each.key}-rg"
  location = "eastus"

  tags = {
    team    = each.key
    project = "graft"
  }
}
