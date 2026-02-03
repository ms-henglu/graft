module "labels" {
  override {
    # Override the delimiter used in the label
    locals {
      delimiter = "_"
    }
  }
}
