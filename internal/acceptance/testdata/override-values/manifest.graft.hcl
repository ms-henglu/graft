module "app" {
  override {
    resource "local_file" "config" {
      content = "overridden content"
    }
  }
}
