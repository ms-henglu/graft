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

```bash
go install github.com/ms-henglu/graft@latest
```

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

---

## Architecture

`graft` operates as a **Build-Time Transpiler** for your infrastructure code.

```text
       [Upstream Registry]            [manifest.graft.hcl]
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
2.  **Patch**: It parses the `manifest.graft.hcl` and applies your modifications inside the vendored directory through three specific mechanisms:
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
# Requires manifest.graft.hcl to be present
graft build

[+] Reading manifest.graft.hcl...
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

*   **Behavior**:
    1.  **Vendor**: Copies modules to `.graft/build/`.
    2.  **Patch**: Applies `override` rules.
    3.  **Link**: Updates `.terraform/modules/modules.json` to point the module `Dir` to the local `.graft/build/` path.


### **`scaffold`**
Interactively scans your project modules and generates a `manifest.graft.hcl`.

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
✨ manifest.graft.hcl saved to ./manifest.graft.hcl
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

## Graft Configuration

The Graft configuration file (typically `manifest.graft.hcl`) acts as an enhanced version of [Terraform Override Files](https://developer.hashicorp.com/terraform/language/files/override). It retains standard Terraform behavior, while introducing powerful capabilities for adding, modifying, and removing infrastructure elements.


### Basic Structure

A Graft configuration uses `module` blocks to navigate the dependency tree and `override` blocks to apply changes.

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

Standard Terraform `override` files can only modify existing resources. Graft extends this by allowing you to define **new** top-level blocks (resources, outputs, providers, locals) inside an `override` block. These are appended to the target module.

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
