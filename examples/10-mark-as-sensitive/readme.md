# Example: Mark Values as Sensitive

This example demonstrates how to use `graft` to retrofit security best practices onto third-party modules that may be leaking sensitive data.

## Scenario

We have a module `sensitive-module` that generates a `random_string` but forgets to mark the output as `sensitive = true`. This causes the password to be displayed in plain text in logs.

## The Solution

We use `graft` to override the output definition in the module. We wrap the original value (accessed via `graft.source`) with Terraform's built-in `sensitive()` function and enforce `sensitive = true`.

### Manifest

```hcl
module "app" {
  override {
    output "db_password" {
      # graft.source resolves to the original expression
      value     = sensitive(graft.source)
      sensitive = true
    }
  }
}
```

## Running the Example

1. Initialize Terraform:
   ```bash
   terraform init
   ```

2. Build the graft (vendor and patch):
   ```bash
   graft build
   ```

3. Run Terraform Plan/Apply:
   ```bash
   terraform apply
   ```

   You will see that `module.app` outputs are now redacted `(sensitive value)`, whereas the original module would have shown them.

   **Note:** Because the module output is now sensitive, you must also treat it as sensitive in your root `main.tf` outputs, or Terraform will return an error protecting you from accidental exposure.
