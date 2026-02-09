package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/cmd"
)

// testConfig defines the test configuration parsed from expected.hcl
type testConfig struct {
	Command       string   // "build" (default), "scaffold", or "absorb"
	CommandArgs   []string // Arguments for the command (e.g., module keys for scaffold, plan file for absorb)
	ExpectedFiles []expectedFile
}

// expectedFile defines an expected output file
type expectedFile struct {
	Path        string
	Content     string   // Expected content (whitespace-normalized comparison)
	Contains    []string // Strings that should appear (for non-exact matching)
	NotContains []string // Strings that should NOT appear
}

// TestAcceptance runs end-to-end acceptance tests using the testdata folder.
// Each test case directory should contain:
// - main.tf: The Terraform configuration
// - manifest.graft.hcl (or *.graft.hcl): The Graft manifest (for build tests)
// - expected.hcl: Expected behavior specification
//
// The expected.hcl file can specify:
// - command = "build" (default) or "scaffold"
// - command_args = ["arg1", "arg2"] (optional arguments for the command)
// - expected "path/to/file" { ... } blocks for file verification
//
// The test will:
// 1. Run terraform init to initialize the test case
// 2. Run the specified graft command (build or scaffold)
// 3. Verify expected output files are generated
func TestAcceptance(t *testing.T) {
	// Check if terraform is available
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform not found in PATH, skipping acceptance tests")
	}

	testdataDir := "testdata"

	// Get list of test case directories
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("Failed to read testdata directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		testName := entry.Name()
		testPath := filepath.Join(testdataDir, testName)

		t.Run(testName, func(t *testing.T) {
			runAcceptanceTest(t, testPath)
		})
	}
}

// runAcceptanceTest runs the graft build process for a single test case
func runAcceptanceTest(t *testing.T, testPath string) {
	// Store original working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Change to test case directory
	if err := os.Chdir(testPath); err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Load test configuration from expected.hcl
	config := loadExpectedHCL(t)

	// Run terraform init (not needed for absorb which reads plan JSON directly)
	if config.Command != "absorb" {
		runTerraformInit(t)
	}

	// Run the appropriate graft command
	switch config.Command {
	case "scaffold":
		runGraftScaffold(t, config.CommandArgs)
	case "absorb":
		runGraftAbsorb(t, config.CommandArgs)
	case "build", "":
		runGraftBuild(t)
	default:
		t.Fatalf("Unknown command: %s", config.Command)
	}

	// Verify expected files
	verifyExpectedFiles(t, config.ExpectedFiles)
}

// loadExpectedHCL loads the expected.hcl file for a test case
func loadExpectedHCL(t *testing.T) testConfig {
	t.Helper()

	content, err := os.ReadFile("expected.hcl")
	if err != nil {
		t.Fatalf("Failed to read expected.hcl: %v", err)
	}

	f, diags := hclwrite.ParseConfig(content, "expected.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("Failed to parse expected.hcl: %s", diags.Error())
	}

	config := testConfig{
		Command: "build", // default command
	}

	// Parse command attribute at top level
	if attr := f.Body().GetAttribute("command"); attr != nil {
		config.Command = strings.Trim(strings.TrimSpace(string(attr.Expr().BuildTokens(nil).Bytes())), "\"")
	}

	// Parse command_args attribute at top level
	if attr := f.Body().GetAttribute("command_args"); attr != nil {
		config.CommandArgs = parseStringList(attr)
	}

	var files []expectedFile
	for _, block := range f.Body().Blocks() {
		if block.Type() != "expected" {
			continue
		}

		if len(block.Labels()) == 0 {
			t.Fatalf("expected block must have a path label")
		}

		ef := expectedFile{
			Path: block.Labels()[0],
		}

		// Parse content block
		for _, contentBlock := range block.Body().Blocks() {
			if contentBlock.Type() == "content" {
				// Get raw content between braces, preserving the HCL inside
				tokens := contentBlock.Body().BuildTokens(nil)
				ef.Content = strings.TrimSpace(string(tokens.Bytes()))
			}
		}

		// Parse contains attribute
		if attr := block.Body().GetAttribute("contains"); attr != nil {
			ef.Contains = parseStringList(attr)
		}

		// Parse not_contains attribute
		if attr := block.Body().GetAttribute("not_contains"); attr != nil {
			ef.NotContains = parseStringList(attr)
		}

		files = append(files, ef)
	}

	config.ExpectedFiles = files
	return config
}

