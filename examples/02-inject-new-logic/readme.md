# Example 2: Inject New Logic

This example demonstrates how to inject new resources and outputs into a third-party module context.

## Scenario
You want to extend a module by adding resources that are tightly coupled with the module's logic, or expose internal values via new outputs, without forking the upstream code.

In this example, we inject a `random_id` resource and a corresponding `output` into the `network` module.

## Files
- `main.tf`: Standard Terraform configuration.
- `manifest.graft.hcl`: Defines the new resource and output to be injected.

## Usage
1. Run `terraform init`.
2. Run `graft build` to apply the patch.
3. Run `terraform apply` to see the new resource and output.
4. Verify the changes with `terraform plan`.

### Expected Plan Output
```
Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # module.network.random_id.graft_logic will be created
  + resource "random_id" "graft_logic" {
      + byte_length = 4
      + hex         = (known after apply)
      # ... other attributes
    }

  # ... other resources (vnet, subnet)

Plan: 3 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + injected_value = (known after apply)
```
