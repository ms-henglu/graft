# Graft: The Overlay Engine for Terraform

Graft is a CLI tool that brings the Overlay Pattern (similar to Kustomize) to Terraform. It acts as a JIT (Just-In-Time) Compiler, allowing you to apply declarative patches to third-party modules at build time.

With Graft, you can treat upstream modules (e.g., from the Public Registry) as immutable base layers and inject your own logic on top—without the maintenance nightmare of forking.

## What Can Graft Do?

Graft allows you to surgically modify any Terraform module, even if you don't own the source code.

* **Override Hardcoded Values**
  Change any attribute inside a module that wasn't exposed as a variable.
  > Example: Force `vm_size = "Standard_D2s_v3"` on a module that hardcoded `"Standard_B1s"`.
* **Inject New Logic**
  Add new resources, data sources, or outputs to an existing module context.
  > Example: Inject a `azurerm_monitor_diagnostic_setting` or an extra `azurerm_role_assignment` into a community AKS module.
* **Remove Attributes & Resources**
  Delete unwanted resources or blocks from upstream modules—something native Terraform overrides cannot do.
  > Example: Remove a default `azurerm_network_security_rule` that violates your company policy.
* **Zero-Fork Maintenance**
  Keep your `main.tf` pointing to the official upstream version (e.g., v5.0.0). When upstream updates, you just bump the version; your patches are re-applied automatically.

---

## Installation

### Go Install

```bash
go install github.com/ms-henglu/graft@latest
```

### Manual Download

