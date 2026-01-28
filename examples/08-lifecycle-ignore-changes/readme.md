# Ignore Changes Example

This example demonstrates how to inject `lifecycle { ignore_changes = [...] }` into a resource defined within a module.

This is useful when external processes (like Auto Scaling or Azure Policy) modify resource attributes that Terraform should not revert.

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
