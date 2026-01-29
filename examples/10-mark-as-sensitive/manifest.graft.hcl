module "app" {
  override {
    output "db_password" {
      # Use sensitive() to mask the value.
      value     = sensitive(graft.source)
      sensitive = true
    }

    output "connection_string" {
      value     = sensitive(graft.source)
      sensitive = true
    }
  }
}

override {
  output "app_password" {
    sensitive = true
  }
}