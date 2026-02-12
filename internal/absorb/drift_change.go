package absorb

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/ms-henglu/graft/internal/utils"
	"github.com/zclconf/go-cty/cty"
)

// DriftChange represents a single resource with drift
type DriftChange struct {
	Address         string
	ModulePath      []string
	ResourceType    string
	ResourceName    string
	ProviderName    string
	Mode            string
	Index           interface{} // nil = no index, float64 = count, string = for_each
	ChangedAttrs    map[string]interface{}
	BeforeAttrs     map[string]interface{}
	FullBlockValues map[string]interface{} // full (pre-deepDiff) block values for dynamic blocks
}

// IsIndexed returns true if the resource uses count or for_each.
func (change DriftChange) IsIndexed() bool {
	return change.Index != nil
}

// IsCountIndexed returns true if the resource uses count (numeric index).
func (change DriftChange) IsCountIndexed() bool {
	_, ok := change.Index.(float64)
	return ok
}

// IsForEachIndexed returns true if the resource uses for_each (string key).
func (change DriftChange) IsForEachIndexed() bool {
	_, ok := change.Index.(string)
	return ok
}

// indexKey returns a string suitable for use as a map key in the lookup expression.
// For count: "0", "1", etc. For for_each: the key string itself.
func (change DriftChange) indexKey() string {
	switch idx := change.Index.(type) {
	case float64:
		return fmt.Sprintf("%d", int(idx))
	case string:
		return idx
	default:
		return ""
	}
}

// indexRef returns the HCL expression referencing the current instance index.
// For count: "count.index". For for_each: "each.key".
func (change DriftChange) indexRef() string {
	if change.IsCountIndexed() {
		return "count.index"
	}
	return "each.key"
}

// resourceKey returns a grouping key for indexed resources: "type.name".
func (change DriftChange) resourceKey() string {
	return change.ResourceType + "." + change.ResourceName
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

	for _, key := range utils.SortedKeys(attrs) {
		value := attrs[key]
		if shouldRenderAsBlock(schema, key) {
			blocks := toBlocks(key, value, schema)
			for _, b := range blocks {
				resBody.AppendBlock(b)
			}
			if len(blocks) > 1 {
				removals = append(removals, key)
			} else if len(blocks) == 1 {
				// Single block — check for nested multi-blocks that need removal
				nestedRemovals := collectNestedRemovals(key, blocks[0])
				removals = append(removals, nestedRemovals...)
			}
		} else {
			ctyVal := utils.ToCtyValue(value)
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

// collectNestedRemovals inspects the child blocks of a single HCL block to find
// block types that appear more than once, returning their dotted paths (e.g.
// "backend_http_settings.connection_draining"). It recurses into block types
// that appear exactly once to discover deeply nested multi-blocks.
func collectNestedRemovals(prefix string, block *hclwrite.Block) []string {
	// Count occurrences of each child block type
	counts := make(map[string]int)
	for _, child := range block.Body().Blocks() {
		counts[child.Type()]++
	}

	var removals []string
	visited := make(map[string]bool)
	for _, child := range block.Body().Blocks() {
		blockType := child.Type()
		if visited[blockType] {
			continue
		}
		visited[blockType] = true
		path := prefix + "." + blockType
		if counts[blockType] > 1 {
			removals = append(removals, path)
		} else {
			removals = append(removals, collectNestedRemovals(path, child)...)
		}
	}
	return removals
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

		keys := utils.SortedKeys(v)

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

			ctyVal := utils.ToCtyValue(attrValue)
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

		// If the values are equal, skip
		if deepEqual(beforeVal, afterVal) {
			continue
		}

		// If there's no before value, the attribute is new — capture full value
		if !hasBefore ||
			// Not a block — capture full value
			!shouldRenderAsBlock(schema, key) ||
			// Not a single block — capture full value
			!isSingleBlock(afterVal) || !isSingleBlock(beforeVal) {
			result[key] = afterVal
			continue
		}

		nestedSchema := nestedBlockSchema(schema, key)
		if nestedSchema == nil {
			// No nested schema available — capture full value
			result[key] = afterVal
			continue
		}

		beforeMap := asSingleBlock(beforeVal)
		afterMap := asSingleBlock(afterVal)
		diffed := deepDiffBlock(beforeMap, afterMap, nestedSchema)
		if len(diffed) > 0 {
			result[key] = diffed
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func isSingleBlock(val interface{}) bool {
	if _, ok := val.(map[string]interface{}); ok {
		return true
	}
	if arr, ok := val.([]interface{}); ok && len(arr) == 1 {
		if _, ok := arr[0].(map[string]interface{}); ok {
			return true
		}
	}
	return false
}

func asSingleBlock(val interface{}) map[string]interface{} {
	if m, ok := val.(map[string]interface{}); ok {
		return m
	}
	if arr, ok := val.([]interface{}); ok && len(arr) == 1 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}
