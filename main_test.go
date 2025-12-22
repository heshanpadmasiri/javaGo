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

			ctx := &MigrationContext{
				javaSource:      content,
				abstractClasses: make(map[string]bool),
			}
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
				`names := []string{"Alice", "Bob", "Charlie"}`,
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
				"var FUNC_TYPE_OR_DEF = []ParserRuleContext{ParserRuleContext_RETURNS_KEYWORD, ParserRuleContext_FUNC_BODY}",
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
				"alternatives = []ParserRuleContext{ParserRuleContext_COMMA, ParserRuleContext_BINARY_OPERATOR, ParserRuleContext_ARG_LIST_END}",
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
				"items := []interface{}{obj.field, this.another.method()}",
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

			ctx := &MigrationContext{
				javaSource:      content,
				abstractClasses: make(map[string]bool),
			}
			migrateTree(ctx, tree)
			result := ctx.source.ToSource()

			// Verify exact structure - check that expected strings appear in order
			resultLines := strings.Split(result, "\n")
			lineIndex := 0
			for _, exp := range tt.expected {
				found := false
				// Search from current position forward to ensure order
				for i := lineIndex; i < len(resultLines); i++ {
					if strings.Contains(resultLines[i], exp) {
						found = true
						lineIndex = i + 1
						break
					}
				}
				if !found {
					t.Errorf("Expected output to contain %q (in order), but got:\n%s", exp, result)
					return
				}
			}
		})
	}
}

func TestTryCatchTranslation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Strings that must appear in output
	}{
		{
			name: "Single catch clause with IllegalStateException",
			input: `class Test {
    Solution getCompletion(ParserRuleContext context, STToken nextToken) {
        ArrayDeque<ParserRuleContext> tempCtxStack = this.ctxStack;
        this.ctxStack = getCtxStackSnapshot();

        Solution sol;
        try {
            sol = getInsertSolution(context);
        } catch (IllegalStateException exception) {
            assert false : "Oh no, something went bad";
            sol = getResolution(context, nextToken);
        }

        this.ctxStack = tempCtxStack;
        return sol;
    }
}`,
			expected: []string{
				"var sol Solution",
				"func() {",
				"defer func() {",
				"if r := recover(); r != nil {",
				"if _, ok := r.(IllegalStateException); ok {",
				"sol = this.getResolution(context, nextToken)",
				"panic(r)",
				"sol = this.getInsertSolution(context)",
			},
		},
		{
			name: "Multiple catch clauses",
			input: `class Test {
    void test() {
        try {
            riskyOperation();
        } catch (IllegalArgumentException e) {
            handleIllegal(e);
        } catch (IllegalStateException e) {
            handleState(e);
        }
    }
}`,
			expected: []string{
				"func() {",
				"defer func() {",
				"if r := recover(); r != nil {",
				"if _, ok := r.(IllegalArgumentException); ok {",
				"this.handleIllegal(e)",
				"if _, ok := r.(IllegalStateException); ok {",
				"this.handleState(e)",
			},
		},
		{
			name: "Try-catch with finally block",
			input: `class Test {
    void test() {
        try {
            doSomething();
        } catch (Exception e) {
            handleError(e);
        } finally {
            cleanup();
        }
    }
}`,
			expected: []string{
				"func() {",
				"defer func() {",
				"if r := recover(); r != nil {",
				"if _, ok := r.(Exception); ok {",
				"this.handleError(e)",
				"this.cleanup()",
			},
		},
		{
			name: "Try-catch with variable assignment in try block",
			input: `class Test {
    int calculate() {
        int result;
        try {
            result = compute();
        } catch (RuntimeException e) {
            result = defaultValue();
        }
        return result;
    }
}`,
			expected: []string{
				"var result int",
				"func() {",
				"defer func() {",
				"if r := recover(); r != nil {",
				"if _, ok := r.(RuntimeException); ok {",
				"result = this.defaultValue()",
				"result = this.compute()",
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

			ctx := &MigrationContext{
				javaSource:      content,
				abstractClasses: make(map[string]bool),
			}
			migrateTree(ctx, tree)
			result := ctx.source.ToSource()

			// Verify exact structure - check that expected strings appear in order
			resultLines := strings.Split(result, "\n")
			lineIndex := 0
			for _, exp := range tt.expected {
				found := false
				// Search from current position forward to ensure order
				for i := lineIndex; i < len(resultLines); i++ {
					if strings.Contains(resultLines[i], exp) {
						found = true
						lineIndex = i + 1
						break
					}
				}
				if !found {
					t.Errorf("Expected output to contain %q (in order), but got:\n%s", exp, result)
					return
				}
			}
		})
	}
}

