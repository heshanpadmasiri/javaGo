package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heshanpadmasiri/javaGo/gosrc"
	"github.com/heshanpadmasiri/javaGo/java"
)

var update = flag.Bool("update", false, "update expected Go files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func getGoFilePath(javaFile string) string {
	baseName := strings.TrimSuffix(filepath.Base(javaFile), ".java")
	return filepath.Join("testdata", "go", baseName+".go")
}

func formatGoCode(code string) (string, error) {
	cmd := exec.Command("gofmt", "-s")
	cmd.Stdin = strings.NewReader(code)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gofmt failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func updateExpectedFile(goFile string, content string) error {
	dir := filepath.Dir(goFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(goFile, []byte(content), 0o644)
}

func TestMigration(t *testing.T) {
	javaDir := filepath.Join("testdata", "java")
	entries, err := os.ReadDir(javaDir)
	if err != nil {
		t.Fatalf("Failed to read testdata/java directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".java") {
			continue
		}

		javaFile := filepath.Join(javaDir, entry.Name())
		testName := strings.TrimSuffix(entry.Name(), ".java")

		t.Run(testName, func(t *testing.T) {
			// Read Java content
			javaContent, err := os.ReadFile(javaFile)
			if err != nil {
				t.Fatalf("Failed to read Java file %s: %v", javaFile, err)
			}

			// Get corresponding Go file path
			goFile := getGoFilePath(javaFile)

			// Run migration
			tree := java.ParseJava(javaContent)
			defer tree.Close()

			ctx := &java.MigrationContext{
				JavaSource:      javaContent,
				AbstractClasses: make(map[string]bool),
				EnumConstants:   make(map[string]string),
				Constructors:    make(map[gosrc.Type][]java.FunctionData),
			}
			java.MigrateTree(ctx, tree)
			config := gosrc.Config{
				PackageName:   "converted",
				LicenseHeader: "",
			}
			result := ctx.Source.ToSource(config)

			// Format output with go fmt
			formatted, err := formatGoCode(result)
			if err != nil {
				t.Fatalf("Failed to format Go code: %v", err)
			}

			// Read expected Go file
			expected, err := os.ReadFile(goFile)
			if err != nil {
				if *update {
					// Create expected file if it doesn't exist
					if err := updateExpectedFile(goFile, formatted); err != nil {
						t.Fatalf("Failed to update expected file: %v", err)
					}
					t.Logf("Created expected file: %s", goFile)
					return
				}
				t.Fatalf("Failed to read expected Go file %s: %v", goFile, err)
			}

			expectedStr := string(expected)

			// Compare formatted output with expected (exact match)
			if formatted != expectedStr {
				if *update {
					// Update expected file
					if err := updateExpectedFile(goFile, formatted); err != nil {
						t.Fatalf("Failed to update expected file: %v", err)
					}
					t.Logf("Updated expected file: %s", goFile)
					return
				}
				t.Errorf("Output does not match expected:\n--- Got ---\n%s\n--- Expected ---\n%s", formatted, expectedStr)
			}
		})
	}
}

func TestMigrationWithConfig(t *testing.T) {
	tests := []struct {
		name                  string
		configContent         string
		createConfig          bool
		expectedPkg           string
		expectedLicense       string
		expectLicenseInOutput bool
	}{
		{
			name: "both_package_and_license",
			configContent: `package_name = "mypackage"

license_header = """// Copyright 2024 Test Company
// Licensed under Apache 2.0
"""
`,
			createConfig:          true,
			expectedPkg:           "mypackage",
			expectedLicense:       "// Copyright 2024 Test Company\n// Licensed under Apache 2.0\n",
			expectLicenseInOutput: true,
		},
		{
			name: "only_package_name",
			configContent: `package_name = "custompkg"
`,
			createConfig:          true,
			expectedPkg:           "custompkg",
			expectedLicense:       "",
			expectLicenseInOutput: false,
		},
		{
			name: "only_license_header",
			configContent: `license_header = """// MIT License
"""
`,
			createConfig:          true,
			expectedPkg:           "converted",
			expectedLicense:       "// MIT License\n",
			expectLicenseInOutput: true,
		},
		{
			name:                  "no_config_file",
			configContent:         "",
			createConfig:          false,
			expectedPkg:           gosrc.PackageName,
			expectedLicense:       "",
			expectLicenseInOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the test
			tmpDir, err := os.MkdirTemp("", "javago-config-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Save original working directory
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current working directory: %v", err)
			}
			defer os.Chdir(originalWd)

			// Change to temporary directory
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			// Create Config.toml if needed
			if tt.createConfig {
				configPath := filepath.Join(tmpDir, "Config.toml")
				if err := os.WriteFile(configPath, []byte(tt.configContent), 0o644); err != nil {
					t.Fatalf("Failed to write Config.toml: %v", err)
				}
			}

			// Use a simple Java file for testing
			javaContent := []byte("public record Point(int x, int y) {}")

			// Run migration
			tree := java.ParseJava(javaContent)
			defer tree.Close()

			ctx := &java.MigrationContext{
				JavaSource:      javaContent,
				AbstractClasses: make(map[string]bool),
				EnumConstants:   make(map[string]string),
				Constructors:    make(map[gosrc.Type][]java.FunctionData),
			}
			java.MigrateTree(ctx, tree)

			// Load config (should read from Config.toml in current directory)
			config := java.LoadConfig()

			// Verify config was loaded correctly
			if config.PackageName != tt.expectedPkg {
				t.Errorf("Expected package name '%s', got '%s'", tt.expectedPkg, config.PackageName)
			}
			if config.LicenseHeader != tt.expectedLicense {
				t.Errorf("Expected license header:\n%q\nGot:\n%q", tt.expectedLicense, config.LicenseHeader)
			}

			// Generate Go source with config
			result := ctx.Source.ToSource(config)

			// Verify the output contains the expected package name
			expectedPkgLine := "package " + tt.expectedPkg
			if !strings.Contains(result, expectedPkgLine) {
				t.Errorf("Output should contain '%s', got:\n%s", expectedPkgLine, result)
			}

			// Verify the output contains the license header if expected
			if tt.expectLicenseInOutput {
				if !strings.HasPrefix(result, tt.expectedLicense) {
					t.Errorf("Output should start with license header, got:\n%s", result)
				}
			} else {
				if strings.Contains(result, tt.expectedLicense) && tt.expectedLicense != "" {
					t.Errorf("Output should not contain license header, got:\n%s", result)
				}
			}

			// Verify the output contains the expected struct
			if !strings.Contains(result, "type Point struct") {
				t.Errorf("Output should contain Point struct, got:\n%s", result)
			}
		})
	}
}
