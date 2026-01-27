# Example 5: Conditional Overrides with `for_each`

This example demonstrates how to use `graft.source` combined with `each.key` to selectively apply overrides to specific instances of a resource created with `for_each`.

## Scenario
The upstream module creates multiple subnets using `for_each`. We want to modify `enforce_private_link_endpoint_network_policies` ONLY for `subnet1` to be `true`, while keeping the default behavior (upstream logic) for all other subnets.

## Files
- `main.tf`: Uses a module to create multiple subnets (`subnet1`, `subnet2`).
- `manifest.graft.hcl`: Uses conditional logic with `graft.source` to toggle the setting based on `each.key`.

## Usage
1. Run `terraform init` to download the upstream modules.
2. Run `graft build` to vendor the module and apply patches.
3. Run `terraform plan` to see the result.

```bash
terraform plan

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # module.network.azurerm_subnet.subnet_for_each["subnet1"] will be created
  + resource "azurerm_subnet" "subnet_for_each" {
      + name                                           = "subnet1"
      # For subnet1, we overrode this to true
      + enforce_private_link_endpoint_network_policies = true
      # ...
    }

  # module.network.azurerm_subnet.subnet_for_each["subnet2"] will be created
  + resource "azurerm_subnet" "subnet_for_each" {
      + name                                           = "subnet2"
      # For subnet2, kept original logic (false, default)
      + enforce_private_link_endpoint_network_policies = false
      # ...
    }

  # ...
  
Plan: 3 to add, 0 to change, 0 to destroy.
```
