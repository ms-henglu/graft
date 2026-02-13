# How to Use `graft absorb`

`graft absorb` automatically generates a graft manifest from Terraform plan drift. Instead of manually writing override blocks when external changes make your Terraform plan show unwanted updates, `graft absorb` reads the plan output and produces the manifest for you.

## When to Use

Use `graft absorb` when:

- Azure Policy adds tags to your resources (e.g., `Creator`, `DateCreated`)
- A team member changes settings directly in the Azure portal
- An external automation script modifies resource attributes
- Security rules, subnets, or other blocks are added outside Terraform
- You want to accept the current state as the new desired state without reverting

## Prerequisites

- [Graft CLI](https://github.com/ms-henglu/graft) installed
- [Terraform](https://developer.hashicorp.com/terraform/downloads) installed and initialized in your working directory
- An active Terraform deployment with detected drift

## Step-by-Step Walkthrough

This guide uses a real-world scenario: a public Azure module (`Azure/network/azurerm`) where tags, VNet settings, and subnet configurations drift after deployment.

### 1. Start with a Terraform configuration

Here's a typical setup using a public registry module:

```hcl
# main.tf
resource "azurerm_resource_group" "example" {
  name     = "graft-absorb-module-rg"
  location = "eastus"

  tags = {
    environment = "dev"
    project     = "graft"
  }
}

module "network" {
  source  = "Azure/network/azurerm"
  version = "5.3.0"

  resource_group_name = azurerm_resource_group.example.name
  use_for_each        = true
  vnet_name           = "graft-example-vnet"
  vnet_location       = azurerm_resource_group.example.location
  address_space       = ["10.0.0.0/16"]

  subnet_names    = ["web-subnet", "app-subnet"]
  subnet_prefixes = ["10.0.1.0/24", "10.0.2.0/24"]

  tags = {
    environment = "dev"
    managed_by  = "terraform"
  }
}
```

Deploy it:

```bash
terraform init
terraform apply
```

### 2. Drift happens

Over time, external changes accumulate. For example:

```bash
# Azure Policy adds tags to the resource group
# A team member promotes the environment to production
az group update --name graft-absorb-module-rg \
  --tags environment=production project=graft owner=platform-team

# VNet tags are updated to match
az network vnet update --resource-group graft-absorb-module-rg \
  --name graft-example-vnet \
  --set tags.environment=production tags.owner=platform-team

# A service endpoint is added to the web subnet
az network vnet subnet update --resource-group graft-absorb-module-rg \
  --vnet-name graft-example-vnet --name web-subnet \
  --service-endpoints Microsoft.Web \
  --default-outbound-access false
```

### 3. Detect drift with `terraform plan`

```bash
terraform plan -out=tfplan
```

Terraform shows it wants to **revert** all external changes:

```
  # azurerm_resource_group.example will be updated in-place
  ~ resource "azurerm_resource_group" "example" {
      ~ tags     = {
          - "Creator"     = "admin@contoso.com" -> null
          - "DateCreated" = "2026-01-15T09:22:31Z" -> null
          ~ "environment" = "production" -> "dev"
          - "owner"       = "platform-team" -> null
            # (1 unchanged element hidden)
        }
    }

  # module.network.azurerm_virtual_network.vnet will be updated in-place
  ~ resource "azurerm_virtual_network" "vnet" {
      ~ tags = {
          ~ "environment" = "production" -> "dev"
          - "owner"       = "platform-team" -> null
        }
    }

  # module.network.azurerm_subnet.subnet_for_each["web-subnet"] will be updated in-place
  ~ resource "azurerm_subnet" "subnet_for_each" {
      ~ default_outbound_access_enabled = false -> true
      ~ service_endpoints               = [
          - "Microsoft.Web",
        ]
    }

Plan: 0 to add, 3 to change, 0 to destroy.
```

You want to **accept** these changes, not revert them. But the VNet and subnet resources are inside a public module — you'd have to dig through its source code to find the right variables to set, and some attributes (like `service_endpoints` on individual subnets) may not even be exposed as variables.

### 4. Export the plan as JSON

```bash
terraform show -json tfplan > plan.json
```

This produces a JSON file that `graft absorb` can parse.

### 5. Run `graft absorb`

```bash
graft absorb plan.json
```

Output:

```
[+] Fetching providers schema...
[+] Reading Terraform plan JSON...
[+] Found 3 resource(s) with drift...
    - azurerm_resource_group.example
    - module.network.azurerm_virtual_network.vnet
    - module.network.azurerm_subnet.subnet_for_each["web-subnet"]
[+] Generating manifest...
✨ Manifest saved to ./absorb.graft.hcl
```

`graft absorb` automatically:
1. Parses the plan JSON to find resources with `update` actions
2. Fetches the providers schema (runs `terraform providers schema -json`) to filter out computed-only attributes
3. Compares the remote state (`before`) with the desired config (`after`) to extract drifted attributes
4. Generates an `absorb.graft.hcl` manifest with override blocks

### 6. Review the generated manifest

```hcl
# absorb.graft.hcl — Generated by graft absorb

override {
  # Absorb drift for: azurerm_resource_group.example
  resource "azurerm_resource_group" "example" {
    tags = {
      Creator     = "admin@contoso.com"
      DateCreated = "2026-01-15T09:22:31Z"
      environment = "production"
      owner       = "platform-team"
      project     = "graft"
    }
  }
}

module "network" {
  override {
    # Absorb drift for: module.network.azurerm_virtual_network.vnet
    resource "azurerm_virtual_network" "vnet" {
      tags = {
        environment = "production"
        managed_by  = "terraform"
        owner       = "platform-team"
      }
    }

    # Absorb drift for: module.network.azurerm_subnet.subnet_for_each["web-subnet"]
    resource "azurerm_subnet" "subnet_for_each" {
      default_outbound_access_enabled = lookup({
        "web-subnet" = false
      }, each.key, graft.source)
      service_endpoints = lookup({
        "web-subnet" = ["Microsoft.Web"]
      }, each.key, graft.source)
    }
  }
}
```

Notice how the manifest handles different situations:

| Situation | How it's handled |
|-----------|-----------------|
| Root-level resource | Simple `override { resource ... }` block |
| Resource inside a module | Wrapped in `module "network" { override { ... } }` |
| `for_each` resource with partial drift | Uses `lookup(map, each.key, graft.source)` so only drifted instances are overridden |

### 7. Build and verify

```bash
graft build
terraform plan
```

Expected result:
```
No changes. Your infrastructure matches the configuration.
```

The drift is now absorbed — your Terraform config accepts the external changes as the desired state, without forking the public module.

## Command Reference

```
graft absorb [flags] <plan.json>
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `absorb.graft.hcl` | Output file path for the generated manifest |
| `--providers-schema` | `-p` | *(auto-fetched)* | Path to a pre-generated providers schema JSON |

### Providers schema

By default, `graft absorb` runs `terraform providers schema -json` automatically to fetch schema information. This schema is used to filter out **computed-only attributes** from the generated manifest, producing cleaner output.

If Terraform isn't initialized or you want to skip the auto-fetch (e.g., in CI pipelines), provide a pre-generated schema:

```bash
# Generate once
terraform providers schema -json > providers.json

# Use it with absorb
graft absorb -p providers.json plan.json
```

If no schema is available, `graft absorb` still works but may include some computed attributes that you'll want to manually remove.

## What Gets Absorbed

`graft absorb` handles the following types of drift:

| Drift Type | Example | Generated Pattern |
|-----------|---------|-------------------|
| Root-level attributes | Tags, location | Direct attribute override |
| Single nested blocks | `os_disk` on a VM | Block override |
| Multiple sibling blocks | `security_rule`, `subnet` | Block override + `_graft { remove }` |
| Module resources | Resources inside `module "x"` | `module "x" { override { ... } }` |
| `count`-indexed resources | `resource.name[0]`, `[1]` | `lookup(map, count.index, graft.source)` |
| `for_each`-indexed resources | `resource.name["key"]` | `lookup(map, each.key, graft.source)` |
| Block drift on indexed resources | Different blocks per instance | `dynamic` block with `lookup()` |

For the full support scope and edge cases, see the [design document](../docs/design/absorb-support-scope.md).

## Examples

| # | Example | Drift Type |
|---|---------|-----------|
| [12](../examples/12-absorb-tag-drift) | Absorb Tag Drift | Simple tag drift on a single resource |
| [13](../examples/13-absorb-security-rule-drift) | Absorb Security Rule Drift | Nested module drift with block-type attributes |
| [14](../examples/14-absorb-indexed-drift) | Absorb Indexed Resource Drift | `count` and `for_each` with `lookup()` |
| [15](../examples/15-absorb-indexed-block-drift) | Absorb Indexed Block Drift | `dynamic` blocks on indexed resources |
| [16](../examples/16-absorb-public-module-drift) | Absorb Public Module Drift | End-to-end walkthrough with a public registry module |

