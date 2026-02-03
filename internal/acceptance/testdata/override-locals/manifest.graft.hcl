module "app" {
  override {
    locals {
      environment = "production"
      extra_tags = {
        ManagedBy = "Graft"
      }
    }
  }
}
