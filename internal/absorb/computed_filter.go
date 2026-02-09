package absorb

import (
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

// filterComputedAttrs recursively filters computed-only attributes from a value
// using the provider schema. attrPath is the dot-separated path from the root
// schema (e.g. "" for top-level, "subnet" for a nested block, "subnet.name"
// for a field inside that block).
func filterComputedAttrs(val interface{}, schema *tfjson.SchemaBlock, attrPath string) interface{} {
	if schema == nil || val == nil {
		return val
	}
	if attrPath != "" && isAttrPathComputedOnly(schema, attrPath) {
		return nil
	}
	switch v := val.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, value := range v {
			childPath := key
			if attrPath != "" {
				childPath = attrPath + "." + key
			}
			if result := filterComputedAttrs(value, schema, childPath); result != nil {
				out[key] = result
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			if result := filterComputedAttrs(item, schema, attrPath); result != nil {
				out = append(out, result)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return val
	}
}

// isAttrPathComputedOnly checks whether the attribute at the given dot-separated
// path (e.g. "subnet.name") is computed-only according to the provider schema.
// It recursively walks through nested blocks, nested attribute types, and
// old-style cty attribute-as-block structures to resolve the final attribute.
func isAttrPathComputedOnly(schema *tfjson.SchemaBlock, attrPath string) bool {
	if schema == nil || attrPath == "" {
		return false
	}

	// Split the first segment from the remaining path.
	segment, rest, _ := strings.Cut(attrPath, ".")

	// Base case: no remaining path, so this is the leaf attribute.
	if rest == "" {
		if attr, ok := schema.Attributes[segment]; ok {
			return isComputedOnly(attr)
		}
		return false
	}

	// Recursive case: navigate into the nested schema for this segment,
	// then recurse with the remaining path.

	// Try nested block (e.g. block { ... } in schema).
	if bt, ok := schema.NestedBlocks[segment]; ok && bt.Block != nil {
		return isAttrPathComputedOnly(bt.Block, rest)
	}

	// Try nested attribute type (e.g. attribute with a nested object type).
	if attr, ok := schema.Attributes[segment]; ok && attr != nil && attr.AttributeNestedType != nil {
		nestedSchema := &tfjson.SchemaBlock{
			Attributes: attr.AttributeNestedType.Attributes,
		}
		return isAttrPathComputedOnly(nestedSchema, rest)
	}

	// Try old-style cty attribute-as-block: a list/set of objects represented
	// as a plain cty type rather than a nested block. We synthesize a schema
	// from the object's field names, marking "id" as computed and everything
	// else as optional.
	//
	// Known limitation: only "id" is treated as computed here. Other
	// computed-only fields (e.g. resource_guid) inside attribute-as-block
	// objects will not be filtered and may appear in the generated manifest.
	// A future improvement could consult the full provider schema or accept
	// a user-supplied exclusion list.
	if attr, ok := schema.Attributes[segment]; ok && attr != nil {
		ty := attr.AttributeType
		if ty == cty.NilType || (!ty.IsListType() && !ty.IsSetType()) {
			return false
		}
		elem := ty.ElementType()
		if !elem.IsObjectType() {
			return false
		}
		syntheticAttrs := make(map[string]*tfjson.SchemaAttribute, len(elem.AttributeTypes()))
		for fieldName := range elem.AttributeTypes() {
			if fieldName == "id" {
				syntheticAttrs[fieldName] = &tfjson.SchemaAttribute{Computed: true}
			} else {
				syntheticAttrs[fieldName] = &tfjson.SchemaAttribute{Optional: true}
			}
		}
		return isAttrPathComputedOnly(&tfjson.SchemaBlock{Attributes: syntheticAttrs}, rest)
	}

	return false
}

// isComputedOnly returns true if a schema attribute is computed but NOT optional/required,
// meaning it cannot be set in configuration.
func isComputedOnly(attr *tfjson.SchemaAttribute) bool {
	return attr != nil && attr.Computed && !attr.Optional && !attr.Required
}
