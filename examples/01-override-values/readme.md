# Example 1: Override Hardcoded Values

This example demonstrates how to use Graft to override attributes in a third-party module.

## Scenario
We are using the `Azure/network/azurerm` module, and we want to enforce specific tags on the Virtual Network resource created by the module.

## Files
- `main.tf`: Standard Terraform configuration using the public registry module.
- `manifest.graft.hcl`: The Graft manifest defining the override rules.

## Usage
1. Run `terraform init` to download the upstream modules.
2. Run `graft build` to vendor the module and apply patches.
3. Run `terraform plan` to see that the tags have been applied.

```bash
# Initialize Terraform
terraform init

# Apply Graft patches
graft build
[+] Reading manifest.graft.hcl...
[+] Vendoring modules...
    - network (v5.3.0) [Cache Hit]
[+] Applying patches...
    - network: 1 override
[+] Linking modules...
✨ Build complete!

# Verify the changes
terraform plan

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # module.network.azurerm_virtual_network.vnet will be created
  + resource "azurerm_virtual_network" "vnet" {
      + address_space       = [
          + "10.0.0.0/16",
        ]
      + location            = "westus"
      + name                = "example-vnet"
      + resource_group_name = "example-rg"
      + tags                = {
          + "Environment" = "Production"
          + "Memo"        = "Overridden by Graft"
        }
      # ... other attributes
    }

Plan: 2 to add, 0 to change, 0 to destroy.
```
