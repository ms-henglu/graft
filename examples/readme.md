# Graft Examples

This directory contains a series of practical examples demonstrating how to use Graft to overlay and modify Terraform modules. Each example includes a brief description and relevant files.

### Build & Patch

| # | Example | Description |
|---|---------|-------------|
| 01 | [Override Hardcoded Values](./01-override-values) | Override a module's hardcoded `tags` on an `azurerm_virtual_network` without forking the module. |
| 02 | [Inject New Logic](./02-inject-new-logic) | Add a new `azurerm_storage_account` resource and output into an upstream module that doesn't define them. |
| 03 | [Surgical Removal](./03-remove-attributes) | Remove specific attributes, nested blocks, or entire resources from a module using the `_graft` block. |
| 04 | [Using `graft.source`](./04-graft-source) | Use `merge(graft.source, {...})` to append tags to a resource without losing the original values. |
| 05 | [Conditional Overrides with `for_each`](./05-for-each-override) | Combine `graft.source` with `each.key` to selectively override settings on specific `for_each` instances. |
| 06 | [Multi-Layer Module Override](./06-multi-layer-module-override) | Override resources and variables across multiple levels of a nested module hierarchy (root → parent → child). |
| 07 | [Scaffold Manifest](./07-scaffold) | Use `graft scaffold` to auto-discover modules and generate a boilerplate manifest with placeholder overrides. |
| 08 | [Ignore Changes](./08-lifecycle-ignore-changes) | Inject `lifecycle { ignore_changes = [tags] }` into a module resource to prevent Terraform from reverting external tag changes. |
| 09 | [Prevent Destroy](./09-lifecycle-prevent-destroy) | Add `lifecycle { prevent_destroy = true }` to a module resource to guard against accidental deletion. |
| 10 | [Mark Values as Sensitive](./10-mark-as-sensitive) | Retrofit `sensitive = true` onto module outputs and variables to prevent sensitive data from appearing in logs. |
| 11 | [Nested Block Override](./11-multi-block-resource) | Deep merge into static and dynamic nested blocks (e.g., add `default_outbound_access_enabled` to all subnets) while preserving original attributes. |

### Absorb Drift

| # | Example | Description |
|---|---------|-------------|
| 12 | [Absorb Tag Drift](./12-absorb-tag-drift) | Auto-generate a graft manifest when resource group tags are changed outside Terraform (e.g., via Azure CLI or Azure Policy). |
| 13 | [Absorb Security Rule Drift](./13-absorb-security-rule-drift) | Absorb NSG security rule drift, demonstrating how `graft absorb` handles the attribute-as-block pattern with automatic `_graft { remove }` generation. |
| 14 | [Absorb Indexed Resource Drift](./14-absorb-indexed-drift) | Absorb drift on `count` and `for_each` resources, demonstrating how `graft absorb` groups instances and generates `lookup()` expressions with `count.index`/`each.key`. |