func TestAbstractMethodTranslation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Strings that must appear in output
	}{
		{
			name: "Abstract method with void return type",
			input: `abstract class Test {
    public abstract void doSomething();
}`,
			expected: []string{
				"type TestData interface {",
				"}",
				"type Test interface {",
				"    TestData",
				"    DoSomething()",
				"}",
				"type TestBase struct {",
				"}",
				"type TestMethods struct {",
				"    Self Test",
				"}",
			},
		},
		{
			name: "Abstract method with return type",
			input: `abstract class Test {
    public abstract int calculate();
}`,
			expected: []string{
				"type TestData interface {",
				"}",
				"type Test interface {",
				"    TestData",
				"    Calculate() int",
				"}",
				"type TestBase struct {",
				"}",
				"type TestMethods struct {",
				"    Self Test",
				"}",
			},
		},
		{
			name: "Abstract method with parameters",
			input: `abstract class Test {
    public abstract String process(String input, int count);
}`,
			expected: []string{
				"type TestData interface {",
				"}",
				"type Test interface {",
				"    TestData",
				"    Process(input string, count int) string",
				"}",
				"type TestBase struct {",
				"}",
				"type TestMethods struct {",
				"    Self Test",
				"}",
			},
		},
		{
			name: "Non-abstract method should not have panic",
			input: `class Test {
    public void doSomething() {
        System.out.println("Hello");
    }
}`,
			expected: []string{
				"System.out.println",
			},
		},
		{
			name: "Abstract and non-abstract methods in same class",
			input: `abstract class Test {
    public abstract void abstractMethod();
    public void concreteMethod() {
        System.out.println("Concrete");
    }
}`,
			expected: []string{
				"type TestData interface {",
				"}",
				"type Test interface {",
				"    TestData",
				"    AbstractMethod()",
				"    ConcreteMethod()",
				"}",
				"type TestBase struct {",
				"}",
				"type TestMethods struct {",
				"    Self Test",
				"}",
				"func (m *TestMethods) ConcreteMethod() {",
				"m.Self.System.out.println",
				"}",
			},
		},
		{
			name: "Abstract class with fields and methods",
			input: `abstract class Foo {
    int a;
    abstract int f();
    int b() {
        return f() + a;
    }
}`,
			expected: []string{
				"type FooData interface {",
				"    GetA() int",
				"    SetA(a int)",
				"}",
				"type Foo interface {",
				"    FooData",
				"    F() int",
				"    B() int",
				"}",
				"type FooBase struct {",
				"    A int",
				"}",
				"type FooMethods struct {",
				"    Self Foo",
				"}",
				"func (b *FooBase) GetA() int {",
				"return b.A",
				"}",
				"func (b *FooBase) SetA(a int) {",
				"b.A = a",
				"}",
				"func (m *FooMethods) B() int {",
				"return (m.Self.F() + m.Self.GetA())",
				"}",
			},
		},
		{
			name: "Subclass extending abstract class",
			input: `abstract class Foo {
    int a;
    abstract int f();
    int b() {
        return f() + a;
    }
}
class Bar extends Foo {
    int f() {
        return 42;
    }
}`,
			expected: []string{
				"type Bar struct",
				"FooBase",
				"FooMethods",
				"func (b *Bar) F() int",
				"return 42",
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

			ctx := &MigrationContext{
				javaSource:      content,
				abstractClasses: make(map[string]bool),
			}
			migrateTree(ctx, tree)
			result := ctx.source.ToSource()

			// Verify exact structure - check that expected strings appear in order
			resultLines := strings.Split(result, "\n")
			lineIndex := 0
			for _, exp := range tt.expected {
				found := false
				// Search from current position forward to ensure order
				for i := lineIndex; i < len(resultLines); i++ {
					if strings.Contains(resultLines[i], exp) {
						found = true
						lineIndex = i + 1
						break
					}
				}
				if !found {
					t.Errorf("Expected output to contain %q (in order), but got:\n%s", exp, result)
					return
				}
			}
		})
	}
}
