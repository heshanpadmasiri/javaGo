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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(goFile, []byte(content), 0644)
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
			tree := parseJava(javaContent)
			defer tree.Close()

			ctx := &MigrationContext{
				javaSource:      javaContent,
				abstractClasses: make(map[string]bool),
			}
			migrateTree(ctx, tree)
			result := ctx.source.ToSource()

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
