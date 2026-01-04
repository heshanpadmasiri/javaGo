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
				Methods:         make(map[string][]java.FunctionData),
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
				Methods:         make(map[string][]java.FunctionData),
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

func TestMethodTracking(t *testing.T) {
	javaSource := []byte(`
public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
    
    public int add(int a, int b, int c) {
        return a + b + c;
    }
    
    public String concat(String s1, String s2) {
        return s1 + s2;
    }
    
    private void helper() {
    }
    
    public static int multiply(int x, int y) {
        return x * y;
    }
}
`)

	// Parse and analyze
	tree := java.ParseJava(javaSource)
	defer tree.Close()

	ctx := &java.MigrationContext{
		JavaSource:      javaSource,
		AbstractClasses: make(map[string]bool),
		EnumConstants:   make(map[string]string),
		Constructors:    make(map[gosrc.Type][]java.FunctionData),
		Methods:         make(map[string][]java.FunctionData),
	}

	java.MigrateTree(ctx, tree)

	// Test 1: Check 'add' method has 2 overloads
	addMethods, hasAdd := ctx.Methods["add"]
	if !hasAdd {
		t.Error("Expected 'add' method to be tracked")
	}
	if len(addMethods) != 2 {
		t.Errorf("Expected 2 'add' method overloads, got %d", len(addMethods))
	}

	// Test 2: Check 'add' overloads have correct signatures
	found2Param := false
	found3Param := false
	for _, method := range addMethods {
		if len(method.ArgumentTypes) == 2 {
			if method.ArgumentTypes[0] != "int" || method.ArgumentTypes[1] != "int" {
				t.Errorf("Expected (int, int) for 2-param add, got (%v, %v)",
					method.ArgumentTypes[0], method.ArgumentTypes[1])
			}
			found2Param = true
		} else if len(method.ArgumentTypes) == 3 {
			if method.ArgumentTypes[0] != "int" ||
				method.ArgumentTypes[1] != "int" ||
				method.ArgumentTypes[2] != "int" {
				t.Errorf("Expected (int, int, int) for 3-param add, got %v", method.ArgumentTypes)
			}
			found3Param = true
		}
	}

	if !found2Param {
		t.Error("Did not find 2-parameter add method")
	}
	if !found3Param {
		t.Error("Did not find 3-parameter add method")
	}

	// Test 3: Check 'concat' method
	concatMethods, hasConcat := ctx.Methods["concat"]
	if !hasConcat {
		t.Error("Expected 'concat' method to be tracked")
	}
	if len(concatMethods) != 1 {
		t.Errorf("Expected 1 'concat' method, got %d", len(concatMethods))
	}
	if len(concatMethods) > 0 {
		if len(concatMethods[0].ArgumentTypes) != 2 {
			t.Errorf("Expected 2 parameters for concat, got %d", len(concatMethods[0].ArgumentTypes))
		}
		if concatMethods[0].ArgumentTypes[0] != "string" || concatMethods[0].ArgumentTypes[1] != "string" {
			t.Errorf("Expected (string, string) for concat, got (%v, %v)",
				concatMethods[0].ArgumentTypes[0], concatMethods[0].ArgumentTypes[1])
		}
	}

	// Test 4: Check 'helper' method (no parameters)
	helperMethods, hasHelper := ctx.Methods["helper"]
	if !hasHelper {
		t.Error("Expected 'helper' method to be tracked")
	}
	if len(helperMethods) != 1 {
		t.Errorf("Expected 1 'helper' method, got %d", len(helperMethods))
	}
	if len(helperMethods) > 0 && len(helperMethods[0].ArgumentTypes) != 0 {
		t.Errorf("Expected 0 parameters for helper, got %d", len(helperMethods[0].ArgumentTypes))
	}

	// Test 5: Check 'multiply' method (static method should also be tracked)
	multiplyMethods, hasMultiply := ctx.Methods["multiply"]
	if !hasMultiply {
		t.Error("Expected 'multiply' method to be tracked")
	}
	if len(multiplyMethods) != 1 {
		t.Errorf("Expected 1 'multiply' method, got %d", len(multiplyMethods))
	}
	if len(multiplyMethods) > 0 {
		if len(multiplyMethods[0].ArgumentTypes) != 2 {
			t.Errorf("Expected 2 parameters for multiply, got %d", len(multiplyMethods[0].ArgumentTypes))
		}
	}
}

