# Changelog

## v0.2.0
### Features
- **`absorb` Command**: New command that analyzes a Terraform plan JSON file to detect drift and automatically generates a graft manifest (`absorb.graft.hcl`) with override blocks to match the current remote state. Supports provider schema for improved output accuracy.
- **Deep Merge for Nested Blocks**: Override files now perform deep merge on nested blocks, preserving original attributes from the source while applying override values. This applies to both static blocks and dynamic blocks.

## v0.1.0
### Features
- Initial release of Graft - The Overlay Engine for Terraform
- `graft build` command to apply patches to Terraform modules
- `graft clean` command to clean up generated files
- `graft scaffold` command to generate manifest files from existing modules