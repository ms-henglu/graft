# Tests removing all nested blocks (both static and dynamic) of a type

module "network" {
  override {
    resource "azurerm_virtual_network" "main" {
      # Remove all subnet blocks - both static and dynamic
      _graft {
        remove = ["subnet"]
      }
    }
  }
}
