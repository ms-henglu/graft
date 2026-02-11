package absorb

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

func TestShouldRenderAsBlock(t *testing.T) {
	objType := cty.Object(map[string]cty.Type{
		"name": cty.String,
		"id":   cty.Number,
	})

	testcases := []struct {
		name     string
		schema   *tfjson.SchemaBlock
		attrName string
		expected bool
	}{
		{
			name:     "nil schema returns false",
			schema:   nil,
			attrName: "anything",
			expected: false,
		},
		{
			name:     "empty schema returns false",
			schema:   &tfjson.SchemaBlock{},
			attrName: "anything",
			expected: false,
		},
		{
			name: "nil attributes returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: nil,
			},
			attrName: "anything",
			expected: false,
		},
		{
			name: "defined nested block returns true",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"inline_block": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block:       &tfjson.SchemaBlock{},
					},
				},
			},
			attrName: "inline_block",
			expected: true,
		},
		{
			name: "nonexistent nested block returns false",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"inline_block": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block:       &tfjson.SchemaBlock{},
					},
				},
			},
			attrName: "nonexistent",
			expected: false,
		},
		{
			name: "nil NestedBlocks with string attribute returns false",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: nil,
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {
						AttributeType: cty.String,
					},
				},
			},
			attrName: "name",
			expected: false,
		},
		{
			name: "attribute not found returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {
						AttributeType: cty.String,
					},
				},
			},
			attrName: "missing",
			expected: false,
		},
		{
			name: "nil attribute value returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"nil_attr": nil,
				},
			},
			attrName: "nil_attr",
			expected: false,
		},
		{
			name: "nested attribute with list nesting mode returns true",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeList,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"inner": {AttributeType: cty.String},
							},
						},
					},
				},
			},
			attrName: "attr",
			expected: true,
		},
		{
			name: "nested attribute with set nesting mode returns true",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeSet,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"inner": {AttributeType: cty.String},
							},
						},
					},
				},
			},
			attrName: "attr",
			expected: true,
		},
		{
			name: "nested attribute with single nesting mode returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeSingle,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"inner": {AttributeType: cty.String},
							},
						},
					},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "nested attribute with group nesting mode returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeGroup,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"inner": {AttributeType: cty.String},
							},
						},
					},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "nested attribute with map nesting mode returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeMap,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"inner": {AttributeType: cty.String},
							},
						},
					},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty list of objects returns true",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.List(objType)},
				},
			},
			attrName: "attr",
			expected: true,
		},
		{
			name: "cty set of objects returns true",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.Set(objType)},
				},
			},
			attrName: "attr",
			expected: true,
		},
		{
			name: "cty list of strings returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.List(cty.String)},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty set of numbers returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.Set(cty.Number)},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty string returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.String},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty number returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.Number},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty bool returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.Bool},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty map of strings returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.Map(cty.String)},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty object (not list/set) returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: objType},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "cty NilType returns false",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attr": {AttributeType: cty.NilType},
				},
			},
			attrName: "attr",
			expected: false,
		},
		{
			name: "nested block takes precedence over attribute",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"dual": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block:       &tfjson.SchemaBlock{},
					},
				},
				Attributes: map[string]*tfjson.SchemaAttribute{
					"dual": {
						AttributeType: cty.String,
					},
				},
			},
			attrName: "dual",
			expected: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldRenderAsBlock(tc.schema, tc.attrName)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}
