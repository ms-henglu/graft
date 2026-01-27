package vendors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTrueSourcePath(t *testing.T) {
	// Setup temporary cache dir
	tmpCacheDir, err := os.MkdirTemp("", "graft-cache")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpCacheDir) }()

	// Set env var to force GlobalCacheDir to use tmpCacheDir
	if err := os.Setenv("GRAFT_CACHE_DIR", tmpCacheDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("GRAFT_CACHE_DIR") }()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Helper to calculate expected cache path
	getExpectedCachePath := func(source, version string) string {
		key := GetCacheKey(source, version)
		return filepath.Join(tmpCacheDir, key)
	}

	tests := []struct {
		name      string
		targetKey string
		moduleMap map[string]Module
		expected  string
		wantErr   bool
	}{
		{
			name:      "Root Module",
			targetKey: "",
			moduleMap: map[string]Module{},
			expected:  cwd,
			wantErr:   false,
		},
		{
			name:      "Remote Anchor",
			targetKey: "vpc",
			moduleMap: map[string]Module{
				"vpc": {
					Key:     "vpc",
					Source:  "terraform-aws-modules/vpc/aws",
					Version: "3.0.0",
					Dir:     "IGNORE_ME",
				},
			},
			expected: getExpectedCachePath("terraform-aws-modules/vpc/aws", "3.0.0"),
			wantErr:  false,
		},
		{
			name:      "Local Module with Remote Parent",
			targetKey: "vpc.subnet",
			moduleMap: map[string]Module{
				"vpc": {
					Key:     "vpc",
					Source:  "terraform-aws-modules/vpc/aws",
					Version: "3.0.0",
				},
				"vpc.subnet": {
					Key:    "vpc.subnet",
					Source: "./modules/subnet",
				},
			},
			expected: filepath.Join(getExpectedCachePath("terraform-aws-modules/vpc/aws", "3.0.0"), "modules/subnet"),
			wantErr:  false,
		},
		{
			name:      "Nested Local Module",
			targetKey: "vpc.subnet.db",
			moduleMap: map[string]Module{
				"vpc": {
					Key:     "vpc",
					Source:  "terraform-aws-modules/vpc/aws",
					Version: "3.0.0",
				},
				"vpc.subnet": {
					Key:    "vpc.subnet",
					Source: "./modules/subnet",
				},
				"vpc.subnet.db": {
					Key:    "vpc.subnet.db",
					Source: "../db",
				},
			},
			expected: filepath.Join(getExpectedCachePath("terraform-aws-modules/vpc/aws", "3.0.0"), "modules/db"),
			wantErr:  false,
		},
		{
			name:      "Local Module at Root",
			targetKey: "my_local",
			moduleMap: map[string]Module{
				"my_local": {
					Key:    "my_local",
					Source: "./local_modules/my_mod",
				},
			},
			expected: filepath.Join(cwd, "local_modules/my_mod"),
			wantErr:  false,
		},
		{
			name:      "Missing Module",
			targetKey: "missing",
			moduleMap: map[string]Module{},
			expected:  "",
			wantErr:   true,
		},
		{
			name:      "Missing Parent",
			targetKey: "parent.child",
			moduleMap: map[string]Module{
				"parent.child": {
					Key:    "parent.child",
					Source: "./child",
				},
			},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTrueSourcePath(tt.targetKey, tt.moduleMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveTrueSourcePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ResolveTrueSourcePath() = %v, want %v", got, tt.expected)
			}
		})
	}
}
