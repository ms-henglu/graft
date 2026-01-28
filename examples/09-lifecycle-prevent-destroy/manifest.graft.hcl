module "example" {
  override {
    resource "azurerm_resource_group" "this" {
      lifecycle {
        prevent_destroy = true
      }
    }
  }
}
