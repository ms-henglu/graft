package absorb

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

func TestLookupResourceSchema(t *testing.T) {
	block := &tfjson.SchemaBlock{
		Attributes: map[string]*tfjson.SchemaAttribute{
			"name": {Optional: true},
		},
	}

	schemas := &tfjson.ProviderSchemas{
		Schemas: map[string]*tfjson.ProviderSchema{
			"registry.terraform.io/hashicorp/azurerm": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"azurerm_resource_group": {
						Block: block,
					},
				},
			},
		},
	}

	tests := []struct {
		name         string
		schemas      *tfjson.ProviderSchemas
		providerName string
		resourceType string
		expectNil    bool
	}{
		{
			name:         "nil schemas returns nil",
			schemas:      nil,
			providerName: "any",
			resourceType: "any",
			expectNil:    true,
		},
		{
			name:         "provider not found returns nil",
			schemas:      schemas,
			providerName: "registry.terraform.io/hashicorp/aws",
			resourceType: "aws_instance",
			expectNil:    true,
		},
		{
			name:         "resource type not found returns nil",
			schemas:      schemas,
			providerName: "registry.terraform.io/hashicorp/azurerm",
			resourceType: "azurerm_virtual_network",
			expectNil:    true,
		},
		{
			name:         "valid provider and resource returns block",
			schemas:      schemas,
			providerName: "registry.terraform.io/hashicorp/azurerm",
			resourceType: "azurerm_resource_group",
			expectNil:    false,
		},
		{
			name: "nil provider schema value returns nil",
			schemas: &tfjson.ProviderSchemas{
				Schemas: map[string]*tfjson.ProviderSchema{
					"registry.terraform.io/hashicorp/azurerm": nil,
				},
			},
			providerName: "registry.terraform.io/hashicorp/azurerm",
			resourceType: "azurerm_resource_group",
			expectNil:    true,
		},
		{
			name: "nil resource schema value returns nil",
			schemas: &tfjson.ProviderSchemas{
				Schemas: map[string]*tfjson.ProviderSchema{
					"registry.terraform.io/hashicorp/azurerm": {
						ResourceSchemas: map[string]*tfjson.Schema{
							"azurerm_resource_group": nil,
						},
					},
				},
			},
			providerName: "registry.terraform.io/hashicorp/azurerm",
			resourceType: "azurerm_resource_group",
			expectNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lookupResourceSchema(tt.schemas, tt.providerName, tt.resourceType)
			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Error("expected non-nil schema block")
				}
				if result != block {
					t.Error("expected the correct schema block to be returned")
				}
			}
		})
	}
}
