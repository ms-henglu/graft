package absorb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseModulePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single module",
			input:    "module.network",
			expected: []string{"network"},
		},
		{
			name:     "nested modules",
			input:    "module.network.module.subnet",
			expected: []string{"network", "subnet"},
		},
		{
			name:     "deeply nested",
			input:    "module.a.module.b.module.c",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseModulePath(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestFindDriftedAttributes(t *testing.T) {
	tests := []struct {
		name     string
		before   map[string]interface{}
		after    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil after returns nil",
			before:   map[string]interface{}{"key": "value"},
			after:    nil,
			expected: nil,
		},
		{
			name:     "no changes",
			before:   map[string]interface{}{"key": "value"},
			after:    map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{},
		},
		{
			name:     "simple change - returns after value",
			before:   map[string]interface{}{"key": "old"},
			after:    map[string]interface{}{"key": "new"},
			expected: map[string]interface{}{"key": "new"},
		},
		{
			name: "map change - returns after value",
			before: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Prod"},
			},
			after: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Dev"},
			},
			expected: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Dev"},
			},
		},
		{
			name: "skips timeouts",
			before: map[string]interface{}{
				"name":     "test",
				"timeouts": map[string]interface{}{"create": "30m"},
			},
			after: map[string]interface{}{
				"name":     "changed",
				"timeouts": map[string]interface{}{"create": "60m"},
			},
			expected: map[string]interface{}{
				"name": "changed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findDriftedAttributes(tt.before, tt.after)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d attributes, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for k, v := range tt.expected {
				if !deepEqual(result[k], v) {
					t.Errorf("for key %s: expected %v, got %v", k, v, result[k])
				}
			}
		})
	}
}

func TestParsePlanFile(t *testing.T) {
	planJSON := `{
		"format_version": "1.0",
		"terraform_version": "1.5.0",
		"resource_changes": [
			{
				"address": "module.network.azurerm_virtual_network.vnet",
				"module_address": "module.network",
				"mode": "managed",
				"type": "azurerm_virtual_network",
				"name": "vnet",
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "tags": { "Env": "Dev" } },
					"after": { "tags": { "Env": "Prod" } }
				}
			},
			{
				"address": "azurerm_resource_group.main",
				"module_address": "",
				"mode": "managed",
				"type": "azurerm_resource_group",
				"name": "main",
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "location": "westus" },
					"after": { "location": "eastus" }
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	result, err := ParsePlanFile(planFile)
	if err != nil {
		t.Fatalf("ParsePlanFile failed: %v", err)
	}

	// All drifted resources should be included (both module-scoped and root)
	if len(result) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(result))
	}

	change := result[0]
	if change.Address != "module.network.azurerm_virtual_network.vnet" {
		t.Errorf("expected address 'module.network.azurerm_virtual_network.vnet', got '%s'", change.Address)
	}
	if change.ResourceType != "azurerm_virtual_network" {
		t.Errorf("expected resource type 'azurerm_virtual_network', got '%s'", change.ResourceType)
	}
	if change.ResourceName != "vnet" {
		t.Errorf("expected resource name 'vnet', got '%s'", change.ResourceName)
	}
	if change.ProviderName != "registry.terraform.io/hashicorp/azurerm" {
		t.Errorf("expected provider name 'registry.terraform.io/hashicorp/azurerm', got '%s'", change.ProviderName)
	}
	if len(change.ModulePath) != 1 || change.ModulePath[0] != "network" {
		t.Errorf("expected module path ['network'], got %v", change.ModulePath)
	}

	rootChange := result[1]
	if rootChange.Address != "azurerm_resource_group.main" {
		t.Errorf("expected address 'azurerm_resource_group.main', got '%s'", rootChange.Address)
	}
	if rootChange.ModulePath != nil {
		t.Errorf("expected nil module path for root resource, got %v", rootChange.ModulePath)
	}
}

func TestParsePlanFileRetainsAllAttributes(t *testing.T) {
	// ParsePlanFile should NOT filter computed attributes — that's the manifest's job
	planJSON := `{
		"format_version": "1.0",
		"terraform_version": "1.5.0",
		"resource_changes": [
			{
				"address": "module.network.azurerm_virtual_network.main",
				"module_address": "module.network",
				"mode": "managed",
				"type": "azurerm_virtual_network",
				"name": "main",
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": {
						"name": "my-vnet",
						"guid": "new-guid",
						"subnet": [
							{"name": "subnet1", "address_prefixes": ["10.0.5.0/24"], "id": "new-id"}
						]
					},
					"after": {
						"name": "my-vnet",
						"guid": "old-guid",
						"subnet": [
							{"name": "subnet1", "address_prefixes": ["10.0.1.0/24"], "id": "old-id"}
						]
					}
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	result, err := ParsePlanFile(planFile)
	if err != nil {
		t.Fatalf("ParsePlanFile failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result))
	}

	change := result[0]

	// guid should still be present (ParsePlanFile doesn't know about schema)
	if _, ok := change.ChangedAttrs["guid"]; !ok {
		t.Error("expected 'guid' to be present in raw ParsePlanFile result")
	}

	// subnet should have id still present
	subnet, ok := change.ChangedAttrs["subnet"]
	if !ok {
		t.Fatal("expected subnet in changed attrs")
	}
	subnetArr, ok := subnet.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", subnet)
	}
	subnetItem, ok := subnetArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", subnetArr[0])
	}
	if _, hasID := subnetItem["id"]; !hasID {
		t.Error("expected 'id' to still be present in raw ParsePlanFile result")
	}
}

func TestParsePlanFileNotFound(t *testing.T) {
	_, err := ParsePlanFile("/nonexistent/plan.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read plan file") {
		t.Errorf("expected 'failed to read plan file' error, got: %v", err)
	}
}

func TestParsePlanFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(planFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	_, err := ParsePlanFile(planFile)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse plan JSON") {
		t.Errorf("expected 'failed to parse plan JSON' error, got: %v", err)
	}
}

