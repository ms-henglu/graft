module "app" {
  override {
    # Inject a new resource into the module
    resource "random_id" "injected" {
      byte_length = 4
    }

    # Inject a new output to expose the value
    output "injected_id" {
      value = random_id.injected.hex
    }
  }
}
