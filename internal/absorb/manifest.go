package absorb

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
)

// GenerateManifest creates an absorb.graft.hcl from the absorb result.
// If schemasPath is provided, it loads the provider schemas to correctly render
// nested blocks vs attributes and to filter out computed-only attributes.
func GenerateManifest(changes []DriftChange, schemasPath string) ([]byte, error) {
	root := NewModuleItem("")
	for _, change := range changes {
		root.AddChange(change)
	}

	// Load provider schemas if provided
	var providerSchemas *tfjson.ProviderSchemas
	if schemasPath != "" {
		schemaData, err := os.ReadFile(schemasPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read providers schema file: %w", err)
		}
		providerSchemas = &tfjson.ProviderSchemas{}
		if err := json.Unmarshal(schemaData, providerSchemas); err != nil {
			return nil, fmt.Errorf("failed to parse providers schema JSON: %w", err)
		}
	}

	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()
	rootBody.AppendUnstructuredTokens(root.ToHCL(providerSchemas))

	return hclwrite.Format(f.Bytes()), nil
}
