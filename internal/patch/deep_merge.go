package patch

import (
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/internal/hcl"
)

// metaArgumentBlocks are block types that have special override behavior in Terraform.
// These should NOT be deep-merged because Terraform handles them with special semantics:
//
// - lifecycle: Terraform merges on an argument-by-argument basis
// - connection: If override contains a connection block, it completely replaces the original
// - provisioner: If override contains ANY provisioner blocks, ALL original provisioners are ignored
//
// For these blocks, we pass the override through as-is and let Terraform handle the merge/replace.
// See: https://developer.hashicorp.com/terraform/language/files/override
var metaArgumentBlocks = map[string]bool{
	"lifecycle":   true,
	"connection":  true,
	"provisioner": true,
}

// deepMergeNestedBlock performs deep merge of nested blocks for override files.
// For Terraform override files, nested blocks need to contain ALL attributes because
// Terraform uses shallow merge for blocks (replaces the entire block).
//
// This function takes an override block and the existing source block, and returns
// a new block with:
// 1. Top-level attributes from override only (Terraform handles shallow merge)
// 2. Nested blocks deep-merged: copy all attributes from source block, then override
// 3. Meta-argument blocks (lifecycle, connection, provisioner) are NOT deep-merged
func deepMergeNestedBlock(sourceBlock, overrideBlock *hclwrite.Block, isRootLevel bool) *hclwrite.Block {
	if overrideBlock == nil {
		return hcl.DeepCopyBlock(sourceBlock)
	}
	result := hclwrite.NewBlock(sourceBlock.Type(), sourceBlock.Labels())

	var fromBlock *hclwrite.Block
	var toBlock *hclwrite.Block
	if sourceBlock.Type() == "dynamic" {
		// Copy dynamic block header attributes in sorted order
		attrs := sourceBlock.Body().Attributes()
		for _, name := range hcl.SortedAttributeNames(sourceBlock.Body()) {
			result.Body().SetAttributeRaw(name, attrs[name].Expr().BuildTokens(nil))
		}
		// Copy iterator block if present
		for _, block := range sourceBlock.Body().Blocks() {
			switch block.Type() {
			case "iterator":
				result.Body().AppendBlock(hcl.DeepCopyBlock(block))
			case "content":
				fromBlock = block
			}
		}
		toBlock = hclwrite.NewBlock("content", nil)
		result.Body().AppendBlock(toBlock)
	} else {
		fromBlock = sourceBlock
		toBlock = result
	}

	if !isRootLevel && fromBlock != nil {
		// Copy all attributes from source in sorted order
		attrs := fromBlock.Body().Attributes()
		for _, name := range hcl.SortedAttributeNames(fromBlock.Body()) {
			toBlock.Body().SetAttributeRaw(name, attrs[name].Expr().BuildTokens(nil))
		}
	}

	// Override with attributes from override in sorted order
	overrideAttrs := overrideBlock.Body().Attributes()
	for _, name := range hcl.SortedAttributeNames(overrideBlock.Body()) {
		toBlock.Body().SetAttributeRaw(name, overrideAttrs[name].Expr().BuildTokens(nil))
	}

	// Get all nested block types from the override
	// Process each nested block type in sorted order for deterministic output
	for _, blockType := range hcl.SortedBlockTypes(overrideBlock.Body()) {
		overrideNestedBlocks := hcl.BlocksByType(overrideBlock.Body(), blockType)

		// Skip meta-argument blocks - Terraform handles these specially
		// (lifecycle/connection are replaced, provisioner is appended)
		if isRootLevel && metaArgumentBlocks[blockType] {
			for _, ob := range overrideNestedBlocks {
				newBlock := hcl.DeepCopyBlock(ob)
				toBlock.Body().AppendBlock(newBlock)
			}
			continue
		}

		// Use the first override block as the template for merging
		// (typically there's only one override block per type for specifying what to merge)
		var overrideTemplate *hclwrite.Block
		if len(overrideNestedBlocks) > 0 {
			overrideTemplate = overrideNestedBlocks[0]
		}

		// Find all matching blocks in source (both static and dynamic)
		sourceStaticBlocks := hcl.BlocksByType(fromBlock.Body(), blockType)
		sourceDynamicBlocks := hcl.DynamicBlocksByType(fromBlock.Body(), blockType)

		// Deep merge each static block from source
		for _, sourceNested := range sourceStaticBlocks {
			mergedBlock := deepMergeNestedBlock(sourceNested, overrideTemplate, false)
			toBlock.Body().AppendBlock(mergedBlock)
		}

		// Deep merge each dynamic block from source
		for _, sourceDynamic := range sourceDynamicBlocks {
			mergedDynamic := deepMergeNestedBlock(sourceDynamic, overrideTemplate, false)
			toBlock.Body().AppendBlock(mergedDynamic)
		}

		// If no source blocks exist for this type, add the override block as-is
		if len(sourceStaticBlocks) == 0 && len(sourceDynamicBlocks) == 0 {
			for _, ob := range overrideNestedBlocks {
				newBlock := hcl.DeepCopyBlock(ob)
				toBlock.Body().AppendBlock(newBlock)
			}
		}
	}

	return result
}
