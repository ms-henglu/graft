package vendors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ms-henglu/graft/internal/log"
)

const defaultRegistryHost = "registry.terraform.io"

// RegistryDiscoveryResponse represents the response from .well-known/terraform.json
type RegistryDiscoveryResponse struct {
	ModulesV1 string `json:"modules.v1"`
}

// resolveRegistrySource takes a module source string (e.g. "terraform-aws-modules/vpc/aws")
// and returns the actual download URL (e.g. "git::https://github.com/...")
func resolveRegistrySource(source, version string) (string, error) {
	hostname, namespace, name, provider, err := parseModuleSource(source)
	if err != nil {
		return "", err
	}

	// 1. Service Discovery
	modulesPath, err := discoverModulesPath(hostname)
	if err != nil {
		return "", fmt.Errorf("service discovery failed for %s: %w", hostname, err)
	}

	// 2. Get Download URL
	// Construct the URL: https://{hostname}{modulesPath}{namespace}/{name}/{provider}/{version}/download
	// modulesPath usually starts with /, but let's be safe
	path := fmt.Sprintf("%s%s/%s/%s/%s/download", modulesPath, namespace, name, provider, version)
	// Clean double slashes if any
	path = strings.ReplaceAll(path, "//", "/")

	downloadURL := fmt.Sprintf("https://%s%s", hostname, path)

	log.Debug("Querying Registry for download URL: %s", downloadURL)

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("registry request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	// 3. Extract X-Terraform-Get header
	xTerraformGet := resp.Header.Get("X-Terraform-Get")
	if xTerraformGet == "" {
		return "", fmt.Errorf("registry response missing X-Terraform-Get header")
	}

	return xTerraformGet, nil
}

// parseModuleSource parses "namespace/name/provider" or "hostname/namespace/name/provider"
func parseModuleSource(source string) (hostname, namespace, name, provider string, err error) {
	parts := strings.Split(source, "/")
	if len(parts) == 3 {
		// Public Registry: namespace/name/provider
		return defaultRegistryHost, parts[0], parts[1], parts[2], nil
	} else if len(parts) == 4 {
		// Private Registry: hostname/namespace/name/provider
		return parts[0], parts[1], parts[2], parts[3], nil
	}
	return "", "", "", "", fmt.Errorf("invalid module source format: %s (expected 3 or 4 parts)", source)
}

// discoverModulesPath queries .well-known/terraform.json
func discoverModulesPath(hostname string) (string, error) {
	url := fmt.Sprintf("https://%s/.well-known/terraform.json", hostname)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		// Fallback for public registry if discovery fails?
		if hostname == defaultRegistryHost {
			return "/v1/modules/", nil
		}
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if hostname == defaultRegistryHost {
			return "/v1/modules/", nil
		}
		return "", fmt.Errorf("discovery returned status %d", resp.StatusCode)
	}

	var discovery RegistryDiscoveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return "", fmt.Errorf("failed to parse discovery response: %w", err)
	}

	if discovery.ModulesV1 == "" {
		return "", fmt.Errorf("registry does not support modules.v1")
	}

	return discovery.ModulesV1, nil
}

// isRegistrySource checks if the string looks like a registry source
func isRegistrySource(source string) bool {
	// Simple heuristic:
	// - No "git::", "http::", "https::", "s3::", etc.
	// - No starting with "./" or "/"
	// - Has 3 or 4 parts separated by "/"

	if strings.Contains(source, "::") {
		return false
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") {
		return false
	}

	parts := strings.Split(source, "/")
	return len(parts) == 3 || len(parts) == 4
}
