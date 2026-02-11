package absorb

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
)

func TestNewModuleItem(t *testing.T) {
	t.Run("creates root item with empty name", func(t *testing.T) {
		m := NewModuleItem("")
		if m.Name != "" {
			t.Errorf("expected empty name, got %q", m.Name)
		}
		if m.Children == nil {
			t.Fatal("expected non-nil Children map")
		}
		if len(m.Children) != 0 {
			t.Errorf("expected empty Children, got %d", len(m.Children))
		}
		if m.Changes == nil {
			t.Fatal("expected non-nil Changes slice")
		}
		if len(m.Changes) != 0 {
			t.Errorf("expected empty Changes, got %d", len(m.Changes))
		}
	})

	t.Run("creates named item", func(t *testing.T) {
		m := NewModuleItem("network")
		if m.Name != "network" {
			t.Errorf("expected name 'network', got %q", m.Name)
		}
	})
}

func TestModuleItemIsRoot(t *testing.T) {
	tests := []struct {
		name     string
		modName  string
		expected bool
	}{
		{"empty name is root", "", true},
		{"non-empty name is not root", "network", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModuleItem(tt.modName)
			if m.IsRoot() != tt.expected {
				t.Errorf("expected IsRoot() = %v, got %v", tt.expected, m.IsRoot())
			}
		})
	}
}

func TestModuleItemAddChange(t *testing.T) {
	t.Run("adds change to root module", func(t *testing.T) {
		root := NewModuleItem("")
		change := DriftChange{
			Address:      "azurerm_resource_group.main",
			ModulePath:   nil,
			ResourceType: "azurerm_resource_group",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"location": "westus"},
		}
		root.AddChange(change)

		if len(root.Changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(root.Changes))
		}
		if root.Changes[0].Address != "azurerm_resource_group.main" {
			t.Errorf("expected address 'azurerm_resource_group.main', got %q", root.Changes[0].Address)
		}
		if len(root.Children) != 0 {
			t.Errorf("expected no children, got %d", len(root.Children))
		}
	})

	t.Run("adds change to child module", func(t *testing.T) {
		root := NewModuleItem("")
		change := DriftChange{
			Address:      "module.network.azurerm_virtual_network.vnet",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "vnet",
			ChangedAttrs: map[string]interface{}{"name": "changed"},
		}
		root.AddChange(change)

		if len(root.Changes) != 0 {
			t.Errorf("expected 0 changes on root, got %d", len(root.Changes))
		}
		if len(root.Children) != 1 {
			t.Fatalf("expected 1 child, got %d", len(root.Children))
		}
		child, ok := root.Children["network"]
		if !ok {
			t.Fatal("expected child 'network'")
		}
		if child.Name != "network" {
			t.Errorf("expected child name 'network', got %q", child.Name)
		}
		if len(child.Changes) != 1 {
			t.Fatalf("expected 1 change on child, got %d", len(child.Changes))
		}
	})

	t.Run("adds change to deeply nested module", func(t *testing.T) {
		root := NewModuleItem("")
		change := DriftChange{
			Address:      "module.network.module.subnet.azurerm_subnet.main",
			ModulePath:   []string{"network", "subnet"},
			ResourceType: "azurerm_subnet",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"address_prefixes": []interface{}{"10.0.1.0/24"}},
		}
		root.AddChange(change)

		network, ok := root.Children["network"]
		if !ok {
			t.Fatal("expected child 'network'")
		}
		if len(network.Changes) != 0 {
			t.Errorf("expected 0 changes on 'network', got %d", len(network.Changes))
		}
		subnet, ok := network.Children["subnet"]
		if !ok {
			t.Fatal("expected child 'subnet' under 'network'")
		}
		if len(subnet.Changes) != 1 {
			t.Fatalf("expected 1 change on 'subnet', got %d", len(subnet.Changes))
		}
	})

	t.Run("multiple changes to same module share the child node", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "module.network.azurerm_virtual_network.vnet1",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "vnet1",
			ChangedAttrs: map[string]interface{}{"name": "vnet1"},
		})
		root.AddChange(DriftChange{
			Address:      "module.network.azurerm_virtual_network.vnet2",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "vnet2",
			ChangedAttrs: map[string]interface{}{"name": "vnet2"},
		})

		if len(root.Children) != 1 {
			t.Fatalf("expected 1 child, got %d", len(root.Children))
		}
		network := root.Children["network"]
		if len(network.Changes) != 2 {
			t.Errorf("expected 2 changes on 'network', got %d", len(network.Changes))
		}
	})
}

