resource "local_file" "config" {
  content  = "test content"
  filename = "${path.module}/output.txt"
}

resource "local_file" "data" {
  content  = "data content"
  filename = "${path.module}/data.txt"
}

output "file_path" {
  value = local_file.config.filename
}
