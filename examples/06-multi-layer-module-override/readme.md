# Multi-layer Module Override Example

This example demonstrates how `graft` can override resources and variables at multiple levels of the module hierarchy:

1.  **Root Level**: Overriding the `azurerm_resource_group` defined in `main.tf`.
2.  **Child Module Level**: Overriding resources (`azurerm_virtual_machine`, `azurerm_network_security_group`) inside the `linux_servers` module.
3.  **Grandchild Module Level**: Overriding a variable inside the nested `os` module (which is called by `linux_servers`).

## Key Concepts

- `override { ... }` block at the root level targets resources in the root module.
- `module "name" { ... }` allows navigation into a specific module call.
- Inside a `module` block, you can have another `override { ... }` block to target resources/variables in that module.
- You can also nest `module` blocks further (`module "linux_servers" { module "os" { ... } }`) to reach deeply nested modules.

## Files

- `main.tf`: The base Terraform configuration using `Azure/compute/azurerm` and `Azure/network/azurerm` modules.
- `manifest.graft.hcl`: The Graft manifest defining the overrides.

## Usage
1. Run `terraform init`.
2. Run `graft build` to apply the patch.
3. Run `terraform apply` to see the new resource and output.
4. Verify the changes with `terraform plan`.


## Expected Output

You should see the following changes in the plan, reflecting the overrides defined in `manifest.graft.hcl`:

```hcl
Terraform will perform the following actions:

  # azurerm_resource_group.test will be created
  + resource "azurerm_resource_group" "test" {
      + location = "westus"
      + name     = "example-rg-logic"
      + tags     = {
          + "Environment" = "Prod"    # <--- Root level override applied
          + "ManagedBy"   = "Graft"   # <--- Root level override applied
        }
    }

  # module.linux_servers.azurerm_network_security_group.vm[0] will be created
  + resource "azurerm_network_security_group" "vm" {
      + tags                = {
          + "SecurityLevel" = "Critical" # <--- Module override applied
        }
      # ...
    }

  # module.linux_servers.azurerm_virtual_machine.vm_linux[0] will be created
  + resource "azurerm_virtual_machine" "vm_linux" {
      + vm_size                          = "Standard_B4ms" # <--- Value forced by Graft
      + tags                             = {
          + "Patched" = "True"      # <--- Module override applied
          + "Role"    = "Headless"  # <--- Module override applied
        }

      + storage_image_reference {
            id        = null
          + offer     = "UbuntuServer"
          + publisher = "Canonical"
          + sku       = "22.04-LTS-Hardened" # <--- Grandchild module override applied
          + version   = "latest"
        }
      # ...
    }

  # module.network.azurerm_virtual_network.vnet will be created
  + resource "azurerm_virtual_network" "vnet" {
      + dns_servers         = [
          + "8.8.8.8",  # <--- List override applied
          + "8.8.4.4",
        ]
      + tags                = {
          + "CostCenter" = "Infra" # <--- Module override applied
        }
      # ...
    }
```
