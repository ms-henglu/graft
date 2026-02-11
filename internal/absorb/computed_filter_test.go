package absorb

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

func TestIsAttrPathComputedOnly(t *testing.T) {
	tests := []struct {
		name     string
		schema   *tfjson.SchemaBlock
		attrPath string
		expected bool
	}{
		{
			name:     "nil schema",
			schema:   nil,
			attrPath: "name",
			expected: false,
		},
		{
			name: "empty attrPath",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Optional: true},
				},
			},
			attrPath: "",
			expected: false,
		},
		{
			name: "top-level computed-only attribute",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"guid": {Computed: true},
				},
			},
			attrPath: "guid",
			expected: true,
		},
		{
			name: "top-level optional attribute",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Optional: true},
				},
			},
			attrPath: "name",
			expected: false,
		},
		{
			name: "top-level optional+computed attribute",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"location": {Optional: true, Computed: true},
				},
			},
			attrPath: "location",
			expected: false,
		},
		{
			name: "attribute not in schema",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Optional: true},
				},
			},
			attrPath: "unknown",
			expected: false,
		},
		{
			name: "nested block computed-only field",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"encryption": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"status":      {Computed: true},
								"enforcement": {Required: true},
							},
						},
					},
				},
			},
			attrPath: "encryption.status",
			expected: true,
		},
		{
			name: "nested block non-computed field",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"encryption": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"status":      {Computed: true},
								"enforcement": {Required: true},
							},
						},
					},
				},
			},
			attrPath: "encryption.enforcement",
			expected: false,
		},
		{
			name: "nested block unknown field",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"encryption": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"status": {Computed: true},
							},
						},
					},
				},
			},
			attrPath: "encryption.unknown_field",
			expected: false,
		},
		{
			name: "nested attribute type computed-only field",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"identity": {
						Optional: true,
						Computed: true,
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeList,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"type":         {Required: true},
								"principal_id": {Computed: true},
								"tenant_id":    {Computed: true},
							},
						},
					},
				},
			},
			attrPath: "identity.principal_id",
			expected: true,
		},
		{
			name: "nested attribute type non-computed field",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"identity": {
						Optional: true,
						Computed: true,
						AttributeNestedType: &tfjson.SchemaNestedAttributeType{
							NestingMode: tfjson.SchemaNestingModeList,
							Attributes: map[string]*tfjson.SchemaAttribute{
								"type":         {Required: true},
								"principal_id": {Computed: true},
							},
						},
					},
				},
			},
			attrPath: "identity.type",
			expected: false,
		},
		{
			name: "cty attribute-as-block with id heuristic",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"subnet": {
						Optional: true,
						Computed: true,
						AttributeType: cty.Set(cty.Object(map[string]cty.Type{
							"name":             cty.String,
							"address_prefixes": cty.List(cty.String),
							"id":               cty.String,
						})),
					},
				},
			},
			attrPath: "subnet.id",
			expected: true,
		},
		{
			name: "cty attribute-as-block non-id field",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"subnet": {
						Optional: true,
						Computed: true,
						AttributeType: cty.Set(cty.Object(map[string]cty.Type{
							"name":             cty.String,
							"address_prefixes": cty.List(cty.String),
							"id":               cty.String,
						})),
					},
				},
			},
			attrPath: "subnet.name",
			expected: false,
		},
		{
			name: "deeply nested block path",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"network": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"security": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"rule_id": {Computed: true},
											"enabled": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			attrPath: "network.security.rule_id",
			expected: true,
		},
		{
			name: "deeply nested block non-computed",
			schema: &tfjson.SchemaBlock{
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"network": {
						NestingMode: tfjson.SchemaNestingModeList,
						Block: &tfjson.SchemaBlock{
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"security": {
									NestingMode: tfjson.SchemaNestingModeList,
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"rule_id": {Computed: true},
											"enabled": {Optional: true},
										},
									},
								},
							},
						},
					},
				},
			},
			attrPath: "network.security.enabled",
			expected: false,
		},
		{
			name: "intermediate segment not navigable",
			schema: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"name": {Optional: true},
				},
			},
			attrPath: "name.sub_field",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAttrPathComputedOnly(tt.schema, tt.attrPath)
			if result != tt.expected {
				t.Errorf("isAttrPathComputedOnly(%q) = %v, want %v", tt.attrPath, result, tt.expected)
			}
		})
	}
}

