package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestIfElseTranslation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Strings that must appear in output
	}{
		{
			name: "Simple if/else",
			input: `class Test {
    void test() {
        if (x) {
            a();
        } else {
            b();
        }
    }
}`,
			expected: []string{
				"if x {",
				"}else {",
				"this.a()",
				"this.b()",
			},
		},
		{
			name: "If/else if/else",
			input: `class Test {
    void test() {
        if (x > 0) {
            positive();
        } else if (x < 0) {
            negative();
        } else {
            zero();
        }
    }
}`,
			expected: []string{
				"if (x > 0) {",
				"}else if (x < 0) {",
				"}else {",
				"this.positive()",
				"this.negative()",
				"this.zero()",
			},
		},
		{
			name: "Multiple else-if with final else",
			input: `class Test {
    void test() {
        if (x == 1) {
            one();
        } else if (x == 2) {
            two();
        } else if (x == 3) {
            three();
        } else {
            other();
        }
    }
}`,
			expected: []string{
				"if (x == 1) {",
				"}else if (x == 2) {",
				"}else if (x == 3) {",
				"}else {",
				"this.one()",
				"this.two()",
				"this.three()",
				"this.other()",
			},
		},
		{
			name: "If/else if without final else",
			input: `class Test {
    void test() {
        if (x == 1) {
            one();
        } else if (x == 2) {
            two();
        }
    }
}`,
			expected: []string{
				"if (x == 1) {",
				"}else if (x == 2) {",
				"this.one()",
				"this.two()",
			},
		},
		{
			name: "Nested if/else",
			input: `class Test {
    void test() {
        if (x) {
            if (y) {
                a();
            } else {
                b();
            }
        } else {
            c();
        }
    }
}`,
			expected: []string{
				"if x {",
				"if y {",
				"}else {",
				"this.a()",
				"this.b()",
				"this.c()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "test_*.java")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run translation
			content, err := os.ReadFile(tmpfile.Name())
			if err != nil {
				t.Fatal(err)
			}

			tree := parseJava(content)
			defer tree.Close()

			ctx := &MigrationContext{javaSource: content}
			migrateTree(ctx, tree)
			result := ctx.source.ToSource()

			// Restore stdout
			w.Close()
			var buf bytes.Buffer
			buf.ReadFrom(r)
			os.Stdout = oldStdout

			// Check all expected strings are present
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected output to contain %q, but got:\n%s", exp, result)
				}
			}
		})
	}
}

