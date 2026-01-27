module "network" {
  override {
    resource "azurerm_virtual_network" "vnet" {
      # 'graft.source' represents the value of 'tags' from the original module code.
      # We use merge() to combine the original tags with our new ones.
      tags = merge(graft.source, {
        "Owner"     = "DevOps Team"
        "ManagedBy" = "Graft"
      })
    }
  }
}
