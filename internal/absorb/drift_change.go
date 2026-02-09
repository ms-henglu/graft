package absorb

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// DriftChange represents a single resource with drift
type DriftChange struct {
	Address      string
	ModulePath   []string
	ResourceType string
	ResourceName string
	ProviderName string
	Mode         string
	ChangedAttrs map[string]interface{}
	BeforeAttrs  map[string]interface{}
}

func (change DriftChange) ToBlock(schema *tfjson.SchemaBlock) *hclwrite.Block {
	// Filter computed attributes when schema is available
	result := filterComputedAttrs(change.ChangedAttrs, schema, "")
	attrs, _ := result.(map[string]interface{})
	if len(attrs) == 0 {
		return nil
	}

	// Narrow to minimal diff using before state and schema
	if change.BeforeAttrs != nil && schema != nil {
		attrs = deepDiffBlock(change.BeforeAttrs, attrs, schema)
		if len(attrs) == 0 {
			return nil
		}
	}

	resBlock := hclwrite.NewBlock("resource", []string{change.ResourceType, change.ResourceName})
	resBody := resBlock.Body()

	// Collect block-type attributes that need _graft removal
	var removals []string

	var keys []string
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := attrs[key]
		if shouldRenderAsBlock(schema, key) {
			blocks := toBlocks(key, value, schema)
			for _, b := range blocks {
				resBody.AppendBlock(b)
			}
			if len(blocks) > 1 {
				removals = append(removals, key)
			}
		} else {
			ctyVal := interfaceToCtyValue(value)
			resBody.SetAttributeValue(key, ctyVal)
		}
	}

	if len(removals) > 0 {
		sort.Strings(removals)
		graftBlock := resBody.AppendNewBlock("_graft", nil)
		graftBody := graftBlock.Body()
		var vals []cty.Value
		for _, r := range removals {
			vals = append(vals, cty.StringVal(r))
		}
		graftBody.SetAttributeValue("remove", cty.ListVal(vals))
	}

	return resBlock
}

func toBlocks(blockName string, value interface{}, schema *tfjson.SchemaBlock) []*hclwrite.Block {
	out := make([]*hclwrite.Block, 0)
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				out = append(out, toBlocks(blockName, itemMap, schema)...)
			}
		}
	case map[string]interface{}:
		block := hclwrite.NewBlock(blockName, nil)
		// Get the nested schema for this block
		nestedSchema := nestedBlockSchema(schema, blockName)

		var keys []string
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			attrValue := v[key]
			if attrValue == nil {
				continue
			}
			// Skip empty strings in nested blocks.
			// Known limitation: legitimate empty-string values (e.g.
			// description = "") are also dropped. A future improvement
			// could use the provider schema to distinguish intentional
			// empty strings from default/unset values.
			if str, ok := attrValue.(string); ok && str == "" {
				continue
			}

			// Recursively handle nested blocks within blocks
			if nestedSchema != nil && shouldRenderAsBlock(nestedSchema, key) {
				blocks := toBlocks(key, attrValue, nestedSchema)
				for _, b := range blocks {
					block.Body().AppendBlock(b)
				}
				continue
			}

			ctyVal := interfaceToCtyValue(attrValue)
			block.Body().SetAttributeValue(key, ctyVal)
		}
		out = append(out, block)
	}
	return out
}

// deepDiffBlock performs a schema-aware recursive diff between before and after
// maps, returning only the changed attributes in after. For single nested blocks
// (map values where shouldRenderAsBlock is true), it recurses to find the minimal
// diff. For multiple blocks (slice values where shouldRenderAsBlock is true) and
// plain attributes, it captures the full after value.
func deepDiffBlock(before, after map[string]interface{}, schema *tfjson.SchemaBlock) map[string]interface{} {
	if len(after) == 0 {
		return nil
	}

	result := make(map[string]interface{})

	for key, afterVal := range after {
		beforeVal, hasBefore := before[key]

		// If there's no before value, the attribute is new — capture full value
		if !hasBefore {
			result[key] = afterVal
			continue
		}

		// If the values are equal, skip
		if deepEqual(beforeVal, afterVal) {
			continue
		}

		// Check if this key should be rendered as a block
		if shouldRenderAsBlock(schema, key) {
			afterMap, beforeMap, isSingleBlock := extractSingleBlock(afterVal, beforeVal)

			// Single block — recurse for minimal diff
			if isSingleBlock {
				nestedSchema := nestedBlockSchema(schema, key)
				if nestedSchema != nil {
					diffed := deepDiffBlock(beforeMap, afterMap, nestedSchema)
					if len(diffed) > 0 {
						result[key] = diffed
					}
				} else {
					// No nested schema available — capture full value
					result[key] = afterVal
				}
				continue
			}

			// Multiple blocks (slice) — capture full array (needed for _graft remove)
			result[key] = afterVal
			continue
		}

		// Plain attribute (scalars, maps like tags) — capture full value
		result[key] = afterVal
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// extractSingleBlock checks whether both after and before values represent a
// single block. In real Terraform plan JSON, single blocks are encoded as a
// one-element array (e.g. "os_disk": [{...}]), but in test data they may appear
// as plain maps. This function handles both representations and returns the
// inner maps if exactly one block is present on each side.
func extractSingleBlock(afterVal, beforeVal interface{}) (afterMap, beforeMap map[string]interface{}, ok bool) {
	afterMap, afterOk := asSingleBlockMap(afterVal)
	beforeMap, beforeOk := asSingleBlockMap(beforeVal)
	if afterOk && beforeOk {
		return afterMap, beforeMap, true
	}
	return nil, nil, false
}

// asSingleBlockMap extracts a single map from a value that is either a plain
// map or a one-element slice containing a map.
func asSingleBlockMap(val interface{}) (map[string]interface{}, bool) {
	if m, ok := val.(map[string]interface{}); ok {
		return m, true
	}
	if arr, ok := val.([]interface{}); ok && len(arr) == 1 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			return m, true
		}
	}
	return nil, false
}

func interfaceToCtyValue(v interface{}) cty.Value {
	if v == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}

	switch val := v.(type) {
	case bool:
		return cty.BoolVal(val)
	case float64:
		if val == float64(int64(val)) {
			return cty.NumberIntVal(int64(val))
		}
		return cty.NumberFloatVal(val)
	case string:
		return cty.StringVal(val)
	case []interface{}:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType)
		}
		var vals []cty.Value
		for _, item := range val {
			vals = append(vals, interfaceToCtyValue(item))
		}
		return cty.TupleVal(vals)
	case map[string]interface{}:
		if len(val) == 0 {
			return cty.MapValEmpty(cty.DynamicPseudoType)
		}
		vals := make(map[string]cty.Value)
		for k, item := range val {
			vals[k] = interfaceToCtyValue(item)
		}
		return cty.ObjectVal(vals)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return cty.StringVal(fmt.Sprintf("%v", v))
		}
		ctyVal, err := ctyjson.Unmarshal(jsonBytes, cty.DynamicPseudoType)
		if err != nil {
			return cty.StringVal(string(jsonBytes))
		}
		return ctyVal
	}
}
