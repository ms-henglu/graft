# Graft manifest for nested block override example
# This demonstrates deep merge for both static and dynamic nested blocks

module "network" {
  override {
    resource "azurerm_virtual_network" "main" {
      # This attribute will be merged into ALL subnet blocks
      # (both static and dynamic) while preserving their original values
      subnet {
        default_outbound_access_enabled = false
      }
    }
  }
}
