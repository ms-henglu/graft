module "network" {
  override {
   
    resource "azurerm_virtual_network" "vnet" {
      _graft {
        # Remove the 'dns_servers' and 'tags' attributes if it exist
        remove = ["dns_servers", "tags"]
      }
    }

    # Example of removing an entire resource
    # resource "azurerm_network_security_group" "default" {
    #   _graft {
    #     remove = ["self"]
    #   }
    # }
  }
}
