# Tests the workaround for fully replacing nested blocks:
# Use _graft to remove existing blocks, then define new ones

module "network" {
  override {
    resource "azurerm_virtual_network" "main" {
      # First, remove all existing subnet blocks
      _graft {
        remove = ["subnet"]
      }

      # Then define new subnet blocks to replace them
      subnet {
        name             = "new-subnet-1"
        address_prefixes = ["10.0.10.0/24"]
      }

      subnet {
        name             = "new-subnet-2"
        address_prefixes = ["10.0.20.0/24"]
      }
    }
  }
}
