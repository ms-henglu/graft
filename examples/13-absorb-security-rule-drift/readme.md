# Example 13: Absorb Multi-Level Nested Module Drift

This example demonstrates how `graft absorb` handles drift across a **multi-level nested module** hierarchy, with **nested block drift** (subnets, security rules) at each level.

## Architecture

```
root
├── azurerm_resource_group.test           ← Level 1 (root)
└── module.network                        ← Level 2 (child module)
    ├── azurerm_virtual_network.main        (VNet with inline subnet blocks)
    └── module.security                   ← Level 3 (grandchild module)
        └── azurerm_network_security_group.main  (NSG with security_rule blocks)
```

## Scenario

After deployment, external changes are made at **all three levels**:

### Level 1 — Root (Resource Group)
Tags changed via Azure CLI:
```bash
az group update --name graft-absorb-nested-test-rg \
  --tags environment=production project=graft owner=drifttest team=platform
```
Azure Policy also auto-added `Creator` and `DateCreated` tags.

### Level 2 — Child Module (VNet)
Tags changed and a new subnet added:
```bash
az network vnet update --resource-group graft-absorb-nested-test-rg \
  --name graft-absorb-nested-test-vnet \
  --set tags.environment=production tags.owner=drifttest

az network vnet subnet create --resource-group graft-absorb-nested-test-rg \
  --vnet-name graft-absorb-nested-test-vnet \
  --name db-subnet --address-prefixes 10.0.3.0/24 \
  --default-outbound-access false
```

### Level 3 — Grandchild Module (NSG)
A new security rule added and tags changed:
```bash
az network nsg rule create \
  --resource-group graft-absorb-nested-test-rg \
  --nsg-name graft-absorb-nested-test-nsg \
  --name allow-https --priority 200 \
  --destination-port-range 443 \
  --source-address-prefix "10.0.0.0/8" \
  --access Allow --protocol Tcp --direction Inbound

az network nsg update \
  --resource-group graft-absorb-nested-test-rg \
  --name graft-absorb-nested-test-nsg \
  --set tags.environment=production tags.owner=drifttest tags.compliance=required
```

## Files

- `main.tf`: Root Terraform configuration with a resource group and child module call.
- `modules/network/main.tf`: Child module with VNet (inline subnets) and grandchild module call.
- `modules/network/security/main.tf`: Grandchild module with NSG and security rules.
- `plan.json`: Terraform plan JSON output showing drift at all 3 levels.
- `absorb.graft.hcl`: The generated graft manifest (output of `graft absorb`).

## Usage

1. Deploy your infrastructure with `terraform apply`.
2. After drift occurs at multiple levels, generate a plan:

   ```bash
   terraform plan -out=tfplan
   terraform show -json tfplan > plan.json
   ```

3. Run `graft absorb`:

   ```bash
   graft absorb plan.json

   [+] Fetching providers schema...
   [+] Reading Terraform plan JSON...
   [+] Found 3 resource(s) with drift...
       - azurerm_resource_group.test
       - module.network.azurerm_virtual_network.main
       - module.network.module.security.azurerm_network_security_group.main
   [+] Generating manifest...
   ✨ Manifest saved to ./absorb.graft.hcl
   ```

4. Review the generated `absorb.graft.hcl`. Note how the manifest mirrors the module nesting:

   ```hcl
   # Root-level override for the resource group
   override {
     resource "azurerm_resource_group" "test" {
       tags = { ... }
     }
   }

   # Child module override for the VNet
   module "network" {
     override {
       resource "azurerm_virtual_network" "main" {
         subnet { ... }   # web-subnet
         subnet { ... }   # app-subnet
         subnet { ... }   # db-subnet (new)
         tags = { ... }
         _graft { remove = ["subnet"] }
       }
     }

     # Grandchild module override for the NSG
     module "security" {
       override {
         resource "azurerm_network_security_group" "main" {
           security_rule { ... }   # allow-ssh
           security_rule { ... }   # allow-https (new)
           tags = { ... }
           _graft { remove = ["security_rule"] }
         }
       }
     }
   }
   ```

   Key observations:
   - **Module nesting** is preserved: `module "network" { module "security" { ... } }`
   - Both `subnet` and `security_rule` use the "set of objects" (attribute-as-block) pattern, so `_graft { remove = [...] }` is used for full replacement
   - Tag drift is captured at all three levels

5. Build and verify:

   ```bash
   graft build
   terraform plan  # should show zero changes
   ```
