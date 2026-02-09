package absorb

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// ParsePlanFile reads and parses a Terraform plan JSON file and returns the
// drift changes found.
func ParsePlanFile(planPath string) ([]DriftChange, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Build a map of resource address -> after_unknown from resource_changes
	afterUnknownByAddress := buildAfterUnknownMap(plan.ResourceChanges)

	result := make([]DriftChange, 0)

	for _, rc := range plan.ResourceDrift {
		if !rc.Change.Actions.Update() {
			continue
		}

		modulePath := parseModulePath(rc.ModuleAddress)
		if rc.Mode != tfjson.ManagedResourceMode {
			continue
		}

		// Convert cty values to map[string]interface{} for comparison
		beforeMap := ctyValueToMap(rc.Change.Before)
		afterMap := ctyValueToMap(rc.Change.After)

		// Get the after_unknown map for this resource to identify computed attributes
		afterUnknown := afterUnknownByAddress[rc.Address]

		// Find attributes that drifted and capture the 'after' values (current cloud state)
		changedAttrs := findDriftedAttributes(beforeMap, afterMap, afterUnknown)
		if len(changedAttrs) == 0 {
			continue
		}

		change := DriftChange{
			Address:      rc.Address,
			ModulePath:   modulePath,
			ResourceType: rc.Type,
			ResourceName: rc.Name,
			ProviderName: rc.ProviderName,
			Mode:         string(rc.Mode),
			ChangedAttrs: changedAttrs,
		}
		result = append(result, change)
	}

	return result, nil
}

// buildAfterUnknownMap creates a map from resource address to its after_unknown structure.
func buildAfterUnknownMap(resourceChanges []*tfjson.ResourceChange) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for _, rc := range resourceChanges {
		if rc.Change != nil && rc.Change.AfterUnknown != nil {
			afterUnknown := ctyValueToMap(rc.Change.AfterUnknown)
			if afterUnknown != nil {
				result[rc.Address] = afterUnknown
			}
		}
	}
	return result
}

// ctyValueToMap converts a value (from tfjson) to map[string]interface{}.
func ctyValueToMap(val interface{}) map[string]interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case map[string]interface{}:
		return v
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return nil
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil
		}
		return result
	}
}

func parseModulePath(moduleAddr string) []string {
	if moduleAddr == "" {
		return nil
	}

	parts := strings.Split(moduleAddr, ".")
	var path []string
	for i := 0; i < len(parts); i++ {
		if parts[i] == "module" && i+1 < len(parts) {
			path = append(path, parts[i+1])
			i++
		}
	}
	return path
}

// isComputedInAfterUnknown checks if an attribute is marked as computed in after_unknown.
func isComputedInAfterUnknown(afterUnknown map[string]interface{}, key string) bool {
	if afterUnknown == nil {
		return false
	}
	val, exists := afterUnknown[key]
	if !exists {
		return false
	}
	if b, ok := val.(bool); ok && b {
		return true
	}
	return false
}

// findDriftedAttributes compares before and after states and returns the 'after' values
// for attributes that have drifted. Uses afterUnknown to skip computed attributes.
func findDriftedAttributes(before, after, afterUnknown map[string]interface{}) map[string]interface{} {
	if after == nil {
		return nil
	}

	changed := make(map[string]interface{})

	for key, afterVal := range after {
		// Known limitation: if an attribute was non-nil before and nil after,
		// that drift (attribute removal) is silently skipped. A future
		// improvement could emit an explicit null to represent the removal.
		if afterVal == nil {
			continue
		}

		// Skip the timeouts block (it's special)
		if key == "timeouts" {
			continue
		}

		// Skip attributes that are marked as computed in after_unknown
		if isComputedInAfterUnknown(afterUnknown, key) {
			continue
		}

		beforeVal, exists := before[key]

		// Capture the 'after' value if it differs from 'before'
		if !exists || !deepEqual(beforeVal, afterVal) {
			changed[key] = afterVal
		}
	}

	return changed
}

func deepEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
