module "network" {
  override {
    # The upstream module uses "azurerm_subnet" "subnet_for_each" 
    resource "azurerm_subnet" "subnet_for_each" {
      
      # Use conditional logic to change behavior based on the key
      # We target 'enforce_private_link_endpoint_network_policies' which exists in the source.
      # We force it to true for subnet1, and keep original logic (graft.source) for others.
      enforce_private_link_endpoint_network_policies = each.key == "subnet1" ? true : graft.source
    }
  }
}
