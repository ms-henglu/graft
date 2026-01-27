# Example 4: Using `graft.source`

This example demonstrates how to reference the original value of an attribute using the `graft.source` variable. This is useful for appending to lists or merging maps without losing the upstream configuration.

## Scenario
The upstream module applies some default tags to resources. We want to add our own custom tags (e.g., `Owner`, `CostCenter`) without wiping out the module's default tags. Standard Terraform overrides replace the value entirely, but `graft.source` allows us to merge.

## Files
- `main.tf`: Standard Terraform configuration.
- `manifest.graft.hcl`: Uses `merge(graft.source, ...)` to combine tags.

## Usage
1. Run `terraform init`.
2. Run `graft build`.
3. Run `terraform plan`.
4. Inspect the plan to see that both original and new tags are present.

```bash
terraform plan

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

# ...
  # module.network.azurerm_virtual_network.vnet will be created
  + resource "azurerm_virtual_network" "vnet" {
      + address_space       = [
          + "10.0.0.0/16",
        ]
      + location            = "westus"
      + name                = "example-vnet-source"
      # ...
      # Merged tags
      + tags                = {
          + "ManagedBy"     = "Graft"
          + "ModuleDefault" = "true"
          + "Owner"         = "DevOps Team"
        }
    }

Plan: 2 to add, 0 to change, 0 to destroy.
```
