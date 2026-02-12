# Example 15: Absorb Block Drift on Indexed Resources (`count` & `for_each`)

This example demonstrates how `graft absorb` handles **block-type drift** on resources created with `count` or `for_each`. Instead of using static blocks (which would be lossy since all instances would get the same blocks), `graft absorb` generates `dynamic` blocks with `lookup()` expressions so each indexed instance gets its own block content.

## Scenario

You have two sets of Azure resources:

1. **`count`-indexed NSGs** (multiple sibling blocks): Two Network Security Groups created with `count = 2`, each with a single `security_rule`. Over time the security rules drift differently:
   - Instance `[0]`: Two rules added (`allow-ssh` at priority 100, `allow-https` at priority 200).
   - Instance `[1]`: Three rules added (`allow-http` at priority 100, `allow-https` at priority 200, `deny-all` at priority 4096).

2. **`for_each`-indexed VMs** (single nested block): Two Linux Virtual Machines created with `for_each = toset(["web", "api"])`. The `os_disk` settings drift:
   - Instance `["web"]`: caching changed to `ReadWrite`, disk size to `50`, storage account type to `Premium_LRS`.
   - Instance `["api"]`: disk size changed to `64`, storage account type to `StandardSSD_LRS`.

Running `terraform plan` shows four resources with drift.

## Key Concepts

### `dynamic` blocks with `lookup()` for block drift

When block-type attributes differ across indexed instances, `graft absorb` generates `dynamic` blocks with `lookup()` in the `for_each`:

```hcl
dynamic "security_rule" {
    for_each = lookup({
        0 = [{ name = "allow-ssh", priority = 100 }]
        1 = [{ name = "allow-http", priority = 100 }]
    }, count.index, [])
    content {
        name     = security_rule.value.name
        priority = security_rule.value.priority
    }
}
```

Each entry in the lookup map is a list of block objects for that instance. The `content` block uses `blockname.value.attr` references to pull attributes from the current iterator element.

### Single nested blocks

For single nested blocks like `os_disk`, the value in the lookup map is a single-element list:

```hcl
dynamic "os_disk" {
    for_each = lookup({
        "web" = [{ caching = "ReadWrite", disk_size_gb = 50 }]
        "api" = [{ caching = "ReadOnly",  disk_size_gb = 64 }]
    }, each.key, [])
    content {
        caching      = os_disk.value.caching
        disk_size_gb = os_disk.value.disk_size_gb
    }
}
```

### `[]` fallback

The third argument to `lookup()` is `[]` (empty list). This means instances without drift for a given block will produce no dynamic block iterations. Combined with `_graft { remove = ["block_type"] }`, the original static blocks are removed and only the dynamic blocks from the override take effect.

> **Note:** Unlike root-level attributes which use `graft.source` as fallback, block drift uses `[]` because `graft.source` cannot be used with `dynamic` block `for_each` (the original source has static blocks, not a list expression). Instances without block drift in the lookup map will have their original static blocks removed.

### `_graft { remove = [...] }`

A `_graft { remove = ["block_type"] }` directive is added to ensure the original static blocks are removed before the dynamic block generates the instance-specific replacements.

## Files

- `main.tf`: Terraform configuration with `count` NSGs and `for_each` VMs.
- `plan.json`: Terraform plan JSON output showing block drift on all four instances.
- `absorb.graft.hcl`: The generated graft manifest (output of `graft absorb`).

## Usage

1. Deploy your infrastructure with `terraform apply`.
2. After drift occurs, generate a plan:

   ```bash
   terraform plan -out=tfplan
   terraform show -json tfplan > plan.json
   ```

3. Run `graft absorb`:

   ```bash
   graft absorb plan.json
   ```

4. Review the generated `absorb.graft.hcl`.

5. Apply the overrides:

   ```bash
   graft build
   terraform plan   # Should show zero changes
   ```

## Comparison with Non-Indexed Block Drift

| Aspect | Non-indexed (Example 13) | Indexed (This Example) |
|--------|-------------------------|----------------------|
| Block rendering | Static blocks | `dynamic` blocks with `lookup()` |
| Per-instance values | N/A (single instance) | Each instance gets its own block content |
| Fallback | Original config preserved | `[]` (empty — known limitation) |
| `_graft remove` | For multi-blocks only | Always (dynamic replaces static) |
