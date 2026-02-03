resource "local_file" "config" {
  content  = "original content"
  filename = "${path.module}/output.txt"
}

output "file_path" {
  value = local_file.config.filename
}
