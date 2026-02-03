# Tests that Graft can remove specific attributes from resources using _graft.remove.

expected ".graft/build/app/main.tf" {
  content {
    resource "local_file" "config" {
      content  = "test content"
      filename = "${path.module}/output.txt"
    }

    output "file_path" {
      value = local_file.config.filename
    }
  }
  not_contains = ["file_permission"]
}

expected ".terraform/modules/modules.json" {
  contains = [".graft/build/app"]
}