func TestFilterComputedAttrsComprehensive(t *testing.T) {
	t.Run("nil schema returns value unchanged", func(t *testing.T) {
		val := map[string]interface{}{"key": "value"}
		result := filterComputedAttrs(val, nil, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if resultMap["key"] != "value" {
			t.Errorf("expected 'value', got %v", resultMap["key"])
		}
	})

	t.Run("nil value returns nil", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{}
		result := filterComputedAttrs(nil, schema, "")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("filters top-level computed-only attributes", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"name": {Optional: true},
				"guid": {Computed: true},
				"tags": {Optional: true, Computed: true},
			},
		}
		val := map[string]interface{}{
			"name": "test",
			"guid": "abc-123",
			"tags": map[string]interface{}{"env": "prod"},
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if _, has := resultMap["guid"]; has {
			t.Error("expected 'guid' to be filtered out")
		}
		if _, has := resultMap["name"]; !has {
			t.Error("expected 'name' to be present")
		}
		if _, has := resultMap["tags"]; !has {
			t.Error("expected 'tags' to be present")
		}
	})

	t.Run("filters nested block computed-only fields", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			NestedBlocks: map[string]*tfjson.SchemaBlockType{
				"encryption": {
					NestingMode: tfjson.SchemaNestingModeList,
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"enforcement": {Required: true},
							"status":      {Computed: true},
						},
					},
				},
			},
		}
		val := map[string]interface{}{
			"encryption": []interface{}{
				map[string]interface{}{
					"enforcement": "enabled",
					"status":      "active",
				},
			},
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		enc, ok := resultMap["encryption"]
		if !ok {
			t.Fatal("expected 'encryption' in result")
		}
		arr, ok := enc.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", enc)
		}
		if len(arr) != 1 {
			t.Fatalf("expected 1 item, got %d", len(arr))
		}
		item, ok := arr[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", arr[0])
		}
		if _, has := item["status"]; has {
			t.Error("expected 'status' to be filtered out")
		}
		if _, has := item["enforcement"]; !has {
			t.Error("expected 'enforcement' to be present")
		}
	})

	t.Run("returns nil for all-computed map", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"id":   {Computed: true},
				"guid": {Computed: true},
			},
		}
		val := map[string]interface{}{
			"id":   "abc",
			"guid": "def",
		}
		result := filterComputedAttrs(val, schema, "")
		if result != nil {
			t.Errorf("expected nil for all-computed map, got %v", result)
		}
	})

	t.Run("returns nil for all-computed array items", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			NestedBlocks: map[string]*tfjson.SchemaBlockType{
				"items": {
					NestingMode: tfjson.SchemaNestingModeList,
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"id": {Computed: true},
						},
					},
				},
			},
		}
		val := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": "1"},
				map[string]interface{}{"id": "2"},
			},
		}
		result := filterComputedAttrs(val, schema, "")
		if result != nil {
			t.Errorf("expected nil when all array items are empty after filtering, got %v", result)
		}
	})

	t.Run("primitive value is returned as-is", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"name": {Optional: true},
			},
		}
		result := filterComputedAttrs("hello", schema, "name")
		if result != "hello" {
			t.Errorf("expected 'hello', got %v", result)
		}
	})

	t.Run("computed-only primitive is filtered", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"guid": {Computed: true},
			},
		}
		result := filterComputedAttrs("some-guid", schema, "guid")
		if result != nil {
			t.Errorf("expected nil for computed-only primitive, got %v", result)
		}
	})

	t.Run("filters cty attribute-as-block id field", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"subnet": {
					Optional: true,
					Computed: true,
					AttributeType: cty.Set(cty.Object(map[string]cty.Type{
						"name":             cty.String,
						"address_prefixes": cty.List(cty.String),
						"id":               cty.String,
					})),
				},
			},
		}
		val := map[string]interface{}{
			"subnet": []interface{}{
				map[string]interface{}{
					"name":             "subnet1",
					"address_prefixes": []interface{}{"10.0.1.0/24"},
					"id":               "/some/id",
				},
			},
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		subnet, ok := resultMap["subnet"]
		if !ok {
			t.Fatal("expected 'subnet' in result")
		}
		arr, ok := subnet.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", subnet)
		}
		item, ok := arr[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", arr[0])
		}
		if _, has := item["id"]; has {
			t.Error("expected 'id' to be filtered out from subnet")
		}
		if _, has := item["name"]; !has {
			t.Error("expected 'name' to be present")
		}
		if _, has := item["address_prefixes"]; !has {
			t.Error("expected 'address_prefixes' to be present")
		}
	})

	t.Run("filters nested attribute type computed fields", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"identity": {
					Optional: true,
					Computed: true,
					AttributeNestedType: &tfjson.SchemaNestedAttributeType{
						NestingMode: tfjson.SchemaNestingModeList,
						Attributes: map[string]*tfjson.SchemaAttribute{
							"type":         {Required: true},
							"principal_id": {Computed: true},
							"tenant_id":    {Computed: true},
						},
					},
				},
			},
		}
		val := map[string]interface{}{
			"identity": []interface{}{
				map[string]interface{}{
					"type":         "SystemAssigned",
					"principal_id": "abc-123",
					"tenant_id":    "def-456",
				},
			},
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		identity, ok := resultMap["identity"]
		if !ok {
			t.Fatal("expected 'identity' in result")
		}
		arr, ok := identity.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", identity)
		}
		item, ok := arr[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", arr[0])
		}
		if _, has := item["principal_id"]; has {
			t.Error("expected 'principal_id' to be filtered out")
		}
		if _, has := item["tenant_id"]; has {
			t.Error("expected 'tenant_id' to be filtered out")
		}
		if _, has := item["type"]; !has {
			t.Error("expected 'type' to be present")
		}
	})

	t.Run("empty attrPath at top level", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"name": {Optional: true},
			},
		}
		val := map[string]interface{}{
			"name": "test",
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if resultMap["name"] != "test" {
			t.Errorf("expected 'test', got %v", resultMap["name"])
		}
	})

	t.Run("mixed computed and non-computed at multiple levels", func(t *testing.T) {
		schema := &tfjson.SchemaBlock{
			Attributes: map[string]*tfjson.SchemaAttribute{
				"name":    {Optional: true},
				"guid":    {Computed: true},
				"etag":    {Computed: true},
				"enabled": {Optional: true},
			},
			NestedBlocks: map[string]*tfjson.SchemaBlockType{
				"config": {
					NestingMode: tfjson.SchemaNestingModeList,
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"key":        {Required: true},
							"value":      {Optional: true},
							"version_id": {Computed: true},
						},
					},
				},
			},
		}
		val := map[string]interface{}{
			"name":    "my-resource",
			"guid":    "computed-guid",
			"etag":    "computed-etag",
			"enabled": true,
			"config": []interface{}{
				map[string]interface{}{
					"key":        "setting1",
					"value":      "val1",
					"version_id": "v123",
				},
			},
		}
		result := filterComputedAttrs(val, schema, "")
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		// guid and etag should be filtered
		if _, has := resultMap["guid"]; has {
			t.Error("expected 'guid' to be filtered")
		}
		if _, has := resultMap["etag"]; has {
			t.Error("expected 'etag' to be filtered")
		}

		// name and enabled should remain
		if _, has := resultMap["name"]; !has {
			t.Error("expected 'name' to be present")
		}
		if _, has := resultMap["enabled"]; !has {
			t.Error("expected 'enabled' to be present")
		}

		// config should have version_id filtered
		config, ok := resultMap["config"]
		if !ok {
			t.Fatal("expected 'config' in result")
		}
		arr, ok := config.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", config)
		}
		item, ok := arr[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", arr[0])
		}
		if _, has := item["version_id"]; has {
			t.Error("expected 'version_id' to be filtered out in config")
		}
		if _, has := item["key"]; !has {
			t.Error("expected 'key' to be present in config")
		}
		if _, has := item["value"]; !has {
			t.Error("expected 'value' to be present in config")
		}
	})
}
