# Root level override
override {
  resource "azurerm_resource_group" "test" {
    tags = {
      "ManagedBy"   = "Graft"
      "Environment" = "Prod"
    }
  }
}

# 1. Patching the 'linux_servers' module (Azure/compute/azurerm)
module "linux_servers" {
  override {
    # 1.1 Override the Virtual Machine resource defined directly inside this module
    # Note: 'azurerm_virtual_machine' is the resource type used inside that module version
    resource "azurerm_virtual_machine" "vm_linux" {
      # Forcing a specific VM size, ignoring the module input or default
      vm_size = "Standard_B4ms"

      tags = {
        "Role"    = "Headless"
        "Patched" = "True"
      }
    }

    # 1.2 Override the Network Security Group also in this module
    resource "azurerm_network_security_group" "vm" {
      tags = {
        "SecurityLevel" = "Critical"
      }
    }
  }

  # 1.3 Patching the nested 'os' module inside 'linux_servers'
  # This shows we can navigate down the module tree
  module "os" {
    override {
      # Override the default value of a variable defined in the child (os) module
      variable "standard_os" {
        default = {
          "UbuntuServer" = "Canonical,UbuntuServer,22.04-LTS-Hardened"
        }
      }
    }
  }
}

# 2. Patching the 'network' module (Azure/network/azurerm)
module "network" {
  override {
    resource "azurerm_virtual_network" "vnet" {
      # Injecting values that might override defaults
      dns_servers = ["8.8.8.8", "8.8.4.4"]

      tags = {
        "CostCenter" = "Infra"
      }
    }
  }
}
