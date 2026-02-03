resource "local_file" "config" {
  content         = "test content"
  filename        = "${path.module}/output.txt"
  file_permission = "0644"
}

output "file_path" {
  value = local_file.config.filename
}