func TestParsePlanFileCountIndexed(t *testing.T) {
	planJSON := `{
		"format_version": "1.0",
		"terraform_version": "1.5.0",
		"resource_changes": [
			{
				"address": "azurerm_resource_group.main[0]",
				"mode": "managed",
				"type": "azurerm_resource_group",
				"name": "main",
				"index": 0,
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "tags": { "env": "dev" } },
					"after": { "tags": { "env": "prod" } }
				}
			},
			{
				"address": "azurerm_resource_group.main[1]",
				"mode": "managed",
				"type": "azurerm_resource_group",
				"name": "main",
				"index": 1,
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "tags": { "env": "staging" } },
					"after": { "tags": { "env": "production" } }
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	result, err := ParsePlanFile(planFile)
	if err != nil {
		t.Fatalf("ParsePlanFile failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(result))
	}

	// Verify first change
	if result[0].Address != "azurerm_resource_group.main[0]" {
		t.Errorf("expected address 'azurerm_resource_group.main[0]', got '%s'", result[0].Address)
	}
	if !result[0].IsCountIndexed() {
		t.Error("expected first change to be count-indexed")
	}
	if result[0].indexKey() != "0" {
		t.Errorf("expected indexKey '0', got '%s'", result[0].indexKey())
	}
	if result[0].ResourceName != "main" {
		t.Errorf("expected resource name 'main', got '%s'", result[0].ResourceName)
	}

	// Verify second change
	if result[1].Address != "azurerm_resource_group.main[1]" {
		t.Errorf("expected address 'azurerm_resource_group.main[1]', got '%s'", result[1].Address)
	}
	if !result[1].IsCountIndexed() {
		t.Error("expected second change to be count-indexed")
	}
	if result[1].indexKey() != "1" {
		t.Errorf("expected indexKey '1', got '%s'", result[1].indexKey())
	}
}

func TestParsePlanFileForEachIndexed(t *testing.T) {
	planJSON := `{
		"format_version": "1.0",
		"terraform_version": "1.5.0",
		"resource_changes": [
			{
				"address": "azurerm_resource_group.main[\"web\"]",
				"mode": "managed",
				"type": "azurerm_resource_group",
				"name": "main",
				"index": "web",
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "location": "westus" },
					"after": { "location": "eastus" }
				}
			},
			{
				"address": "azurerm_resource_group.main[\"api\"]",
				"mode": "managed",
				"type": "azurerm_resource_group",
				"name": "main",
				"index": "api",
				"provider_name": "registry.terraform.io/hashicorp/azurerm",
				"change": {
					"actions": ["update"],
					"before": { "location": "westus2" },
					"after": { "location": "eastus2" }
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	result, err := ParsePlanFile(planFile)
	if err != nil {
		t.Fatalf("ParsePlanFile failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(result))
	}

	if !result[0].IsForEachIndexed() {
		t.Error("expected first change to be for_each-indexed")
	}
	if result[0].indexKey() != "web" {
		t.Errorf("expected indexKey 'web', got '%s'", result[0].indexKey())
	}
	if result[0].indexRef() != "each.key" {
		t.Errorf("expected indexRef 'each.key', got '%s'", result[0].indexRef())
	}

	if !result[1].IsForEachIndexed() {
		t.Error("expected second change to be for_each-indexed")
	}
	if result[1].indexKey() != "api" {
		t.Errorf("expected indexKey 'api', got '%s'", result[1].indexKey())
	}
}

func TestDriftChangeHelpers(t *testing.T) {
	t.Run("non-indexed", func(t *testing.T) {
		c := DriftChange{Index: nil}
		if c.IsIndexed() {
			t.Error("expected not indexed")
		}
		if c.IsCountIndexed() {
			t.Error("expected not count indexed")
		}
		if c.IsForEachIndexed() {
			t.Error("expected not for_each indexed")
		}
	})

	t.Run("count-indexed", func(t *testing.T) {
		c := DriftChange{Index: float64(2)}
		if !c.IsIndexed() {
			t.Error("expected indexed")
		}
		if !c.IsCountIndexed() {
			t.Error("expected count indexed")
		}
		if c.IsForEachIndexed() {
			t.Error("expected not for_each indexed")
		}
		if c.indexKey() != "2" {
			t.Errorf("expected indexKey '2', got '%s'", c.indexKey())
		}
		if c.indexRef() != "count.index" {
			t.Errorf("expected indexRef 'count.index', got '%s'", c.indexRef())
		}
	})

	t.Run("for_each-indexed", func(t *testing.T) {
		c := DriftChange{Index: "mykey"}
		if !c.IsIndexed() {
			t.Error("expected indexed")
		}
		if c.IsCountIndexed() {
			t.Error("expected not count indexed")
		}
		if !c.IsForEachIndexed() {
			t.Error("expected for_each indexed")
		}
		if c.indexKey() != "mykey" {
			t.Errorf("expected indexKey 'mykey', got '%s'", c.indexKey())
		}
		if c.indexRef() != "each.key" {
			t.Errorf("expected indexRef 'each.key', got '%s'", c.indexRef())
		}
	})
}