func TestMethodTrackingInNestedClasses(t *testing.T) {
	javaSource := []byte(`
public class Outer {
    public void outerMethod() {
    }
    
    public int process(String input) {
        return 0;
    }
    
    public static class NestedStatic {
        public void nestedMethod() {
        }
        
        public String convert(int value) {
            return "";
        }
        
        public String convert(int value, boolean flag) {
            return "";
        }
    }
    
    public class NestedInner {
        public void innerMethod(int x, int y) {
        }
    }
    
    public enum Status {
        ACTIVE, INACTIVE;
        
        public boolean isActive() {
            return this == ACTIVE;
        }
    }
    
    public record Data(int value) {
        public int doubled() {
            return value * 2;
        }
    }
}
`)

	// Parse and analyze
	tree := java.ParseJava(javaSource)
	defer tree.Close()

	ctx := &java.MigrationContext{
		JavaSource:      javaSource,
		AbstractClasses: make(map[string]bool),
		EnumConstants:   make(map[string]string),
		Constructors:    make(map[gosrc.Type][]java.FunctionData),
		Methods:         make(map[string][]java.FunctionData),
	}

	java.MigrateTree(ctx, tree)

	// Test 1: Outer class methods
	outerMethods, hasOuterMethod := ctx.Methods["outerMethod"]
	if !hasOuterMethod {
		t.Error("Expected 'outerMethod' to be tracked")
	}
	if len(outerMethods) != 1 {
		t.Errorf("Expected 1 'outerMethod', got %d", len(outerMethods))
	}
	if len(outerMethods) > 0 && len(outerMethods[0].ArgumentTypes) != 0 {
		t.Errorf("Expected 0 parameters for outerMethod, got %d", len(outerMethods[0].ArgumentTypes))
	}

	processMethods, hasProcess := ctx.Methods["process"]
	if !hasProcess {
		t.Error("Expected 'process' method to be tracked")
	}
	if len(processMethods) != 1 {
		t.Errorf("Expected 1 'process' method, got %d", len(processMethods))
	}
	if len(processMethods) > 0 {
		if len(processMethods[0].ArgumentTypes) != 1 {
			t.Errorf("Expected 1 parameter for process, got %d", len(processMethods[0].ArgumentTypes))
		}
		if processMethods[0].ArgumentTypes[0] != "string" {
			t.Errorf("Expected string parameter for process, got %v", processMethods[0].ArgumentTypes[0])
		}
	}

	// Test 2: Nested static class methods
	nestedMethods, hasNestedMethod := ctx.Methods["nestedMethod"]
	if !hasNestedMethod {
		t.Error("Expected 'nestedMethod' from nested static class to be tracked")
	}
	if len(nestedMethods) != 1 {
		t.Errorf("Expected 1 'nestedMethod', got %d", len(nestedMethods))
	}

	// Test 3: Overloaded methods in nested class
	convertMethods, hasConvert := ctx.Methods["convert"]
	if !hasConvert {
		t.Error("Expected 'convert' method from nested static class to be tracked")
	}
	if len(convertMethods) != 2 {
		t.Errorf("Expected 2 'convert' method overloads in nested class, got %d", len(convertMethods))
	}
	found1Param := false
	found2Param := false
	for _, method := range convertMethods {
		if len(method.ArgumentTypes) == 1 {
			if method.ArgumentTypes[0] != "int" {
				t.Errorf("Expected (int) for 1-param convert, got (%v)", method.ArgumentTypes[0])
			}
			found1Param = true
		} else if len(method.ArgumentTypes) == 2 {
			if method.ArgumentTypes[0] != "int" || method.ArgumentTypes[1] != "bool" {
				t.Errorf("Expected (int, bool) for 2-param convert, got (%v, %v)",
					method.ArgumentTypes[0], method.ArgumentTypes[1])
			}
			found2Param = true
		}
	}
	if !found1Param {
		t.Error("Did not find 1-parameter convert method in nested class")
	}
	if !found2Param {
		t.Error("Did not find 2-parameter convert method in nested class")
	}

	// Test 4: Nested inner class methods
	innerMethods, hasInnerMethod := ctx.Methods["innerMethod"]
	if !hasInnerMethod {
		t.Error("Expected 'innerMethod' from nested inner class to be tracked")
	}
	if len(innerMethods) != 1 {
		t.Errorf("Expected 1 'innerMethod', got %d", len(innerMethods))
	}
	if len(innerMethods) > 0 {
		if len(innerMethods[0].ArgumentTypes) != 2 {
			t.Errorf("Expected 2 parameters for innerMethod, got %d", len(innerMethods[0].ArgumentTypes))
		}
		if innerMethods[0].ArgumentTypes[0] != "int" || innerMethods[0].ArgumentTypes[1] != "int" {
			t.Errorf("Expected (int, int) for innerMethod, got (%v, %v)",
				innerMethods[0].ArgumentTypes[0], innerMethods[0].ArgumentTypes[1])
		}
	}

	// Test 5: Enum methods
	isActiveMethods, hasIsActive := ctx.Methods["isActive"]
	if !hasIsActive {
		t.Error("Expected 'isActive' method from enum to be tracked")
	}
	if len(isActiveMethods) != 1 {
		t.Errorf("Expected 1 'isActive' method, got %d", len(isActiveMethods))
	}
	if len(isActiveMethods) > 0 && len(isActiveMethods[0].ArgumentTypes) != 0 {
		t.Errorf("Expected 0 parameters for isActive, got %d", len(isActiveMethods[0].ArgumentTypes))
	}

	// Test 6: Record methods
	doubledMethods, hasDoubled := ctx.Methods["doubled"]
	if !hasDoubled {
		t.Error("Expected 'doubled' method from record to be tracked")
	}
	if len(doubledMethods) != 1 {
		t.Errorf("Expected 1 'doubled' method, got %d", len(doubledMethods))
	}
	if len(doubledMethods) > 0 && len(doubledMethods[0].ArgumentTypes) != 0 {
		t.Errorf("Expected 0 parameters for doubled, got %d", len(doubledMethods[0].ArgumentTypes))
	}
}