Download the appropriate binary for your platform from the [Releases](https://github.com/ms-henglu/graft/releases) page.

---

## Examples

Check out the [examples](./examples) directory for practical scenarios:

*   [Basic Overrides](./examples/01-override-values)
*   [Injecting New Logic](./examples/02-inject-new-logic)
*   [Remove Attributes & Resources](./examples/03-remove-attributes)
*   [Referencing Source Values](./examples/04-graft-source)
*   [Handling For-Each Resources](./examples/05-for-each-override)
*   [Multi-Layer Overrides](./examples/06-multi-layer-module-override)
*   [Scaffolding a Manifest](./examples/07-scaffold)
*   [Lifecycle: Ignore Changes](./examples/08-lifecycle-ignore-changes)
*   [Lifecycle: Prevent Destroy](./examples/09-lifecycle-prevent-destroy)
*   [Mark Values as Sensitive](./examples/10-mark-as-sensitive)

---

## Architecture

`graft` operates as a **Build-Time Transpiler** for your infrastructure code.

```text
       [Upstream Registry]            [Graft Manifest]
               |                            |
          (Downloads)                   (Defines)
               |                            |
               v                            v
      +------------------------------------------+
      |                graft CLI                 |
      +------------------------------------------+
               |                    |
          (Generates)          (Updates)
               |                    |
               v                    v
      [.graft/ Directory]   [.terraform/modules/modules.json]
                                    |
                                    v
                               [ Terraform ]
```

1.  **Vendor**: It downloads/copies the specified upstream module to a local `.graft/` directory.
2.  **Patch**: It parses the graft manifest (`*.graft.hcl`) and applies your modifications inside the vendored directory through three specific mechanisms:
    *   **Generation of `_graft_add.tf`**: New resources or blocks defined in your manifest are written to this file, effectively appending them to the module.
    *   **Generation of `_graft_override.tf`**: Attribute overrides are written to this file, leveraging Terraform's native override behavior to merge configurations.
    *   **Source Modification**: Changes that cannot be handled by overrides—such as removing resources or specific attributes—are applied directly to the source files within the vendored directory.
3.  **Link**: It updates `.terraform/modules/modules.json` to point module paths to the local `.graft/` directory. This allows `main.tf` to remain unchanged (pointing to the original Registry version) while Terraform executes your patched code.
4.  **Run**: You run `terraform plan` / `apply` as normal. To Terraform, it looks like it's running the registry module, but it's actually running your local graft.

---

## CLI Commands

### **`build`**
Vendors modules, applies local patches, and configures Terraform to use them using the "Linker Strategy".

```bash
# Vendors modules and redirects .terraform/modules/modules.json to point to them
# Auto-discovers all *.graft.hcl files in the current directory
graft build

[+] Reading 2 graft manifests...
[+] Vendoring modules...
    - linux_servers (v5.3.0) [Cache Hit]
    - linux_servers.os (Local)
    - network (v5.3.0) [Cache Hit]
[+] Applying patches...
    - linux_servers: 1 override
    - linux_servers.os: 1 override
    - network: 1 override
[+] Linking modules...
✨ Build complete!
```

You can also specify a single graft manifest explicitly:

```bash
graft build -m custom.graft.hcl
```

*   **Behavior**:
    1.  **Vendor**: Copies modules to `.graft/build/`.
    2.  **Patch**: Applies `override` rules.
    3.  **Link**: Updates `.terraform/modules/modules.json` to point the module `Dir` to the local `.graft/build/` path.


### **`scaffold`**
Interactively scans your project modules and generates a graft manifest (`scaffold.graft.hcl`).

It automatically discovers all module calls in your project, displays a tree view of the module hierarchy, and generates a boilerplate manifest with placeholder overrides for every resource found.

```bash
# Generate scaffold for all modules
graft scaffold

[+] Discovering modules in .terraform/modules...
root
├── linux_servers (registry: registry.terraform.io/Azure/compute/azurerm, 5.3.0)
│   ├── [18 resources]
│   └── linux_servers.os (local: ./os)
│       └── [0 resources]
└── network (registry: registry.terraform.io/Azure/network/azurerm, 5.3.0)
    └── [3 resources]
-> Tip: Run 'graft scaffold <MODULE_KEY>' to generate a manifest for a specific module.
-> Example: graft scaffold linux_servers.os
✨ Graft manifest saved to ./scaffold.graft.hcl
```

You can also scaffold for a specific module:

```bash
graft scaffold linux_servers.os
```

### **`clean`**
Cleans up graft artifacts and resets module redirection to upstream.

```bash
graft clean

[+] Removing build artifacts...
    - .graft directory
    - _graft_override.tf
[+] Resetting module links...
    - modules.json updated
-> Next Step: Run 'terraform init' to restore original paths.
✨ Clean complete!
```
*   **Behavior**:
    1.  Removes `.graft/` directory.
    2.  Removes `_graft_override.tf`.
    3.  Resets `.terraform/modules/modules.json` to point back to original sources.

---

## Graft Manifest

The graft manifest (typically `manifest.graft.hcl` or any `*.graft.hcl` file) acts as an enhanced version of [Terraform Override Files](https://developer.hashicorp.com/terraform/language/files/override). It retains standard Terraform behavior, while introducing powerful capabilities for adding, modifying, and removing infrastructure elements.

### Multi-File Support

Graft supports splitting your manifest across multiple `*.graft.hcl` files. When you run `graft build`, all graft manifests in the current directory are automatically discovered, sorted alphabetically, and deep-merged together.

**Merge Behavior**:
*   Files are processed in alphabetical order (e.g., `a.graft.hcl` before `b.graft.hcl`).
*   For conflicting attributes, **last write wins** (later files override earlier ones).
*   Blocks are merged by type and labels (e.g., two `resource "azurerm_virtual_network" "main"` blocks are merged, not duplicated).

### Basic Structure

A graft manifest uses `module` blocks to navigate the dependency tree and `override` blocks to apply changes.

```hcl
# filename: manifest.graft.hcl

# Root module override
override {

}

# Target a module by its name in the upstream source
module "networking" {
  # Apply overrides within this module's context
  override {
    resource "azurerm_virtual_network" "main" {
      tags = { Environment = "Production" }
    }
  }
}
```

### 1. Add New Resources

Standard Terraform `override` files can only modify existing resources. The graft manifest extends this by allowing you to define **new** top-level blocks (resources, outputs, providers, locals) inside an `override` block. These are appended to the target module.

```hcl
override {

  # This resource does not exist in the upstream module; it will be added.
  resource "azurerm_storage_account" "extra_logs" {
    name = "myapplogs"
  }

  # This output does not exist in the upstream module; it will be added.
  output "new_output" {
    value = "This is a new output added by graft"
  }

}
```

### 2. Remove Existing Resources/Blocks/Attributes

Graft introduces the `_graft` block to perform destructive actions, a capability not present in native Terraform overrides. You can remove attributes, nested blocks, or entire resources.

The `remove` argument accepts a list of strings, each representing the name of an attribute, nested block, or resource to be removed. It can also accept the special value `"self"` to indicate the entire block should be removed. You can also use dot notation to remove attributes inside nested blocks.

```hcl
override {

  resource "azurerm_virtual_network" "web" {
    # Remove specific attributes, nested blocks, or nested attributes (using dot notation)
    _graft {
      remove = ["description", "ingress", "timeouts.create"]
    }
  }

  module "legacy_db" {
    # Remove the entire module call
    _graft {
      remove = ["self"]
    }
  }

}
```

### 3. Using `graft.source` to Reference Original Values

In native Terraform overrides, defining an attribute completely replaces the original value (e.g., overriding `tags` wipes out the original tags).

Graft solves this by introducing the `graft.source` expression, which references the original value defined in the upstream module. This allows you to append to lists or merge maps instead of overwriting them.

```hcl
override {
  resource "azurerm_virtual_network" "app" {
    # Native override would delete original tags.
    # graft.source lets us keep them and append a new one.
    tags = merge(graft.source, { 
      "PatchedBy" = "Graft" 
    })
  }
}
```

This generates the following `_graft_override.tf` inside the vendored module:

```hcl
# filename: _graft_override.tf

resource "azurerm_virtual_network" "app" {
  tags = merge(
    {
      "Environment" = "Staging"
    }, 
    {
      "PatchedBy" = "Graft" 
    }
  )
}
```

We'll consider to add more advanced features in future releases, such as build-time variables and glob matching.

---

## Deep Dive

### Override Behavior Details

Graft extends Terraform's native override behavior to provide more intuitive merging for nested blocks.

#### Deep Merge for Nested Blocks

In native Terraform overrides, nested blocks are **replaced entirely**. This means if you override a single attribute in a nested block, you lose all other attributes from the source.

Graft performs **deep merge** on nested blocks, preserving original attributes while applying your overrides:

```hcl
# Source module (main.tf)
resource "azurerm_virtual_network" "main" {
  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }
}

# Graft manifest
override {
  resource "azurerm_virtual_network" "main" {
    subnet {
      default_outbound_access_enabled = false  # Add new attribute
    }
  }
}

# Generated _graft_override.tf (deep merged)
resource "azurerm_virtual_network" "main" {
  subnet {
    address_prefixes                = ["10.0.1.0/24"]  # Preserved from source
    name                            = "subnet1"        # Preserved from source
    default_outbound_access_enabled = false            # Added from override
  }
}
```

#### Dynamic Block Support

Graft also handles `dynamic` blocks correctly. When you override attributes in a dynamic block, Graft merges into the `content` block:

```hcl
# Source module with dynamic block
resource "azurerm_virtual_network" "main" {
  dynamic "subnet" {
    for_each = var.subnets
    content {
      name             = subnet.value.name
      address_prefixes = subnet.value.address_prefixes
    }
  }
}

# Graft manifest
override {
  resource "azurerm_virtual_network" "main" {
    subnet {
      default_outbound_access_enabled = false
    }
  }
}

# Generated _graft_override.tf
resource "azurerm_virtual_network" "main" {
  dynamic "subnet" {
    for_each = var.subnets
    content {
      address_prefixes                = subnet.value.address_prefixes
      name                            = subnet.value.name
      default_outbound_access_enabled = false  # Merged into content
    }
  }
}
```

#### Meta-Argument Blocks

Certain Terraform meta-argument blocks have special override semantics and are **not** deep merged by Graft:

- `lifecycle` - Merged on an argument-by-argument basis by Terraform (e.g., override `create_before_destroy` preserves existing `ignore_changes`)
- `connection` - Completely replaced by the override block
- `provisioner` - Override blocks replace all original provisioner blocks entirely

These blocks are passed through to the override file as-is, letting Terraform handle them according to its native rules.

#### Limitations

The current deep merge implementation has some limitations:

1. **No selective targeting**: When a resource has multiple nested blocks of the same type, the override is applied to **all** of them. There is currently no way to target a specific nested block by index or label.

2. **Single override block**: If your manifest contains multiple blocks of the same nested type within an override, only the **first** one is used. Additional blocks are ignored.

3. **Full replacement workaround**: If you need to completely replace all nested blocks (native Terraform override behavior), you can use `_graft` to remove the existing blocks first, then define the new blocks in the override:

   ```hcl
   override {
     resource "azurerm_virtual_network" "main" {
       # First, remove all existing subnet blocks
       _graft {
         remove = ["subnet"]
       }
       # Then define the new subnet blocks
       subnet {
         name             = "new-subnet-1"
         address_prefixes = ["10.0.10.0/24"]
       }
       subnet {
         name             = "new-subnet-2"
         address_prefixes = ["10.0.20.0/24"]
       }
     }
   }
   ```

### Why The Linker Strategy?

Why do we modify `.terraform/modules/modules.json` instead of generating a simple `override.tf` to point to the local path?

Native Terraform `override.tf` has a critical limitation: **Conflict between Version Constraints and Local Paths**.

1.  **The Scenario**: Your `main.tf` uses a public module:
    ```hcl
    module "network" {
      source  = "Azure/network/azurerm"
      version = "5.3.0"
    }
    ```
2.  **The Failed Override Approach**: If we generated an `override.tf` pointing to a local patched folder:
    ```hcl
    module "network" {
      source = "./.graft/network"
    }
    ```
    Terraform would fail with: `Error: Cannot apply a version constraint to module "network" because it has a relative local path`.
3.  **The Limitation**: You cannot "delete" or "unset" the `version` argument from `main.tf` using an override file.
4.  **The Solution**: The Linker Strategy. By updating Terraform's internal map (`modules.json`), we trick Terraform into believing it is satisfying the Registry requirement (source+version match), while physically loading the files from our local patched directory.
