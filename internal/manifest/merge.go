package manifest

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/internal/hcl"
)

// mergeModuleLists merges two lists of modules
// Modules with the same name are deep merged
func mergeModuleLists(base, other []Module) []Module {
	// Build a map for quick lookup
	moduleMap := make(map[string]Module)
	var order []string

	// Add base modules
	for _, mod := range base {
		moduleMap[mod.Name] = mod
		order = append(order, mod.Name)
	}

	// Merge other modules
	for _, mod := range other {
		if existing, ok := moduleMap[mod.Name]; ok {
			// Deep merge modules with the same name
			moduleMap[mod.Name] = mergeModules(existing, mod)
		} else {
			moduleMap[mod.Name] = mod
			order = append(order, mod.Name)
		}
	}

	// Rebuild ordered list
	var result []Module
	for _, name := range order {
		result = append(result, moduleMap[name])
	}

	return result
}

// mergeModules deep merges two modules with the same name
func mergeModules(base, other Module) Module {
	result := Module{
		Name:           base.Name,
		Source:         base.Source,
		Version:        base.Version,
		OverrideBlocks: mergeOverrideBlocks(base.OverrideBlocks, other.OverrideBlocks),
		Modules:        mergeModuleLists(base.Modules, other.Modules),
	}

	// Last write wins for source and version if specified in other
	if other.Source != "" {
		result.Source = other.Source
	}
	if other.Version != "" {
		result.Version = other.Version
	}

	return result
}

// mergeOverrideBlocks merges two lists of flattened content blocks (resource, data, locals, etc.)
// Blocks with the same type and labels are deep merged using "last write wins" semantics.
func mergeOverrideBlocks(base, other []*hclwrite.Block) []*hclwrite.Block {
	if len(base) == 0 {
		return other
	}
	if len(other) == 0 {
		return base
	}

	// Track blocks by type and labels for merging
	blockMap := make(map[string]*hclwrite.Block)
	var blockOrder []string

	// Process base blocks
	for _, block := range base {
		key := blockKey(block)
		blockMap[key] = block
		blockOrder = append(blockOrder, key)
	}

	// Merge other blocks
	for _, block := range other {
		key := blockKey(block)
		if existing, ok := blockMap[key]; ok {
			// Merge blocks with the same type and labels
			blockMap[key] = mergeBlocks(existing, block)
		} else {
			blockMap[key] = block
			blockOrder = append(blockOrder, key)
		}
	}

	// Build result list preserving order
	var result []*hclwrite.Block
	for _, key := range blockOrder {
		result = append(result, blockMap[key])
	}

	return result
}

// blockKey generates a unique key for a block based on type and labels
func blockKey(block *hclwrite.Block) string {
	return block.Type() + ":" + strings.Join(block.Labels(), ".")
}

// mergeBlocks merges two blocks with the same type and labels
// Uses "last write wins" for attributes
func mergeBlocks(base, other *hclwrite.Block) *hclwrite.Block {
	result := hclwrite.NewBlock(base.Type(), base.Labels())

	// Copy base attributes
	for name, attr := range base.Body().Attributes() {
		result.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}

	// Merge/override with other attributes (last write wins)
	for name, attr := range other.Body().Attributes() {
		result.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}

	// Merge nested blocks
	baseBlocks := base.Body().Blocks()
	otherBlocks := other.Body().Blocks()

	nestedBlockMap := make(map[string]*hclwrite.Block)
	var nestedBlockOrder []string

	for _, b := range baseBlocks {
		key := blockKey(b)
		nestedBlockMap[key] = b
		nestedBlockOrder = append(nestedBlockOrder, key)
	}

	for _, b := range otherBlocks {
		key := blockKey(b)
		if existing, ok := nestedBlockMap[key]; ok {
			nestedBlockMap[key] = mergeBlocks(existing, b)
		} else {
			nestedBlockMap[key] = b
			nestedBlockOrder = append(nestedBlockOrder, key)
		}
	}

	for _, key := range nestedBlockOrder {
		block := nestedBlockMap[key]
		result.Body().AppendBlock(hcl.DeepCopyBlock(block))
	}

	return result
}
