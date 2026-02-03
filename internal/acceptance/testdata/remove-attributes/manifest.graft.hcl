module "app" {
  override {
    resource "local_file" "config" {
      _graft {
        # Remove the 'file_permission' attribute
        remove = ["file_permission"]
      }
    }
  }
}
