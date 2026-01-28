# Prevent Destroy Example

This example shows how to add `lifecycle { prevent_destroy = true }` to a resource inside a module.

This is critical for stateful resources (like databases or storage) to prevent accidental deletion, even if the module author didn't enable it by default.

## Usage

1. Initialize Terraform:
   ```bash
   terraform init
   ```

2. Generate the patched code:
   ```bash
   graft build
   ```

3. Plan to see the changes:
   ```bash
   terraform plan
   ```
