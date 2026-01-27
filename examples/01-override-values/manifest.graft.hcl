module "network" {
  # Target the azurerm_virtual_network resource usually named "vnet" in this module
  override {
    resource "azurerm_virtual_network" "vnet" {
      # This will override any tags defined in the module with this specific set
      tags = {
        Environment = "Production"
        Memo        = "Overridden by Graft"
      }
    }
  }
}
