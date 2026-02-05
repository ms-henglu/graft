# Example 11: Nested Block Override (Static and Dynamic)

This example demonstrates how Graft performs **deep merge** on nested blocks, allowing you to add or override attributes while preserving the original values from the source module.

## Scenario

You're using a network module that defines:
- Two **static** subnet blocks with hardcoded values
- One **dynamic** subnet block that creates subnets from a variable

You want to add `default_outbound_access_enabled = false` to ALL subnets without losing their original `name` and `address_prefixes` attributes.

## Files

- `main.tf`: Root module that uses the local network module
- `manifest.graft.hcl`: Graft manifest with the nested block override
- `modules/network/main.tf`: Network module with static and dynamic subnet blocks

## Usage

```bash
# Apply Graft patches
graft build

[+] Reading 1 graft manifests...
[+] Vendoring modules...
    - network (Local)
[+] Applying patches...
    - network: 1 override
[+] Linking modules...
✨ Build complete!

# Verify the changes
terraform plan

  # module.network.azurerm_virtual_network.main will be created
  + resource "azurerm_virtual_network" "main" {
      + address_space       = [
          + "10.0.0.0/16",
        ]
      + location            = "eastus"
      + name                = "my-vnet"
      + resource_group_name = "my-rg"
      + subnet              = [
          + {
              + address_prefixes                = [
                  + "10.0.1.0/24",
                ]
              + default_outbound_access_enabled = false
              + name                            = "subnet1"
              # ...
            },
          + {
              + address_prefixes                = [
                  + "10.0.2.0/24",
                ]
              + default_outbound_access_enabled = false
              + name                            = "subnet2"
              # ...
            },
          + {
              + address_prefixes                = [
                  + "10.0.10.0/24",
                ]
              + default_outbound_access_enabled = false
              + name                            = "extra-subnet-1"
              # ...
            },
        ]
    }

Plan: 1 to add, 0 to change, 0 to destroy.
```

All three subnets (two static, one from dynamic block) have `default_outbound_access_enabled = false` applied.

## Generated Override File

```bash
cat .graft/build/network/_graft_override.tf
```

```hcl
resource "azurerm_virtual_network" "main" {
  subnet {
    address_prefixes                = ["10.0.1.0/24"]
    name                            = "subnet1"
    default_outbound_access_enabled = false
  }
  subnet {
    address_prefixes                = ["10.0.2.0/24"]
    name                            = "subnet2"
    default_outbound_access_enabled = false
  }
  dynamic "subnet" {
    for_each = var.extra_subnets
    content {
      address_prefixes                = subnet.value.address_prefixes
      name                            = subnet.value.name
      default_outbound_access_enabled = false
    }
  }
}
```
