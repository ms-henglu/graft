package patch

import (
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TestDeepMergeBlocksForOverride(t *testing.T) {
	testcases := []struct {
		name        string
		sourceHCL   string
		overrideHCL string
		expected    string
	}{
		{
			name: "single static block with attribute override",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name          = "my-vnet"
  address_space = ["10.0.0.0/16"]

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name                            = "subnet1"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
}`,
		},
		{
			name: "multiple static blocks of same type",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  subnet {
    name             = "subnet2"
    address_prefixes = ["10.0.2.0/24"]
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name                            = "subnet1"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
  subnet {
    name                            = "subnet2"
    address_prefixes                = ["10.0.2.0/24"]
    default_outbound_access_enabled = false
  }
}`,
		},
		{
			name: "dynamic block with attribute override",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"

  dynamic "subnet" {
    for_each = var.subnets
    content {
      name             = subnet.value.name
      address_prefixes = subnet.value.address_prefixes
    }
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  dynamic "subnet" {
    for_each = var.subnets
    content {
      name                            = subnet.value.name
      address_prefixes                = subnet.value.address_prefixes
      default_outbound_access_enabled = false
    }
  }
}`,
		},
		{
			name: "mixed static and dynamic blocks",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"

  subnet {
    name             = "static-subnet"
    address_prefixes = ["10.0.1.0/24"]
  }

  dynamic "subnet" {
    for_each = var.extra_subnets
    content {
      name             = subnet.value.name
      address_prefixes = subnet.value.address_prefixes
    }
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name                            = "static-subnet"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
  dynamic "subnet" {
    for_each = var.extra_subnets
    content {
      name                            = subnet.value.name
      address_prefixes                = subnet.value.address_prefixes
      default_outbound_access_enabled = false
    }
  }
}`,
		},
		{
			name: "multi-level nested blocks",
			sourceHCL: `
resource "azurerm_kubernetes_cluster" "main" {
  name = "my-aks"

  default_node_pool {
    name       = "default"
    node_count = 1

    linux_os_config {
      sysctl_config {
        net_core_somaxconn = 65535
      }
    }
  }
}
`,
			overrideHCL: `
resource "azurerm_kubernetes_cluster" "main" {
  default_node_pool {
    linux_os_config {
      sysctl_config {
        vm_max_map_count = 262144
      }
    }
  }
}
`,
			// Note: attribute order may vary due to map iteration, so we check for key attributes
			expected: `resource "azurerm_kubernetes_cluster" "main" {
  default_node_pool {
    node_count = 1
    name       = "default"
    linux_os_config {
      sysctl_config {
        net_core_somaxconn = 65535
        vm_max_map_count   = 262144
      }
    }
  }
}`,
		},
		{
			name: "adding new nested block that doesn't exist in source",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name          = "my-vnet"
  address_space = ["10.0.0.0/16"]
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    name             = "new-subnet"
    address_prefixes = ["10.0.1.0/24"]
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name             = "new-subnet"
    address_prefixes = ["10.0.1.0/24"]
  }
}`,
		},
		{
			name: "preserve sibling blocks not in override",
			sourceHCL: `
resource "azurerm_storage_account" "main" {
  name = "mystorageaccount"

  network_rules {
    default_action = "Deny"
    ip_rules       = ["10.0.0.1"]
  }

  blob_properties {
    cors_rule {
      allowed_headers = ["*"]
    }
  }
}
`,
			overrideHCL: `
resource "azurerm_storage_account" "main" {
  network_rules {
    bypass = ["AzureServices"]
  }
}
`,
			expected: `resource "azurerm_storage_account" "main" {
  network_rules {
    default_action = "Deny"
    ip_rules       = ["10.0.0.1"]
    bypass         = ["AzureServices"]
  }
}`,
		},
		{
			name: "top-level attributes remain in override (for shallow merge)",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name          = "my-vnet"
  address_space = ["10.0.0.0/16"]

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  address_space = ["10.1.0.0/16"]
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  address_space = ["10.1.0.0/16"]
  subnet {
    name                            = "subnet1"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
}`,
		},
		{
			name: "dynamic block with iterator",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"

  dynamic "subnet" {
    for_each = var.subnets
    iterator = s
    content {
      name             = s.value.name
      address_prefixes = s.value.prefixes
    }
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    security_group = azurerm_network_security_group.main.id
  }
}
`,
			expected: `resource "azurerm_virtual_network" "main" {
  dynamic "subnet" {
    for_each = var.subnets
    iterator = s
    content {
      name             = s.value.name
      address_prefixes = s.value.prefixes
      security_group   = azurerm_network_security_group.main.id
    }
  }
}`,
		},
		{
			name: "lifecycle block should NOT be deep merged",
			sourceHCL: `
resource "azurerm_resource_group" "main" {
  name     = "my-rg"
  location = "eastus"

  lifecycle {
    prevent_destroy = false
    ignore_changes  = [tags]
  }
}
`,
			overrideHCL: `
resource "azurerm_resource_group" "main" {
  lifecycle {
    prevent_destroy = true
  }
}
`,
			// lifecycle should be passed through as-is, NOT merged with source
			expected: `resource "azurerm_resource_group" "main" {
  lifecycle {
    prevent_destroy = true
  }
}`,
		},
		{
			name: "provisioner block should NOT be deep merged",
			sourceHCL: `
resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo hello"
    when    = create
  }
}
`,
			overrideHCL: `
resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo goodbye"
  }
}
`,
			// provisioner should be passed through as-is (Terraform appends them)
			expected: `resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo goodbye"
  }
}`,
		},
		{
			name: "mixed meta-argument and regular nested blocks",
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  lifecycle {
    prevent_destroy = false
  }
}
`,
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
  lifecycle {
    prevent_destroy = true
  }
}
`,
			// subnet should be deep merged, lifecycle should NOT
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name                            = "subnet1"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
  lifecycle {
    prevent_destroy = true
  }
}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse source block
			sourceFile, diags := hclwrite.ParseConfig([]byte(tc.sourceHCL), "source.tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("Failed to parse source HCL: %s", diags.Error())
			}
			sourceBlocks := sourceFile.Body().Blocks()
			if len(sourceBlocks) != 1 {
				t.Fatalf("Expected 1 source block, got %d", len(sourceBlocks))
			}
			sourceBlock := sourceBlocks[0]

			// Parse override block
			overrideFile, diags := hclwrite.ParseConfig([]byte(tc.overrideHCL), "override.tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("Failed to parse override HCL: %s", diags.Error())
			}
			overrideBlocks := overrideFile.Body().Blocks()
			if len(overrideBlocks) != 1 {
				t.Fatalf("Expected 1 override block, got %d", len(overrideBlocks))
			}
			overrideBlock := overrideBlocks[0]

			// Perform deep merge
			result := deepMergeNestedBlock(sourceBlock, overrideBlock, true)

			// Generate output
			outFile := hclwrite.NewEmptyFile()
			outFile.Body().AppendBlock(result)
			got := strings.TrimSpace(string(outFile.Bytes()))
			expected := strings.TrimSpace(tc.expected)

			// Normalize whitespace for comparison - compare sorted lines
			gotNorm := normalizeHCLSorted(got)
			expectedNorm := normalizeHCLSorted(expected)

			if gotNorm != expectedNorm {
				t.Errorf("Deep merge mismatch.\n\nGot:\n%s\n\nExpected:\n%s", got, expected)
			}
		})
	}
}

func TestGenerateOverrideFileWithDeepMerge(t *testing.T) {
	testcases := []struct {
		name           string
		overrideHCL    string
		sourceHCL      string
		existingLocals map[string]*hclwrite.Attribute
		expected       string
	}{
		{
			name: "override with nested block deep merge",
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  subnet {
    default_outbound_access_enabled = false
  }
}
`,
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name = "my-vnet"
  
  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }
}
`,
			existingLocals: map[string]*hclwrite.Attribute{},
			expected: `resource "azurerm_virtual_network" "main" {
  subnet {
    name                            = "subnet1"
    address_prefixes                = ["10.0.1.0/24"]
    default_outbound_access_enabled = false
  }
}`,
		},
		{
			name: "override without nested blocks uses shallow merge",
			overrideHCL: `
resource "azurerm_virtual_network" "main" {
  address_space = ["10.1.0.0/16"]
}
`,
			sourceHCL: `
resource "azurerm_virtual_network" "main" {
  name          = "my-vnet"
  address_space = ["10.0.0.0/16"]
}
`,
			existingLocals: map[string]*hclwrite.Attribute{},
			expected: `resource "azurerm_virtual_network" "main" {
  address_space = ["10.1.0.0/16"]
}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse override blocks
			overrideFile, diags := hclwrite.ParseConfig([]byte(tc.overrideHCL), "override.tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("Failed to parse override HCL: %s", diags.Error())
			}
			overrideBlocks := overrideFile.Body().Blocks()

			// Parse source blocks and build existingBlocks map
			sourceFile, diags := hclwrite.ParseConfig([]byte(tc.sourceHCL), "source.tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("Failed to parse source HCL: %s", diags.Error())
			}
			existingBlocks := make(map[string]*hclwrite.Block)
			for _, block := range sourceFile.Body().Blocks() {
				existingBlocks[blockKey(block)] = block
			}

			// Generate override file
			gotFile := generateOverrideFile(overrideBlocks, existingBlocks, tc.existingLocals)
			got := strings.TrimSpace(string(gotFile.Bytes()))
			expected := strings.TrimSpace(tc.expected)

			// Normalize whitespace for comparison - compare sorted lines
			gotNorm := normalizeHCLSorted(got)
			expectedNorm := normalizeHCLSorted(expected)

			if gotNorm != expectedNorm {
				t.Errorf("generateOverrideFile mismatch.\n\nGot:\n%s\n\nExpected:\n%s", got, expected)
			}
		})
	}
}

// normalizeHCLSorted normalizes HCL and sorts attribute lines within blocks
// This handles attribute ordering differences from map iteration
func normalizeHCLSorted(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	// Sort all lines - this is a simple approach that works for our test cases
	// since the structure is preserved but attribute order may vary
	sort.Strings(result)
	return strings.Join(result, "\n")
}
