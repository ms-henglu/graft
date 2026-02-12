package absorb

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

func TestToBlock(t *testing.T) {
	tests := []struct {
		name     string
		change   DriftChange
		schema   *tfjson.SchemaBlock
		expected string
	}{
		{
			name: "simple attributes without schema",
			change: DriftChange{
				ResourceType: "azurerm_resource_group",
				ResourceName: "main",
				ChangedAttrs: map[string]interface{}{
					"location": "eastus",
					"tags":     map[string]interface{}{"Env": "Prod"},
				},
			},
			schema: nil,
			expected: `resource "azurerm_resource_group" "main" {
  location = "eastus"
  tags = {
    Env = "Prod"
  }
}
`,
		},
		{
			name: "filters computed-only attributes",
			change: DriftChange{
				ResourceType: "azurerm_virtual_network",
				ResourceName: "vnet",
				ChangedAttrs: map[string]interface{}{
					"name": "changed",
					"guid": "computed-value",
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Optional: true},
					"guid": {Computed: true},
				},
			},
			expected: `resource "azurerm_virtual_network" "vnet" {
  name = "changed"
}
`,
		},
		{
			name: "nil when all attributes are computed",
			change: DriftChange{
				ResourceType: "azurerm_resource_group",
				ResourceName: "main",
				ChangedAttrs: map[string]interface{}{
					"id":   "/some/id",
					"guid": "some-guid",
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"id":   {Computed: true},
					"guid": {Computed: true},
				},
			},
		},
		{
			name: "nil for empty changed attrs",
			change: DriftChange{
				ResourceType: "azurerm_resource_group",
				ResourceName: "main",
				ChangedAttrs: map[string]interface{}{},
			},
			schema: nil,
		},
		{
			name: "nil for nil changed attrs",
			change: DriftChange{
				ResourceType: "azurerm_resource_group",
				ResourceName: "main",
				ChangedAttrs: nil,
			},
			schema: nil,
		},
		{
			name: "renders nested block from block_type schema",
			change: DriftChange{
				ResourceType: "azurerm_virtual_network",
				ResourceName: "vnet",
				ChangedAttrs: map[string]interface{}{
					"ddos_protection_plan": []interface{}{
						map[string]interface{}{
							"enable": true,
							"id":     "/some/id",
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"ddos_protection_plan": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"enable": {Required: true},
								"id":     {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_virtual_network" "vnet" {
  ddos_protection_plan {
    enable = true
    id     = "/some/id"
  }
}
`,
		},
		{
			name: "multiple blocks trigger _graft removal",
			change: DriftChange{
				ResourceType: "azurerm_virtual_network",
				ResourceName: "vnet",
				ChangedAttrs: map[string]interface{}{
					"subnet": []interface{}{
						map[string]interface{}{"name": "a"},
						map[string]interface{}{"name": "b"},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"subnet": {
						AttributeType: cty.Set(cty.Object(map[string]cty.Type{
							"name": cty.String,
						})),
						Optional: true,
						Computed: true,
					},
				},
			},
			expected: `resource "azurerm_virtual_network" "vnet" {
  subnet {
    name = "a"
  }
  subnet {
    name = "b"
  }
  _graft {
    remove = ["subnet"]
  }
}
`,
		},
		{
			name: "single block does not trigger _graft removal",
			change: DriftChange{
				ResourceType: "azurerm_virtual_network",
				ResourceName: "vnet",
				ChangedAttrs: map[string]interface{}{
					"subnet": []interface{}{
						map[string]interface{}{"name": "only-one"},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"subnet": {
						AttributeType: cty.Set(cty.Object(map[string]cty.Type{
							"name": cty.String,
						})),
						Optional: true,
						Computed: true,
					},
				},
			},
			expected: `resource "azurerm_virtual_network" "vnet" {
  subnet {
    name = "only-one"
  }
}
`,
		},
		{
			name: "multiple block-type attributes collect removals sorted",
			change: DriftChange{
				ResourceType: "azurerm_network_security_group",
				ResourceName: "nsg",
				ChangedAttrs: map[string]interface{}{
					"security_rule": []interface{}{
						map[string]interface{}{"name": "rule1"},
						map[string]interface{}{"name": "rule2"},
					},
					"inbound_rule": []interface{}{
						map[string]interface{}{"protocol": "tcp"},
						map[string]interface{}{"protocol": "udp"},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"security_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {Required: true},
							},
						},
					},
					"inbound_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"protocol": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_network_security_group" "nsg" {
  inbound_rule {
    protocol = "tcp"
  }
  inbound_rule {
    protocol = "udp"
  }
  security_rule {
    name = "rule1"
  }
  security_rule {
    name = "rule2"
  }
  _graft {
    remove = ["inbound_rule", "security_rule"]
  }
}
`,
		},
		{
			name: "nested block inside nested block",
			change: DriftChange{
				ResourceType: "azurerm_linux_virtual_machine",
				ResourceName: "vm",
				ChangedAttrs: map[string]interface{}{
					"os_disk": map[string]interface{}{
						"diff_disk_settings": map[string]interface{}{
							"option": "Local",
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"diff_disk_settings": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"option":    {Required: true},
											"placement": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  os_disk {
    diff_disk_settings {
      option = "Local"
    }
  }
}
`,
		},
		{
			name: "many inner blocks inside nested block",
			change: DriftChange{
				ResourceType: "azurerm_application_gateway",
				ResourceName: "gw",
				ChangedAttrs: map[string]interface{}{
					"backend_http_settings": map[string]interface{}{
						"connection_draining": []interface{}{
							map[string]interface{}{"enabled": true, "drain_timeout_sec": float64(30)},
							map[string]interface{}{"enabled": false, "drain_timeout_sec": float64(60)},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"backend_http_settings": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"connection_draining": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"enabled":           {Required: true},
											"drain_timeout_sec": {Required: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_application_gateway" "gw" {
  backend_http_settings {
    connection_draining {
      drain_timeout_sec = 30
      enabled           = true
    }
    connection_draining {
      drain_timeout_sec = 60
      enabled           = false
    }
  }
  _graft {
    remove = ["backend_http_settings.connection_draining"]
  }
}
`,
		},
		{
			name: "nested list blocks inside single block trigger dotted-path removal",
			change: DriftChange{
				ResourceType: "azurerm_application_gateway",
				ResourceName: "gw",
				ChangedAttrs: map[string]interface{}{
					"backend_http_settings": map[string]interface{}{
						"name": "settings1",
						"connection_draining": []interface{}{
							map[string]interface{}{"enabled": true, "drain_timeout_sec": float64(30)},
							map[string]interface{}{"enabled": false, "drain_timeout_sec": float64(60)},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"backend_http_settings": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {Required: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"connection_draining": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"enabled":           {Required: true},
											"drain_timeout_sec": {Required: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_application_gateway" "gw" {
  backend_http_settings {
    connection_draining {
      drain_timeout_sec = 30
      enabled           = true
    }
    connection_draining {
      drain_timeout_sec = 60
      enabled           = false
    }
    name = "settings1"
  }
  _graft {
    remove = ["backend_http_settings.connection_draining"]
  }
}
`,
		},
		{
			name: "many outer blocks each with nested block",
			change: DriftChange{
				ResourceType: "azurerm_application_gateway",
				ResourceName: "gw",
				ChangedAttrs: map[string]interface{}{
					"backend_http_settings": []interface{}{
						map[string]interface{}{
							"name": "settings1",
							"connection_draining": map[string]interface{}{
								"enabled":           true,
								"drain_timeout_sec": float64(30),
							},
						},
						map[string]interface{}{
							"name": "settings2",
							"connection_draining": map[string]interface{}{
								"enabled":           false,
								"drain_timeout_sec": float64(60),
							},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"backend_http_settings": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {Required: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"connection_draining": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"enabled":           {Required: true},
											"drain_timeout_sec": {Required: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_application_gateway" "gw" {
  backend_http_settings {
    connection_draining {
      drain_timeout_sec = 30
      enabled           = true
    }
    name = "settings1"
  }
  backend_http_settings {
    connection_draining {
      drain_timeout_sec = 60
      enabled           = false
    }
    name = "settings2"
  }
  _graft {
    remove = ["backend_http_settings"]
  }
}
`,
		},
		{
			name: "mixed block and attribute keys",
			change: DriftChange{
				ResourceType: "azurerm_linux_virtual_machine",
				ResourceName: "vm",
				ChangedAttrs: map[string]interface{}{
					"name": "my-vm",
					"os_disk": map[string]interface{}{
						"caching": "ReadWrite",
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Required: true},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  name = "my-vm"
  os_disk {
    caching = "ReadWrite"
  }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := tt.change.ToBlock(tt.schema)
			if tt.expected == "" {
				if block != nil {
					t.Errorf("expected nil block, got non-nil")
				}
				return
			}
			if block == nil {
				t.Fatal("expected non-nil block, got nil")
			}

			f := hclwrite.NewEmptyFile()
			f.Body().AppendBlock(block)
			result := string(hclwrite.Format(f.Bytes()))

			if result != tt.expected {
				t.Errorf("unexpected output:\n got:\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestDeepDiffBlock(t *testing.T) {
	tests := []struct {
		name     string
		before   map[string]interface{}
		after    map[string]interface{}
		schema   *tfjson.SchemaBlock
		expected map[string]interface{}
	}{
		{
			name: "single block recursion - only changed nested attr",
			before: map[string]interface{}{
				"os_disk": map[string]interface{}{
					"caching":      "ReadOnly",
					"disk_size_gb": float64(30),
					"diff_disk_settings": map[string]interface{}{
						"option":    "Local",
						"placement": "CacheDisk",
					},
				},
			},
			after: map[string]interface{}{
				"os_disk": map[string]interface{}{
					"caching":      "ReadOnly",
					"disk_size_gb": float64(30),
					"diff_disk_settings": map[string]interface{}{
						"option":    "None",
						"placement": "CacheDisk",
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching":      {Optional: true},
								"disk_size_gb": {Optional: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"diff_disk_settings": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"option":    {Required: true},
											"placement": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"os_disk": map[string]interface{}{
					"diff_disk_settings": map[string]interface{}{
						"option": "None",
					},
				},
			},
		},
		{
			name: "multiple blocks (slice) - capture full array",
			before: map[string]interface{}{
				"security_rule": []interface{}{
					map[string]interface{}{"name": "rule1", "priority": float64(100)},
				},
			},
			after: map[string]interface{}{
				"security_rule": []interface{}{
					map[string]interface{}{"name": "rule1", "priority": float64(100)},
					map[string]interface{}{"name": "rule2", "priority": float64(200)},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"security_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name":     {Required: true},
								"priority": {Required: true},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"security_rule": []interface{}{
					map[string]interface{}{"name": "rule1", "priority": float64(100)},
					map[string]interface{}{"name": "rule2", "priority": float64(200)},
				},
			},
		},
		{
			name: "plain map attribute (tags) - capture full value",
			before: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Dev", "Team": "Infra"},
			},
			after: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Prod", "Team": "Infra"},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"tags": {Optional: true, AttributeType: cty.Map(cty.String)},
				},
			},
			expected: map[string]interface{}{
				"tags": map[string]interface{}{"Env": "Prod", "Team": "Infra"},
			},
		},
		{
			name: "scalar attribute changed",
			before: map[string]interface{}{
				"name": "old-name",
			},
			after: map[string]interface{}{
				"name": "new-name",
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Required: true},
				},
			},
			expected: map[string]interface{}{
				"name": "new-name",
			},
		},
		{
			name: "no changes - returns nil",
			before: map[string]interface{}{
				"name":     "same",
				"location": "eastus",
			},
			after: map[string]interface{}{
				"name":     "same",
				"location": "eastus",
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name":     {Required: true},
					"location": {Required: true},
				},
			},
			expected: nil,
		},
		{
			name: "mixed scenario - single block + scalar + unchanged",
			before: map[string]interface{}{
				"name": "my-vm",
				"size": "Standard_DS2_v2",
				"os_disk": map[string]interface{}{
					"caching":      "ReadOnly",
					"disk_size_gb": float64(30),
				},
			},
			after: map[string]interface{}{
				"name": "my-vm",
				"size": "Standard_DS3_v2",
				"os_disk": map[string]interface{}{
					"caching":      "ReadWrite",
					"disk_size_gb": float64(30),
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Required: true},
					"size": {Required: true},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching":      {Required: true},
								"disk_size_gb": {Optional: true},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"size": "Standard_DS3_v2",
				"os_disk": map[string]interface{}{
					"caching": "ReadWrite",
				},
			},
		},
		{
			name:   "new attribute not in before - captured",
			before: map[string]interface{}{},
			after: map[string]interface{}{
				"new_attr": "value",
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"new_attr": {Optional: true},
				},
			},
			expected: map[string]interface{}{
				"new_attr": "value",
			},
		},
		{
			name:   "empty after - returns nil",
			before: map[string]interface{}{"name": "test"},
			after:  map[string]interface{}{},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Required: true},
				},
			},
			expected: nil,
		},
		{
			name: "single block unchanged entirely - skipped",
			before: map[string]interface{}{
				"os_disk": map[string]interface{}{
					"caching": "ReadOnly",
				},
			},
			after: map[string]interface{}{
				"os_disk": map[string]interface{}{
					"caching": "ReadOnly",
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Required: true},
							},
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepDiffBlock(tt.before, tt.after, tt.schema)
			if !deepEqual(got, tt.expected) {
				t.Errorf("deepDiffBlock() =\n  %v\nwant:\n  %v", got, tt.expected)
			}
		})
	}
}

func TestToBlockWithBeforeAttrs(t *testing.T) {
	tests := []struct {
		name     string
		change   DriftChange
		schema   *tfjson.SchemaBlock
		expected string
	}{
		{
			name: "minimal diff for single nested block",
			change: DriftChange{
				ResourceType: "azurerm_linux_virtual_machine",
				ResourceName: "vm",
				BeforeAttrs: map[string]interface{}{
					"os_disk": map[string]interface{}{
						"caching":      "ReadOnly",
						"disk_size_gb": float64(30),
						"diff_disk_settings": map[string]interface{}{
							"option":    "Local",
							"placement": "CacheDisk",
						},
					},
				},
				ChangedAttrs: map[string]interface{}{
					"os_disk": map[string]interface{}{
						"caching":      "ReadOnly",
						"disk_size_gb": float64(30),
						"diff_disk_settings": map[string]interface{}{
							"option":    "None",
							"placement": "CacheDisk",
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching":      {Optional: true},
								"disk_size_gb": {Optional: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"diff_disk_settings": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"option":    {Required: true},
											"placement": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  os_disk {
    diff_disk_settings {
      option = "None"
    }
  }
}
`,
		},
		{
			name: "multiple blocks still capture full array with _graft removal",
			change: DriftChange{
				ResourceType: "azurerm_network_security_group",
				ResourceName: "nsg",
				BeforeAttrs: map[string]interface{}{
					"security_rule": []interface{}{
						map[string]interface{}{"name": "rule1"},
					},
				},
				ChangedAttrs: map[string]interface{}{
					"security_rule": []interface{}{
						map[string]interface{}{"name": "rule1"},
						map[string]interface{}{"name": "rule2"},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"security_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_network_security_group" "nsg" {
  security_rule {
    name = "rule1"
  }
  security_rule {
    name = "rule2"
  }
  _graft {
    remove = ["security_rule"]
  }
}
`,
		},
		{
			name: "mixed: changed scalar + unchanged block = only scalar in output",
			change: DriftChange{
				ResourceType: "azurerm_linux_virtual_machine",
				ResourceName: "vm",
				BeforeAttrs: map[string]interface{}{
					"name": "old-name",
					"os_disk": map[string]interface{}{
						"caching": "ReadOnly",
					},
				},
				ChangedAttrs: map[string]interface{}{
					"name": "new-name",
					"os_disk": map[string]interface{}{
						"caching": "ReadOnly",
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Required: true},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  name = "new-name"
}
`,
		},
		{
			name: "nil BeforeAttrs - no deep diff applied, full output",
			change: DriftChange{
				ResourceType: "azurerm_linux_virtual_machine",
				ResourceName: "vm",
				BeforeAttrs:  nil,
				ChangedAttrs: map[string]interface{}{
					"os_disk": map[string]interface{}{
						"caching":      "ReadOnly",
						"disk_size_gb": float64(30),
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching":      {Optional: true},
								"disk_size_gb": {Optional: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  os_disk {
    caching      = "ReadOnly"
    disk_size_gb = 30
  }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := tt.change.ToBlock(tt.schema)
			if tt.expected == "" {
				if block != nil {
					t.Errorf("expected nil block, got non-nil")
				}
				return
			}
			if block == nil {
				t.Fatal("expected non-nil block, got nil")
			}

			f := hclwrite.NewEmptyFile()
			f.Body().AppendBlock(block)
			result := string(hclwrite.Format(f.Bytes()))

			if result != tt.expected {
				t.Errorf("unexpected output:\n got:\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestIndexedChangesToBlockWithBlockDrift(t *testing.T) {
	tests := []struct {
		name     string
		changes  []DriftChange
		schema   *tfjson.SchemaBlock
		expected string
	}{
		{
			name: "category 3: multiple sibling blocks with count",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        float64(0),
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "rule-a", "priority": float64(100)},
						},
					},
				},
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        float64(1),
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "rule-b", "priority": float64(200)},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"security_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name":     {Required: true},
								"priority": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_network_security_group" "nsg" {
  dynamic "security_rule" {
    for_each = lookup({
      "0" = [{
        name     = "rule-a"
        priority = 100
      }]
      "1" = [{
        name     = "rule-b"
        priority = 200
      }]
    }, count.index, [])
    content {
      name     = security_rule.value.name
      priority = security_rule.value.priority
    }
  }
  _graft {
    remove = ["security_rule"]
  }
}
`,
		},
		{
			name: "category 3: multiple sibling blocks with for_each",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        "web",
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "allow-http", "priority": float64(100)},
							map[string]interface{}{"name": "allow-https", "priority": float64(200)},
						},
					},
				},
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        "api",
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "allow-grpc", "priority": float64(300)},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"security_rule": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name":     {Required: true},
								"priority": {Required: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_network_security_group" "nsg" {
  dynamic "security_rule" {
    for_each = lookup({
      api = [{
        name     = "allow-grpc"
        priority = 300
      }]
      web = [{
        name     = "allow-http"
        priority = 100
        }, {
        name     = "allow-https"
        priority = 200
      }]
    }, each.key, [])
    content {
      name     = security_rule.value.name
      priority = security_rule.value.priority
    }
  }
  _graft {
    remove = ["security_rule"]
  }
}
`,
		},
		{
			name: "category 2: single nested block with count",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(0),
					BeforeAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching":      "ReadOnly",
							"disk_size_gb": float64(30),
						},
					},
					ChangedAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching":      "ReadWrite",
							"disk_size_gb": float64(30),
						},
					},
				},
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(1),
					BeforeAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching":      "ReadOnly",
							"disk_size_gb": float64(30),
						},
					},
					ChangedAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching":      "ReadOnly",
							"disk_size_gb": float64(50),
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching":      {Optional: true},
								"disk_size_gb": {Optional: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  dynamic "os_disk" {
    for_each = lookup({
      "0" = [{
        caching      = "ReadWrite"
        disk_size_gb = 30
      }]
      "1" = [{
        caching      = "ReadOnly"
        disk_size_gb = 50
      }]
    }, count.index, [])
    content {
      caching      = os_disk.value.caching
      disk_size_gb = os_disk.value.disk_size_gb
    }
  }
  _graft {
    remove = ["os_disk"]
  }
}
`,
		},
		{
			name: "mixed: attribute drift + block drift on indexed resources",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(0),
					ChangedAttrs: map[string]interface{}{
						"size": "Standard_DS2_v2",
						"os_disk": map[string]interface{}{
							"caching": "ReadWrite",
						},
					},
				},
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(1),
					ChangedAttrs: map[string]interface{}{
						"size": "Standard_DS3_v2",
						"os_disk": map[string]interface{}{
							"caching": "ReadOnly",
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"size": {Optional: true},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Optional: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  size = lookup({
    "0" = "Standard_DS2_v2"
    "1" = "Standard_DS3_v2"
  }, count.index, graft.source)
  dynamic "os_disk" {
    for_each = lookup({
      "0" = [{ caching = "ReadWrite" }]
      "1" = [{ caching = "ReadOnly" }]
    }, count.index, [])
    content {
      caching = os_disk.value.caching
    }
  }
  _graft {
    remove = ["os_disk"]
  }
}
`,
		},
		{
			name: "category 2: single block with nested block inside",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(0),
					BeforeAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching": "ReadOnly",
							"diff_disk_settings": map[string]interface{}{
								"option":    "Local",
								"placement": "CacheDisk",
							},
						},
					},
					ChangedAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching": "ReadWrite",
							"diff_disk_settings": map[string]interface{}{
								"option":    "None",
								"placement": "CacheDisk",
							},
						},
					},
				},
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(1),
					BeforeAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching": "ReadOnly",
							"diff_disk_settings": map[string]interface{}{
								"option":    "Local",
								"placement": "CacheDisk",
							},
						},
					},
					ChangedAttrs: map[string]interface{}{
						"os_disk": map[string]interface{}{
							"caching": "ReadOnly",
							"diff_disk_settings": map[string]interface{}{
								"option":    "Local",
								"placement": "ResourceDisk",
							},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Optional: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"diff_disk_settings": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"option":    {Required: true},
											"placement": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  dynamic "os_disk" {
    for_each = lookup({
      "0" = [{
        caching = "ReadWrite"
        diff_disk_settings = [{
          option    = "None"
          placement = "CacheDisk"
        }]
      }]
      "1" = [{
        caching = "ReadOnly"
        diff_disk_settings = [{
          option    = "Local"
          placement = "ResourceDisk"
        }]
      }]
    }, count.index, [])
    content {
      caching = os_disk.value.caching
      dynamic "diff_disk_settings" {
        for_each = try(os_disk.value.diff_disk_settings, [])
        content {
          option    = diff_disk_settings.value.option
          placement = diff_disk_settings.value.placement
        }
      }
    }
  }
  _graft {
    remove = ["os_disk"]
  }
}
`,
		},
		{
			name: "block drift filters computed-only attributes",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        float64(0),
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "rule-a", "priority": float64(100), "id": "/computed/id/0"},
						},
					},
				},
				{
					ResourceType: "azurerm_network_security_group",
					ResourceName: "nsg",
					Index:        float64(1),
					ChangedAttrs: map[string]interface{}{
						"security_rule": []interface{}{
							map[string]interface{}{"name": "rule-b", "priority": float64(200), "id": "/computed/id/1"},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"security_rule": {
						AttributeType: cty.Set(cty.Object(map[string]cty.Type{
							"name":     cty.String,
							"priority": cty.Number,
							"id":       cty.String,
						})),
						Optional: true,
						Computed: true,
					},
				},
			},
			expected: `resource "azurerm_network_security_group" "nsg" {
  dynamic "security_rule" {
    for_each = lookup({
      "0" = [{
        name     = "rule-a"
        priority = 100
      }]
      "1" = [{
        name     = "rule-b"
        priority = 200
      }]
    }, count.index, [])
    content {
      name     = security_rule.value.name
      priority = security_rule.value.priority
    }
  }
  _graft {
    remove = ["security_rule"]
  }
}
`,
		},
		{
			name: "partial block drift — only some instances have block changes",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(0),
					ChangedAttrs: map[string]interface{}{
						"size": "Standard_DS2_v2",
						"os_disk": map[string]interface{}{
							"caching": "ReadWrite",
						},
					},
				},
				{
					ResourceType: "azurerm_linux_virtual_machine",
					ResourceName: "vm",
					Index:        float64(1),
					ChangedAttrs: map[string]interface{}{
						"size": "Standard_DS3_v2",
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"size": {Optional: true},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"os_disk": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"caching": {Optional: true},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_linux_virtual_machine" "vm" {
  size = lookup({
    "0" = "Standard_DS2_v2"
    "1" = "Standard_DS3_v2"
  }, count.index, graft.source)
  dynamic "os_disk" {
    for_each = lookup({
      "0" = [{ caching = "ReadWrite" }]
    }, count.index, [])
    content {
      caching = os_disk.value.caching
    }
  }
  _graft {
    remove = ["os_disk"]
  }
}
`,
		},
		{
			name: "multi-layer nested blocks with for_each",
			changes: []DriftChange{
				{
					ResourceType: "azurerm_storage_account",
					ResourceName: "each_block",
					Index:        "alpha",
					ChangedAttrs: map[string]interface{}{
						"blob_properties": []interface{}{
							map[string]interface{}{
								"change_feed_enabled":           false,
								"change_feed_retention_in_days": float64(0),
								"container_delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(3)},
								},
								"default_service_version": nil,
								"delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(7), "permanent_delete_enabled": false},
								},
								"last_access_time_enabled": false,
								"versioning_enabled":       true,
							},
						},
					},
				},
				{
					ResourceType: "azurerm_storage_account",
					ResourceName: "each_block",
					Index:        "beta",
					ChangedAttrs: map[string]interface{}{
						"blob_properties": []interface{}{
							map[string]interface{}{
								"change_feed_enabled":           false,
								"change_feed_retention_in_days": float64(0),
								"container_delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(15)},
								},
								"default_service_version": nil,
								"delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(7), "permanent_delete_enabled": false},
								},
								"last_access_time_enabled": false,
								"versioning_enabled":       true,
							},
						},
					},
				},
				{
					ResourceType: "azurerm_storage_account",
					ResourceName: "each_block",
					Index:        "gamma",
					ChangedAttrs: map[string]interface{}{
						"blob_properties": []interface{}{
							map[string]interface{}{
								"change_feed_enabled":           false,
								"change_feed_retention_in_days": float64(0),
								"container_delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(3)},
								},
								"default_service_version": nil,
								"delete_retention_policy": []interface{}{
									map[string]interface{}{"days": float64(30), "permanent_delete_enabled": false},
								},
								"last_access_time_enabled": false,
								"versioning_enabled":       false,
							},
						},
					},
				},
			},
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"blob_properties": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"change_feed_enabled":           {Optional: true},
								"change_feed_retention_in_days": {Optional: true},
								"default_service_version":       {Optional: true},
								"last_access_time_enabled":      {Optional: true},
								"versioning_enabled":            {Optional: true},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"container_delete_retention_policy": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"days": {Optional: true},
										},
									},
								},
								"delete_retention_policy": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"days":                     {Optional: true},
											"permanent_delete_enabled": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: `resource "azurerm_storage_account" "each_block" {
  dynamic "blob_properties" {
    for_each = lookup({
      alpha = [{
        change_feed_enabled           = false
        change_feed_retention_in_days = 0
        container_delete_retention_policy = [{
          days = 3
        }]
        delete_retention_policy = [{
          days                     = 7
          permanent_delete_enabled = false
        }]
        last_access_time_enabled = false
        versioning_enabled       = true
      }]
      beta = [{
        change_feed_enabled           = false
        change_feed_retention_in_days = 0
        container_delete_retention_policy = [{
          days = 15
        }]
        delete_retention_policy = [{
          days                     = 7
          permanent_delete_enabled = false
        }]
        last_access_time_enabled = false
        versioning_enabled       = true
      }]
      gamma = [{
        change_feed_enabled           = false
        change_feed_retention_in_days = 0
        container_delete_retention_policy = [{
          days = 3
        }]
        delete_retention_policy = [{
          days                     = 30
          permanent_delete_enabled = false
        }]
        last_access_time_enabled = false
        versioning_enabled       = false
      }]
    }, each.key, [])
    content {
      change_feed_enabled           = blob_properties.value.change_feed_enabled
      change_feed_retention_in_days = blob_properties.value.change_feed_retention_in_days
      dynamic "container_delete_retention_policy" {
        for_each = try(blob_properties.value.container_delete_retention_policy, [])
        content {
          days = container_delete_retention_policy.value.days
        }
      }
      dynamic "delete_retention_policy" {
        for_each = try(blob_properties.value.delete_retention_policy, [])
        content {
          days                     = delete_retention_policy.value.days
          permanent_delete_enabled = delete_retention_policy.value.permanent_delete_enabled
        }
      }
      last_access_time_enabled = blob_properties.value.last_access_time_enabled
      versioning_enabled       = blob_properties.value.versioning_enabled
    }
  }
  _graft {
    remove = ["blob_properties"]
  }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &IndexedDriftChange{
				Changes: tt.changes,
			}
			block := c.ToBlock(tt.schema)
			if tt.expected == "" {
				if block != nil {
					t.Errorf("expected nil block, got non-nil")
				}
				return
			}
			if block == nil {
				t.Fatal("expected non-nil block, got nil")
			}

			f := hclwrite.NewEmptyFile()
			f.Body().AppendBlock(block)
			result := string(hclwrite.Format(f.Bytes()))

			if normalizeWhitespace(result) != normalizeWhitespace(tt.expected) {
				t.Errorf("unexpected output:\n got:\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}
