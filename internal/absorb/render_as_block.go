package absorb

import (
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

// shouldRenderAsBlock determines whether a key should be rendered as nested block(s)
// rather than as an attribute assignment.
func shouldRenderAsBlock(schema *tfjson.SchemaBlock, name string) bool {
	if schema == nil {
		return false
	}

	// if defined as a block
	if schema.NestedBlocks != nil && schema.NestedBlocks[name] != nil {
		return true
	}

	// if defined as an attribute that is a list/set of objects
	attr, ok := schema.Attributes[name]
	if !ok || attr == nil {
		return false
	}

	// Check if the attribute has a nested type (new-style nested attributes)
	if attr.AttributeNestedType != nil {
		mode := attr.AttributeNestedType.NestingMode
		return mode == tfjson.SchemaNestingModeList || mode == tfjson.SchemaNestingModeSet
	}

	// Check if the cty type is list/set of object
	ty := attr.AttributeType
	if ty == cty.NilType {
		return false
	}
	if ty.IsListType() || ty.IsSetType() {
		elem := ty.ElementType()
		return elem.IsObjectType()
	}
	return false
}
