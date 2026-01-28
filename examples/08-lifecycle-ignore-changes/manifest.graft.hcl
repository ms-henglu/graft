module "example" {
  override {
    resource "azurerm_resource_group" "this" {
      lifecycle {
        ignore_changes = [tags]
      }
    }
  }
}
