package absorb

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// ParsePlanFile reads and parses a Terraform plan JSON file and returns the
// changes found in the plan that need to be absorbed.
func ParsePlanFile(planPath string) ([]DriftChange, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	result := make([]DriftChange, 0)

	for _, rc := range plan.ResourceChanges {
		if !rc.Change.Actions.Update() {
			continue
		}

		modulePath := parseModulePath(rc.ModuleAddress)
		if rc.Mode != tfjson.ManagedResourceMode {
			continue
		}

		// In resource_changes:
		//   before = current cloud state (after drift is applied)
		//   after  = desired config (what Terraform wants to apply)
		// We want to absorb the "before" values (cloud reality) into the config,
		// so that the plan becomes clean.
		cloudState := ctyValueToMap(rc.Change.Before)
		desiredConfig := ctyValueToMap(rc.Change.After)

		// Find attributes that differ and capture the cloud state values
		changedAttrs := findDriftedAttributes(desiredConfig, cloudState)
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
			BeforeAttrs:  desiredConfig,
		}
		result = append(result, change)
	}

	return result, nil
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

// findDriftedAttributes compares before and after states and returns the 'after' values
// for attributes that have drifted.
func findDriftedAttributes(before, after map[string]interface{}) map[string]interface{} {
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
