# Override a resource in the root module
override {
  resource "local_file" "root_config" {
    content = "overridden root content"
  }
}
