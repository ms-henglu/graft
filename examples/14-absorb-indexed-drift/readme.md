# Example 14: Absorb Indexed Resource Drift (`count` & `for_each`)

This example demonstrates how `graft absorb` handles drift on resources created with `count` or `for_each`. Instead of generating one override block per instance, `graft absorb` groups all instances of the same resource and uses a `lookup()` expression to select the correct override per index.

## Scenario

You have two sets of Azure Resource Groups:

1. **`count`-indexed**: Two resource groups created with `count = 2`, both tagged `environment = "test"`. Over time the tags drift differently:
   - Instance `[0]`: tags changed to `environment = "production"`, `owner = "drifttest"` added.
   - Instance `[1]`: tags changed to `environment = "staging"`, `owner = "drifttest"` added.

2. **`for_each`-indexed**: Two resource groups created with `for_each = toset(["web", "api"])`, both in `location = "eastus"`. The locations drift:
   - Instance `["api"]`: location changed to `"westus"`.
   - Instance `["web"]`: location changed to `"centralus"`.

Running `terraform plan` shows four resources with drift.

## Key Concepts

### `lookup()` with `count.index`

When instances of a `count` resource have **different** drifted values, `graft absorb` generates a `lookup()` expression keyed by `count.index`:

```hcl
tags = lookup({
  0 = { environment = "production", owner = "drifttest", project = "graft" }
  1 = { environment = "staging",    owner = "drifttest", project = "graft" }
}, count.index, graft.source)
```

### `lookup()` with `each.key`

For `for_each` resources, the same pattern uses `each.key` with quoted string keys:

```hcl
location = lookup({
  "api" = "westus"
  "web" = "centralus"
}, each.key, graft.source)
```

### `graft.source` fallback

The third argument, `graft.source`, is the fallback. If an instance has no drift for a given attribute (e.g., only some instances drifted), `graft.source` preserves the original config value for the un-drifted instances.

## Files

- `main.tf`: Terraform configuration with `count` and `for_each` resource groups.
- `plan.json`: Terraform plan JSON output showing drift on all four instances.
- `absorb.graft.hcl`: The generated graft manifest (output of `graft absorb`).

## Usage

1. Deploy your infrastructure with `terraform apply`.
2. After drift occurs, generate a plan:

   ```bash
   terraform plan -out=tfplan
   terraform show -json tfplan > plan.json
   ```

3. Run `graft absorb` to generate a manifest from the drift:

   ```bash
   graft absorb plan.json

   [+] Fetching providers schema...
   [+] Reading Terraform plan JSON...
   [+] Found 4 resource(s) with drift...
       - azurerm_resource_group.env[0]
       - azurerm_resource_group.env[1]
       - azurerm_resource_group.team["api"]
       - azurerm_resource_group.team["web"]
   [+] Generating manifest...
   ✨ Manifest saved to ./absorb.graft.hcl
   ```

4. Review the generated `absorb.graft.hcl`:

   ```hcl
   override {
     # Absorb drift for: azurerm_resource_group.env[0], azurerm_resource_group.env[1]
     resource "azurerm_resource_group" "env" {
       tags = lookup({
         0 = {
           environment = "production"
           owner       = "drifttest"
           project     = "graft"
         }
         1 = {
           environment = "staging"
           owner       = "drifttest"
           project     = "graft"
         }
       }, count.index, graft.source)
     }

     # Absorb drift for: azurerm_resource_group.team["api"], azurerm_resource_group.team["web"]
     resource "azurerm_resource_group" "team" {
       location = lookup({
         "api" = "westus"
         "web" = "centralus"
       }, each.key, graft.source)
     }
   }
   ```

5. Build and verify:

   ```bash
   graft build
   terraform plan  # should show zero changes
   ```
