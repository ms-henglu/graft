# `graft absorb` — Support Scope & Limitations

`graft absorb` reads a Terraform plan JSON file, identifies resources with
**update** actions (drift), and generates an `absorb.graft.hcl` manifest that
aligns the configuration with the current remote state.

---

## Supported Drift Types

### 1. Root-Level Attribute Drift

Any change to a top-level attribute — primitives (`string`, `bool`, `number`),
lists/sets, or maps (`tags`).

- **Primitives:** emitted as `attribute = "new_value"`. Only changed attributes appear.
- **Lists/Sets:** the **full collection** is emitted (Terraform treats them atomically).
- **Maps:** the **full map** is emitted (e.g., entire `tags` block).
- Multiple attributes changing on the same resource are combined in one override block.

✅ Fully supported.

---

### 2. Single Nested Block Drift

Attribute changes inside a single nested block (e.g., `os_disk`, `blob_properties`,
`ip_configuration`), at any nesting depth.

- With provider schema: `deepDiffBlock` produces a **minimal diff** — only the
  changed attributes along the block path are emitted.
- Works recursively through 2-layer, 3-layer, and deeper block hierarchies.
- Cross-layer drift (changes at both outer and inner levels) is handled correctly.
- Attributes of any type (primitive, list, map) inside blocks are supported.

✅ Fully supported (with schema). Without schema, the entire block subtree is emitted.

---

### 3. Multiple Sibling Blocks

Resources with multiple blocks of the same type (e.g., `security_rule` in an Azure NSG, multiple `ip_configuration` blocks in an Azure VM, multiple `subnet` blocks in an Azure Virtual Network).

- The **entire array** of blocks is captured — no per-element diffing.
- A `_graft { remove = ["block_type"] }` directive is added so `graft build`
  replaces all original blocks with the overridden set.
- Nested multi-blocks inside a single parent use dotted-path removals
  (e.g., `remove = ["backend_http_settings.connection_draining"]`).

✅ Supported. Full array capture is by design.

---

### 4. Module Resources

Drift in resources inside modules (including nested modules).

- Module path is extracted from the resource address in the plan JSON.
- Overrides are nested inside `module "name" { ... }` blocks matching the hierarchy.

✅ Fully supported.

---

## Out of Scope

The following cases produce an `update` action in the plan but are **not**
absorb targets. They represent the config wanting to *add* something the cloud
doesn't have — the opposite of drift.

- **Map keys removed remotely** (e.g., all `tags` cleared in the cloud).
  The plan wants to push config values back. `graft absorb` has nothing to
  capture because the cloud state is empty. Fix: re-apply or remove the
  declaration from config.

- **Nested block removed remotely** (e.g., `delete_retention_policy` disabled
  in the cloud). The plan wants to re-create the block. `graft absorb` has
  nothing to capture because the cloud state has no block. Fix: re-apply or
  remove the block from config.

In general, `graft absorb` captures what the cloud *has* that differs from
config. It does not override config to *remove* things you intentionally
declared.

---

## TODO

### 5. `count`-Indexed Resource Drift

Resources using `count` (e.g., `azurerm_resource_group.main[0]`).

- Support category 1–3 drift types for each indexed instance.

### 6. `for_each`-Indexed Resource Drift

Resources using `for_each` (e.g., `azurerm_resource_group.main["key"]`).

- Support category 1–3 drift types for each keyed instance.

---