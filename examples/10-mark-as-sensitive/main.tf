module "app" {
  source = "./modules/sensitive-module"
}

output "app_password" {
  value     = module.app.db_password
}
