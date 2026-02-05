# Manifest to test deep merge of nested blocks
module "network" {
  override {
    resource "azurerm_virtual_network" "main" {
      # This should merge into ALL subnet blocks (static and dynamic)
      subnet {
        default_outbound_access_enabled = false
      }
    }
  }
}
