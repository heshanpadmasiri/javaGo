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

func TestArrayInitializerTranslation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Strings that must appear in output
	}{
		{
			name: "Simple array initializer with integers",
			input: `class Test {
    void test() {
        int[] numbers = new int[] { 1, 2, 3, 4, 5 };
    }
}`,
			expected: []string{
				"numbers := []int{1, 2, 3, 4, 5}",
			},
		},
		{
			name: "Array initializer with string literals",
			input: `class Test {
    void test() {
        String[] names = new String[] { "Alice", "Bob", "Charlie" };
    }
}`,
			expected: []string{
				"names := []string{",
				`"Alice"`,
				`"Bob"`,
				`"Charlie"`,
			},
		},
		{
			name: "Static final field with array initializer",
			input: `class Test {
    private static final int[] CONSTANTS = { 10, 20, 30 };
}`,
			expected: []string{
				"var CONSTANTS = []int{10, 20, 30}",
			},
		},
		{
			name: "Array initializer with enum-like constants (single line)",
			input: `class Test {
    void test() {
        Context[] alternatives = new Context[] { Context.START, Context.END };
    }
}`,
			expected: []string{
				"alternatives := []Context{Context_START, Context_END}",
			},
		},
		{
			name: "Multi-line array initializer with enum constants (actual failing case)",
			input: `class Test {
    private static final ParserRuleContext[] FUNC_TYPE_OR_DEF =
            { ParserRuleContext.RETURNS_KEYWORD, ParserRuleContext.FUNC_BODY };
}`,
			expected: []string{
				"var FUNC_TYPE_OR_DEF = []ParserRuleContext{",
				"ParserRuleContext_RETURNS_KEYWORD",
				"ParserRuleContext_FUNC_BODY",
			},
		},
		{
			name: "Array initializer in assignment (BallerinaParserErrorHandler case)",
			input: `class Test {
    void test(ParserRuleContext parentCtx) {
        ParserRuleContext[] alternatives = null;
        switch (parentCtx) {
            case ARG_LIST:
                alternatives = new ParserRuleContext[] { ParserRuleContext.COMMA,
                        ParserRuleContext.BINARY_OPERATOR,
                        ParserRuleContext.ARG_LIST_END };
                break;
        }
    }
}`,
			expected: []string{
				"alternatives = []ParserRuleContext{",
				"ParserRuleContext_COMMA",
				"ParserRuleContext_BINARY_OPERATOR",
				"ParserRuleContext_ARG_LIST_END",
			},
		},
		{
			name: "Empty array initializer",
			input: `class Test {
    void test() {
        int[] empty = new int[] { };
    }
}`,
			expected: []string{
				"empty := []int{}",
			},
		},
		{
			name: "Array initializer without explicit type (type inference)",
			input: `class Test {
    private static final int[] VALUES = { 1, 2, 3 };
}`,
			expected: []string{
				"var VALUES = []int{1, 2, 3}",
			},
		},
		{
			name: "Nested array access in initializer",
			input: `class Test {
    void test() {
        Object[] items = new Object[] { obj.field, another.method() };
    }
}`,
			expected: []string{
				"items := []interface{}{",
				"obj.field",
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

			// Check all expected strings are present
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected output to contain %q, but got:\n%s", exp, result)
				}
			}
		})
	}
}
