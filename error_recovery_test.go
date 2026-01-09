package main

import (
	"strings"
	"testing"

	"github.com/heshanpadmasiri/javaGo/java"
)

func TestErrorRecovery(t *testing.T) {
	// Java source with unsupported annotation_type_declaration
	javaSource := []byte(`
class TestAnnotation {
    int validField1 = 5;
    
    public int getField1() {
        return validField1;
    }
    
    // Annotation declarations are not supported
    @interface MyAnnotation {
    }
    
    int validField2 = 10;
    
    public int getField2() {
        return validField2;
    }
}
`)

	t.Run("non-strict mode continues on error", func(t *testing.T) {
		tree := java.ParseJava(javaSource)
		defer tree.Close()

		ctx := java.NewMigrationContext(javaSource, "test.java", false) // non-strict mode
		java.MigrateTree(ctx, tree)

		// Check that we collected an error
		if len(ctx.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(ctx.Errors))
		}

		if len(ctx.Errors) > 0 {
			err := ctx.Errors[0]
			if !strings.Contains(err.Message, "annotation_type_declaration") {
				t.Errorf("Expected error about annotation_type_declaration, got: %s", err.Message)
			}
			if !strings.Contains(err.Location, "testAnnotation") {
				t.Errorf("Expected error location to mention testAnnotation, got: %s", err.Location)
			}
		}

		// Check that we have a FailedMigration entry
		if len(ctx.Source.FailedMigrations) != 1 {
			t.Errorf("Expected 1 failed migration, got %d", len(ctx.Source.FailedMigrations))
		}

		// Check that valid members were still migrated
		if len(ctx.Source.Structs) != 1 {
			t.Errorf("Expected struct to be created despite error, got %d structs", len(ctx.Source.Structs))
		}

		// Check that both fields were migrated
		if len(ctx.Source.Structs) > 0 {
			fields := ctx.Source.Structs[0].Fields
			if len(fields) != 2 {
				t.Errorf("Expected 2 fields to be migrated despite error, got %d", len(fields))
			}
		}

		// Check that both methods were migrated
		if len(ctx.Source.Methods) != 2 {
			t.Errorf("Expected 2 methods to be migrated despite error, got %d", len(ctx.Source.Methods))
		}
	})

	// Note: We can't easily test strict mode calling os.Exit(1) in a unit test
	// The -Werror flag behavior is tested through integration tests
}