func TestModuleItemToHCL(t *testing.T) {
	t.Run("root with changes generates override block with comments", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "azurerm_resource_group.main",
			ModulePath:   nil,
			ResourceType: "azurerm_resource_group",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"location": "westus"},
		})

		tokens := root.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		expectedPatterns := []string{
			"# Generated by graft absorb",
			"# This manifest contains overrides to match the current remote state",
			"override {",
			`resource "azurerm_resource_group" "main"`,
			`location = "westus"`,
			"# Absorb drift for: azurerm_resource_group.main",
		}
		for _, pattern := range expectedPatterns {
			if !strings.Contains(result, pattern) {
				t.Errorf("expected output to contain %q, got:\n%s", pattern, result)
			}
		}
	})

	t.Run("non-root does not include generated-by comment", func(t *testing.T) {
		child := NewModuleItem("network")
		child.Changes = append(child.Changes, DriftChange{
			Address:      "module.network.azurerm_virtual_network.vnet",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "vnet",
			ChangedAttrs: map[string]interface{}{"name": "changed"},
		})

		tokens := child.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		if strings.Contains(result, "# Generated by graft absorb") {
			t.Errorf("non-root should not contain generated-by comment, got:\n%s", result)
		}
		if !strings.Contains(result, "override {") {
			t.Errorf("expected override block, got:\n%s", result)
		}
	})

	t.Run("root with children generates module blocks", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "module.network.azurerm_virtual_network.vnet",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "vnet",
			ChangedAttrs: map[string]interface{}{"name": "changed"},
		})

		tokens := root.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		if !strings.Contains(result, `module "network"`) {
			t.Errorf("expected module block, got:\n%s", result)
		}
		if !strings.Contains(result, "override {") {
			t.Errorf("expected override block inside module, got:\n%s", result)
		}
	})

	t.Run("changes are sorted by resource type then name", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "azurerm_virtual_network.beta",
			ResourceType: "azurerm_virtual_network",
			ResourceName: "beta",
			ChangedAttrs: map[string]interface{}{"name": "beta"},
		})
		root.AddChange(DriftChange{
			Address:      "azurerm_resource_group.main",
			ResourceType: "azurerm_resource_group",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"location": "westus"},
		})
		root.AddChange(DriftChange{
			Address:      "azurerm_virtual_network.alpha",
			ResourceType: "azurerm_virtual_network",
			ResourceName: "alpha",
			ChangedAttrs: map[string]interface{}{"name": "alpha"},
		})

		tokens := root.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		// azurerm_resource_group should come before azurerm_virtual_network
		rgIdx := strings.Index(result, "azurerm_resource_group")
		vnIdx := strings.Index(result, "azurerm_virtual_network")
		if rgIdx >= vnIdx {
			t.Errorf("expected azurerm_resource_group before azurerm_virtual_network, got:\n%s", result)
		}

		// alpha should come before beta
		alphaIdx := strings.Index(result, `"alpha"`)
		betaIdx := strings.Index(result, `"beta"`)
		if alphaIdx >= betaIdx {
			t.Errorf("expected alpha before beta, got:\n%s", result)
		}
	})

	t.Run("children modules are sorted alphabetically", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "module.storage.azurerm_storage_account.main",
			ModulePath:   []string{"storage"},
			ResourceType: "azurerm_storage_account",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"name": "sa"},
		})
		root.AddChange(DriftChange{
			Address:      "module.network.azurerm_virtual_network.main",
			ModulePath:   []string{"network"},
			ResourceType: "azurerm_virtual_network",
			ResourceName: "main",
			ChangedAttrs: map[string]interface{}{"name": "vnet"},
		})

		tokens := root.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		netIdx := strings.Index(result, `module "network"`)
		storIdx := strings.Index(result, `module "storage"`)
		if netIdx >= storIdx {
			t.Errorf("expected network before storage, got:\n%s", result)
		}
	})

	t.Run("skips changes where all attrs are computed", func(t *testing.T) {
		root := NewModuleItem("")
		root.AddChange(DriftChange{
			Address:      "azurerm_resource_group.main",
			ResourceType: "azurerm_resource_group",
			ResourceName: "main",
			ProviderName: "registry.terraform.io/hashicorp/azurerm",
			ChangedAttrs: map[string]interface{}{
				"id": "/some/id",
			},
		})

		schemas := &tfjson.ProviderSchemas{
			Schemas: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/azurerm": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"azurerm_resource_group": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"id": {Computed: true},
								},
							},
						},
					},
				},
			},
		}

		tokens := root.ToHCL(schemas)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		// override block should not contain any resource blocks since all are computed
		if strings.Contains(result, `resource "azurerm_resource_group"`) {
			t.Errorf("expected computed-only change to be skipped, got:\n%s", result)
		}
	})

	t.Run("empty root produces only comments", func(t *testing.T) {
		root := NewModuleItem("")

		tokens := root.ToHCL(nil)
		f := hclwrite.NewEmptyFile()
		f.Body().AppendUnstructuredTokens(tokens)
		result := string(hclwrite.Format(f.Bytes()))

		if !strings.Contains(result, "# Generated by graft absorb") {
			t.Errorf("expected generated-by comment, got:\n%s", result)
		}
		if strings.Contains(result, "override {") {
			t.Errorf("expected no override block for empty root, got:\n%s", result)
		}
		if strings.Contains(result, "module ") {
			t.Errorf("expected no module block for empty root, got:\n%s", result)
		}
	})
}
