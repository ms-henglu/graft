package absorb

import tfjson "github.com/hashicorp/terraform-json"

// lookupResourceSchema finds the schema block for a resource type from provider schemas.
func lookupResourceSchema(schemas *tfjson.ProviderSchemas, providerName, resourceType string) *tfjson.SchemaBlock {
	if schemas == nil {
		return nil
	}

	ps, ok := schemas.Schemas[providerName]
	if !ok || ps == nil {
		return nil
	}

	rs, ok := ps.ResourceSchemas[resourceType]
	if !ok || rs == nil {
		return nil
	}
	return rs.Block
}
