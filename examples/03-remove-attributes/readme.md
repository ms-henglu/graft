# Example 3: Surgical Removal

This example shows how to remove attributes, nested blocks, or entire resources using the special `_graft` block.

## Scenario
You need to remove a problematic attribute or an unwanted resource from an upstream module. In this example, we assume the upstream module defines a `service_endpoints` argument on the subnet resource, and we want to remove it to comply with our network policies.

## Files
- `main.tf`: Standard Terraform configuration.
- `manifest.graft.hcl`: Specifies the removal using `_graft`.

## Usage
1. Run `terraform init`.
2. Run `graft build`.
3. Run `terraform plan` to see the new resource and output.
4. Verify the attribute is removed (or the resource is gone).

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
      # dns_servers attribute is effectively removed or reset to default
      + dns_servers         = (known after apply) 
      + location            = "westus"
      # ...
      # tags are removed
    }

Plan: 2 to add, 0 to change, 0 to destroy.
```