// parseStringList parses an HCL list attribute into a string slice
func parseStringList(attr *hclwrite.Attribute) []string {
	expr := string(attr.Expr().BuildTokens(nil).Bytes())
	// Remove brackets and whitespace
	expr = strings.TrimSpace(expr)
	expr = strings.TrimPrefix(expr, "[")
	expr = strings.TrimSuffix(expr, "]")
	if expr == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(expr, ",") {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, "\"")
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// runTerraformInit runs terraform init in the current directory
func runTerraformInit(t *testing.T) {
	t.Helper()

	cmd := exec.Command("terraform", "init", "-backend=false", "-input=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("terraform init failed: %v\nOutput: %s", err, string(output))
	}
}

// runGraftBuild runs the graft build command using cmd.NewBuildCmd()
func runGraftBuild(t *testing.T) {
	t.Helper()

	buildCmd := cmd.NewBuildCmd()
	if err := buildCmd.Execute(); err != nil {
		t.Fatalf("graft build failed: %v", err)
	}
}

// runGraftScaffold runs the graft scaffold command using cmd.NewScaffoldCmd()
func runGraftScaffold(t *testing.T, args []string) {
	t.Helper()

	scaffoldCmd := cmd.NewScaffoldCmd()
	scaffoldCmd.SetArgs(args)
	if err := scaffoldCmd.Execute(); err != nil {
		t.Fatalf("graft scaffold failed: %v", err)
	}
}

// runGraftAbsorb runs the graft absorb command using cmd.NewAbsorbCmd()
func runGraftAbsorb(t *testing.T, args []string) {
	t.Helper()

	absorbCmd := cmd.NewAbsorbCmd()
	absorbCmd.SetArgs(args)
	if err := absorbCmd.Execute(); err != nil {
		t.Fatalf("graft absorb failed: %v", err)
	}
}

// verifyExpectedFiles verifies that expected files exist and contain expected content
func verifyExpectedFiles(t *testing.T, expectedFiles []expectedFile) {
	t.Helper()

	for _, ef := range expectedFiles {
		content, err := os.ReadFile(ef.Path)
		if err != nil {
			t.Errorf("Failed to read expected file %s: %v", ef.Path, err)
			continue
		}

		contentStr := string(content)

		// Check for exact content match (whitespace-normalized)
		if ef.Content != "" {
			if !equalIgnoringWhitespace(contentStr, ef.Content) {
				t.Errorf("File %s content mismatch.\nExpected (normalized):\n%s\n\nActual (normalized):\n%s",
					ef.Path, normalizeWhitespace(ef.Content), normalizeWhitespace(contentStr))
			}
		}

		// Check for substring matches
		for _, contains := range ef.Contains {
			if !strings.Contains(contentStr, contains) {
				t.Errorf("File %s does not contain expected string: %q", ef.Path, contains)
			}
		}

		// Check for content that should not exist
		for _, notContains := range ef.NotContains {
			if strings.Contains(contentStr, notContains) {
				t.Errorf("File %s contains unexpected string: %q", ef.Path, notContains)
			}
		}
	}
}

// normalizeWhitespace removes extra whitespace for comparison
func normalizeWhitespace(s string) string {
	// Replace all whitespace sequences with a single space
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

// equalIgnoringWhitespace compares two strings ignoring whitespace differences
func equalIgnoringWhitespace(a, b string) bool {
	return normalizeWhitespace(a) == normalizeWhitespace(b)
}
