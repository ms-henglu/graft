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

// nestedBlockSchema returns the SchemaBlock for a nested block-type key, handling
// all three representations: NestedBlocks, AttributeNestedType, and old-style
// cty list/set of objects (attribute-as-block). Returns nil if no nested schema
// can be resolved.
func nestedBlockSchema(schema *tfjson.SchemaBlock, name string) *tfjson.SchemaBlock {
	if schema == nil {
		return nil
	}

	// Case 1: defined as a nested block
	if schema.NestedBlocks != nil && schema.NestedBlocks[name] != nil {
		return schema.NestedBlocks[name].Block
	}

	attr, ok := schema.Attributes[name]
	if !ok || attr == nil {
		return nil
	}

	// Case 2: new-style nested attribute type
	if attr.AttributeNestedType != nil {
		return &tfjson.SchemaBlock{
			Attributes: attr.AttributeNestedType.Attributes,
		}
	}

	// Case 3: old-style cty list/set of objects — synthesize a schema from the
	// object's field names. "id" is marked computed (common pattern); everything
	// else is optional.
	ty := attr.AttributeType
	if ty == cty.NilType {
		return nil
	}
	if !ty.IsListType() && !ty.IsSetType() {
		return nil
	}
	elem := ty.ElementType()
	if !elem.IsObjectType() {
		return nil
	}
	syntheticAttrs := make(map[string]*tfjson.SchemaAttribute, len(elem.AttributeTypes()))
	for fieldName := range elem.AttributeTypes() {
		if fieldName == "id" {
			syntheticAttrs[fieldName] = &tfjson.SchemaAttribute{Computed: true}
		} else {
			syntheticAttrs[fieldName] = &tfjson.SchemaAttribute{Optional: true}
		}
	}
	return &tfjson.SchemaBlock{Attributes: syntheticAttrs}
}
