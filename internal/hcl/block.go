package hcl

import (
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
)

// BlocksByType returns all blocks of the given type (excludes dynamic blocks)
func BlocksByType(body *hclwrite.Body, blockType string) []*hclwrite.Block {
	var result []*hclwrite.Block
	for _, block := range body.Blocks() {
		if block.Type() == blockType {
			result = append(result, block)
		}
	}
	return result
}

// DynamicBlocksByType returns all dynamic blocks that generate the given block type
func DynamicBlocksByType(body *hclwrite.Body, blockType string) []*hclwrite.Block {
	var result []*hclwrite.Block
	for _, block := range body.Blocks() {
		if block.Type() == "dynamic" && len(block.Labels()) > 0 && block.Labels()[0] == blockType {
			result = append(result, block)
		}
	}
	return result
}

// ListBlockTypes returns a set of all block types in a body
func ListBlockTypes(body *hclwrite.Body) map[string]bool {
	types := make(map[string]bool)
	for _, block := range body.Blocks() {
		types[block.Type()] = true
	}
	return types
}

// SortedBlockTypes returns all block types in sorted order for deterministic iteration
func SortedBlockTypes(body *hclwrite.Body) []string {
	types := ListBlockTypes(body)
	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

// DeepCopyBlock creates a deep copy of an HCL block
func DeepCopyBlock(block *hclwrite.Block) *hclwrite.Block {
	newBlock := hclwrite.NewBlock(block.Type(), block.Labels())
	copyBlockContentsInternal(block, newBlock)
	return newBlock
}

// copyBlockContentsInternal copies all attributes and nested blocks from src to dst
func copyBlockContentsInternal(src, dst *hclwrite.Block) {
	// Copy attributes in sorted order for deterministic output
	attrs := src.Body().Attributes()
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dst.Body().SetAttributeRaw(name, attrs[name].Expr().BuildTokens(nil))
	}

	// Copy nested blocks
	for _, block := range src.Body().Blocks() {
		newBlock := dst.Body().AppendNewBlock(block.Type(), block.Labels())
		copyBlockContentsInternal(block, newBlock)
	}
}

// SortedAttributeNames returns attribute names in sorted order for deterministic iteration
func SortedAttributeNames(body *hclwrite.Body) []string {
	attrs := body.Attributes()
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
