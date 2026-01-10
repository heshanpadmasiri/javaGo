package java

import (
	"os"
	"testing"

	"github.com/heshanpadmasiri/javaGo/gosrc"
)

func TestLoadConfigWithTypeMappings(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a Config.toml with type mappings
	configContent := `package_name = "testpkg"
license_header = "// Test License"

[type_mappings]
DiagnosticCode = "diagnostics.DiagnosticCode"
SyntaxKind = "diagnostics.SyntaxKind"
CustomType = "pkg.CustomType"
`
	if err := os.WriteFile("Config.toml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load config
	config := LoadConfig()

	// Verify package name and license header
	if config.PackageName != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got '%s'", config.PackageName)
	}
	if config.LicenseHeader != "// Test License" {
		t.Errorf("Expected license header '// Test License', got '%s'", config.LicenseHeader)
	}

	// Verify type mappings
	if config.TypeMappings == nil {
		t.Fatal("TypeMappings should not be nil")
	}
	if len(config.TypeMappings) != 3 {
		t.Errorf("Expected 3 type mappings, got %d", len(config.TypeMappings))
	}

	expected := map[string]string{
		"DiagnosticCode": "diagnostics.DiagnosticCode",
		"SyntaxKind":     "diagnostics.SyntaxKind",
		"CustomType":     "pkg.CustomType",
	}

	for key, expectedValue := range expected {
		if actualValue, ok := config.TypeMappings[key]; !ok {
			t.Errorf("Missing type mapping for '%s'", key)
		} else if actualValue != expectedValue {
			t.Errorf("For key '%s', expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestLoadConfigWithoutTypeMappings(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a Config.toml without type mappings
	configContent := `package_name = "testpkg"
license_header = "// Test License"
`
	if err := os.WriteFile("Config.toml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load config
	config := LoadConfig()

	// Verify package name and license header
	if config.PackageName != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got '%s'", config.PackageName)
	}
	if config.LicenseHeader != "// Test License" {
		t.Errorf("Expected license header '// Test License', got '%s'", config.LicenseHeader)
	}

	// Verify type mappings is nil (backward compatibility)
	if config.TypeMappings != nil && len(config.TypeMappings) != 0 {
		t.Errorf("Expected TypeMappings to be nil or empty, got %v", config.TypeMappings)
	}
}

func TestLoadConfigEmptyTypeMappings(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a Config.toml with empty type mappings section
	configContent := `package_name = "testpkg"

[type_mappings]
`
	if err := os.WriteFile("Config.toml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load config
	config := LoadConfig()

	// Verify package name
	if config.PackageName != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got '%s'", config.PackageName)
	}

	// Verify type mappings is empty (either nil or empty map is acceptable)
	if config.TypeMappings != nil && len(config.TypeMappings) != 0 {
		t.Errorf("Expected empty TypeMappings, got %d entries: %v", len(config.TypeMappings), config.TypeMappings)
	}
}

func TestLoadConfigNonexistent(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory (no Config.toml file)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Load config
	config := LoadConfig()

	// Verify defaults
	if config.PackageName != gosrc.PackageName {
		t.Errorf("Expected default package name '%s', got '%s'", gosrc.PackageName, config.PackageName)
	}
	if config.LicenseHeader != "" {
		t.Errorf("Expected empty license header, got '%s'", config.LicenseHeader)
	}
	if config.TypeMappings != nil && len(config.TypeMappings) != 0 {
		t.Errorf("Expected TypeMappings to be nil or empty, got %v", config.TypeMappings)
	}
}

func TestNewMigrationContextWithTypeMappings(t *testing.T) {
	javaSource := []byte("public class Foo {}")
	typeMappings := map[string]string{
		"DiagnosticCode": "diagnostics.DiagnosticCode",
		"CustomType":     "pkg.CustomType",
	}

	ctx := NewMigrationContext(javaSource, "test.java", false, typeMappings)

	// Verify type mappings are set
	if ctx.TypeMappings == nil {
		t.Fatal("TypeMappings should not be nil")
	}
	if len(ctx.TypeMappings) != 2 {
		t.Errorf("Expected 2 type mappings, got %d", len(ctx.TypeMappings))
	}
	if ctx.TypeMappings["DiagnosticCode"] != "diagnostics.DiagnosticCode" {
		t.Errorf("Expected DiagnosticCode mapping, got '%s'", ctx.TypeMappings["DiagnosticCode"])
	}
	if ctx.TypeMappings["CustomType"] != "pkg.CustomType" {
		t.Errorf("Expected CustomType mapping, got '%s'", ctx.TypeMappings["CustomType"])
	}
}

func TestNewMigrationContextWithNilTypeMappings(t *testing.T) {
	javaSource := []byte("public class Foo {}")

	ctx := NewMigrationContext(javaSource, "test.java", false, nil)

	// Verify type mappings is initialized as empty map (not nil)
	if ctx.TypeMappings == nil {
		t.Fatal("TypeMappings should not be nil, it should be an empty map")
	}
	if len(ctx.TypeMappings) != 0 {
		t.Errorf("Expected empty TypeMappings, got %d entries", len(ctx.TypeMappings))
	}
}

func TestTypeMappingInConversion(t *testing.T) {
	// Create Java source with a custom type
	javaSource := []byte(`public class Diagnostic {
    private DiagnosticCode code;
    private CustomType custom;
    
    public DiagnosticCode getCode() {
        return code;
    }
    
    public CustomType getCustom() {
        return custom;
    }
}`)

	// Create type mappings
	typeMappings := map[string]string{
		"DiagnosticCode": "diagnostics.DiagnosticCode",
		"CustomType":     "mypkg.MyCustomType",
	}

	// Parse and migrate
	tree := ParseJava(javaSource)
	defer tree.Close()

	ctx := NewMigrationContext(javaSource, "test.java", true, typeMappings)
	MigrateTree(ctx, tree)

	config := gosrc.Config{
		PackageName:   "test",
		LicenseHeader: "",
	}
	result := ctx.Source.ToSource(config)

	// Expected Go output with type mappings applied
	expected := `package test

type Diagnostic struct {
    code diagnostics.DiagnosticCode
    custom mypkg.MyCustomType
}

func NewDiagnostic() Diagnostic {
this := Diagnostic{};
return this
}

func (this *Diagnostic) GetCode() diagnostics.DiagnosticCode {
// migrated from test.java:5:5
return code
}

func (this *Diagnostic) GetCustom() mypkg.MyCustomType {
// migrated from test.java:9:5
return custom
}

`

	if result != expected {
		t.Errorf("Output does not match expected.\n--- Got ---\n%s\n--- Expected ---\n%s", result, expected)
	}
}

func TestTypeMappingPrecedenceOverBuiltins(t *testing.T) {
	// Create Java source that uses String type
	javaSource := []byte(`public class Example {
    private String name;
    
    public String getName() {
        return name;
    }
}`)

	// Create type mapping that overrides the built-in String mapping
	typeMappings := map[string]string{
		"String": "mystring.CustomString",
	}

	// Parse and migrate
	tree := ParseJava(javaSource)
	defer tree.Close()

	ctx := NewMigrationContext(javaSource, "test.java", true, typeMappings)
	MigrateTree(ctx, tree)

	config := gosrc.Config{
		PackageName:   "test",
		LicenseHeader: "",
	}
	result := ctx.Source.ToSource(config)

	// Expected Go output with String type overridden
	expected := `package test

type Example struct {
    name mystring.CustomString
}

func NewExample() Example {
this := Example{};
return this
}

func (this *Example) GetName() mystring.CustomString {
// migrated from test.java:4:5
return name
}

`

	if result != expected {
		t.Errorf("Output does not match expected.\n--- Got ---\n%s\n--- Expected ---\n%s", result, expected)
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create an invalid Config.toml
	configContent := `package_name = "testpkg"
this is invalid toml ][[[
`
	if err := os.WriteFile("Config.toml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load config - should return defaults without panicking
	config := LoadConfig()

	// Verify defaults are returned
	if config.PackageName != gosrc.PackageName {
		t.Errorf("Expected default package name '%s', got '%s'", gosrc.PackageName, config.PackageName)
	}
	if config.LicenseHeader != "" {
		t.Errorf("Expected empty license header, got '%s'", config.LicenseHeader)
	}
}

func TestConfigIntegration(t *testing.T) {
	// This is an end-to-end test that verifies the entire flow:
	// Config.toml -> LoadConfig -> NewMigrationContext -> Migration

	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a Config.toml with all settings
	configContent := `package_name = "myapp"
license_header = "// Copyright 2024"

[type_mappings]
ErrorCode = "errors.Code"
Status = "status.Status"
`
	if err := os.WriteFile("Config.toml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create Java source
	javaSource := []byte(`public class Service {
    private ErrorCode error;
    private Status status;
}`)

	// Load config
	config := LoadConfig()

	// Create migration context with config's type mappings
	tree := ParseJava(javaSource)
	defer tree.Close()

	ctx := NewMigrationContext(javaSource, "Service.java", true, config.TypeMappings)
	MigrateTree(ctx, tree)

	// Generate Go source
	result := ctx.Source.ToSource(config)

	// Expected complete output with package, license, and type mappings
	expected := `// Copyright 2024

package myapp

type Service struct {
    error errors.Code
    status status.Status
}

func NewService() Service {
this := Service{};
return this
}

`

	if result != expected {
		t.Errorf("Output does not match expected.\n--- Got ---\n%s\n--- Expected ---\n%s", result, expected)
	}
}
