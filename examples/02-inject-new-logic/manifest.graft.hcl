module "network" {
  override {
    # Inject a new resource into the module
    resource "random_id" "graft_logic" {
      byte_length = 4
    }

    # Inject a new output to expose the value
    output "graft_id" {
      value = random_id.graft_logic.hex
    }
  }
}
