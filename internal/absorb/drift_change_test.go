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

func TestInterfaceToCtyValueConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected cty.Value
	}{
		{
			name:     "nil",
			input:    nil,
			expected: cty.NullVal(cty.DynamicPseudoType),
		},
		{
			name:     "bool true",
			input:    true,
			expected: cty.True,
		},
		{
			name:     "bool false",
			input:    false,
			expected: cty.False,
		},
		{
			name:     "integer",
			input:    float64(42),
			expected: cty.NumberIntVal(42),
		},
		{
			name:     "zero",
			input:    float64(0),
			expected: cty.NumberIntVal(0),
		},
		{
			name:     "negative integer",
			input:    float64(-5),
			expected: cty.NumberIntVal(-5),
		},
		{
			name:     "float",
			input:    3.14,
			expected: cty.NumberFloatVal(3.14),
		},
		{
			name:     "string",
			input:    "hello",
			expected: cty.StringVal("hello"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: cty.StringVal(""),
		},
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: cty.ListValEmpty(cty.DynamicPseudoType),
		},
		{
			name:  "slice of strings",
			input: []interface{}{"a", "b", "c"},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b"),
				cty.StringVal("c"),
			}),
		},
		{
			name:  "mixed slice",
			input: []interface{}{"text", float64(1), true},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("text"),
				cty.NumberIntVal(1),
				cty.True,
			}),
		},
		{
			name:  "slice with nil",
			input: []interface{}{"a", nil, "c"},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.NullVal(cty.DynamicPseudoType),
				cty.StringVal("c"),
			}),
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: cty.MapValEmpty(cty.DynamicPseudoType),
		},
		{
			name: "map",
			input: map[string]interface{}{
				"name": "test",
				"port": float64(8080),
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"name": cty.StringVal("test"),
				"port": cty.NumberIntVal(8080),
			}),
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "deep",
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"outer": cty.ObjectVal(map[string]cty.Value{
					"inner": cty.StringVal("deep"),
				}),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interfaceToCtyValue(tt.input)
			if !got.RawEquals(tt.expected) {
				t.Errorf("interfaceToCtyValue(%v) = %#v, want %#v", tt.input, got, tt.expected)
			}
		})
	}
}
