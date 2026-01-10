package java

import (
	"fmt"
	"os"
	"strings"

	"github.com/heshanpadmasiri/javaGo/gosrc"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// ParseJava parses Java source code and returns a tree-sitter tree
func ParseJava(source []byte) *tree_sitter.Tree {
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_java.Language()))
	tree := parser.Parse(source, nil)
	return tree
}

// TryGetChildByFieldName attempts to find a child node by field name
func TryGetChildByFieldName(node *tree_sitter.Node, fieldName string) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.FieldNameForNamedChild(uint32(i)) == fieldName {
			return child
		}
	}
	return nil
}

// MigrationPanic represents a panic during migration with structured error information
type MigrationPanic struct {
	Message    string
	JavaSource string
	SExpr      string
	NodeKind   string
	ParentName string
}

// UnhandledChild reports an unhandled child node and exits (in strict mode) or panics (in non-strict mode)
func UnhandledChild(ctx *MigrationContext, node *tree_sitter.Node, parentName string) {
	msg := fmt.Sprintf("unhandled %s child node kind: %s\nS-expression: %s\nSource: %s",
		parentName,
		node.Kind(),
		node.ToSexp(),
		node.Utf8Text(ctx.JavaSource))

	if ctx.StrictMode {
		fmt.Fprintf(os.Stderr, "Fatal: %s\n", msg)
		os.Exit(1)
	}

	// In non-strict mode, panic with structured error info
	panic(MigrationPanic{
		Message:    msg,
		JavaSource: node.Utf8Text(ctx.JavaSource),
		SExpr:      node.ToSexp(),
		NodeKind:   node.Kind(),
		ParentName: parentName,
	})
}

// FatalError reports a fatal error and exits (in strict mode) or panics (in non-strict mode)
// This is useful for errors during type parsing or other operations where graceful recovery is desired
func FatalError(ctx *MigrationContext, node *tree_sitter.Node, msg string, parentName string) {
	if ctx.StrictMode {
		fmt.Fprintf(os.Stderr, "Fatal: %s: %s\n", node.ToSexp(), msg)
		os.Exit(1)
	}

	// In non-strict mode, panic with structured error info
	panic(MigrationPanic{
		Message:    msg,
		JavaSource: node.Utf8Text(ctx.JavaSource),
		SExpr:      node.ToSexp(),
		NodeKind:   node.Kind(),
		ParentName: parentName,
	})
}

// Assert checks a condition and exits with an error message if false
func Assert(msg string, condition bool) {
	if condition {
		return
	}
	fmt.Fprintf(os.Stderr, "Assertion failed: %s\n", msg)
	os.Exit(1)
}

// IterateChildren iterates over all children of a node and calls fn for each
func IterateChildren(node *tree_sitter.Node, fn func(child *tree_sitter.Node)) {
	if node == nil {
		return
	}
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		fn(&child)
	}
}

// IterateChildrenWhile iterates over all children of a node while fn returns true
func IterateChildrenWhile(node *tree_sitter.Node, fn func(child *tree_sitter.Node) bool) {
	if node == nil {
		return
	}
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		if !fn(&child) {
			return
		}
	}
}

func constructorName(ctx *MigrationContext, isPublic bool, ty gosrc.Type, params ...gosrc.Param) string {
	var paramTys []gosrc.Type
	for _, param := range params {
		paramTys = append(paramTys, param.Ty)
	}
	constructors, hasConstructors := ctx.Constructors[ty]
	if hasConstructors {
		name, hasMatching := findOverloadedMethod(constructors, paramTys)
		if hasMatching {
			return name
		}
	}
	// Return default constructor name: new${Ty}
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(gosrc.ToIdentifier("new", isPublic))
	nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(ty.ToSource()))
	return nameBuilder.String()
}

func findOverloadedMethod(methods []FunctionData, parameterTys []gosrc.Type) (string, bool) {
	for _, fn := range methods {
		argTys := fn.ArgumentTypes
		if len(argTys) != len(parameterTys) {
			continue
		}
		matched := true
		for i, argTy := range argTys {
			if parameterTys[i] != argTy {
				matched = false
				break
			}
		}
		if matched {
			return fn.Name, true
		}
	}
	return "", false
}

func tryGuessOverloadedMethod(methods []FunctionData, nParams int) (string, bool, bool) {
	const (
		defaultName = ""
	)
	name := defaultName
	multipeMatch := false
	for _, fn := range methods {
		argTys := fn.ArgumentTypes
		if len(argTys) != nParams {
			continue
		}
		if name != defaultName {
			multipeMatch = true
		} else {
			name = fn.Name
		}
	}
	return name, name != defaultName, multipeMatch
}

func overloadedName(baseName string, args []gosrc.Type) string {
	if len(args) == 0 {
		return baseName + "WithoutArgs"
	}
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(baseName)
	nameBuilder.WriteString("With")
	for _, ty := range args {
		nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(ty.ToSource()))
	}
	return nameBuilder.String()
}

// getConvertedMethodName looks up the converted method name for an invocation
// Handles overloaded method resolution by argument count
// Returns: (convertedName, found, multipleMatches)
func getConvertedMethodName(ctx *MigrationContext, methodName string, argCount int) (string, bool, bool) {
	methods, exists := ctx.Methods[methodName]
	if !exists {
		// Maybe it is public?
		methods, exists = ctx.Methods[gosrc.ToIdentifier(methodName, true)]
		if !exists {
			// Method not tracked - use original name
			return methodName, false, false
		}
	}

	if len(methods) == 1 {
		// No overloading - return the single method name
		return methods[0].Name, true, false
	}

	// Multiple methods - try to guess by argument count
	return tryGuessOverloadedMethod(methods, argCount)
}

// tryMigrateMember wraps a migration function with panic recovery
// Returns a FailedMigration if the migration panics, nil otherwise
func tryMigrateMember(ctx *MigrationContext, location string, node *tree_sitter.Node, fn func()) *gosrc.FailedMigration {
	defer func() {
		if r := recover(); r != nil {
			// Let strict mode panics propagate
			if ctx.StrictMode {
				panic(r)
			}
			// Otherwise this is handled by handleMigrationPanic below
		}
	}()

	// Set up inner recovery to capture the panic and convert to FailedMigration
	var failed *gosrc.FailedMigration
	func() {
		defer func() {
			if r := recover(); r != nil {
				failed = handleMigrationPanic(ctx, location, node, r)
			}
		}()
		fn()
	}()

	return failed
}

// handleMigrationPanic handles a panic during migration by recording the error
// and returning a FailedMigration placeholder
func handleMigrationPanic(ctx *MigrationContext, location string, node *tree_sitter.Node, r any) *gosrc.FailedMigration {
	var err MigrationError

	switch v := r.(type) {
	case MigrationPanic:
		err = MigrationError{
			Location:   location,
			JavaSource: v.JavaSource,
			SExpr:      v.SExpr,
			Message:    v.Message,
			NodeKind:   v.NodeKind,
		}
	default:
		// Handle unexpected panics
		javaSource := ""
		sexpr := ""
		nodeKind := ""
		if node != nil {
			javaSource = node.Utf8Text(ctx.JavaSource)
			sexpr = node.ToSexp()
			nodeKind = node.Kind()
		}
		err = MigrationError{
			Location:   location,
			JavaSource: javaSource,
			SExpr:      sexpr,
			Message:    fmt.Sprintf("unexpected panic: %v", r),
			NodeKind:   nodeKind,
		}
	}

	ctx.Errors = append(ctx.Errors, err)

	// TODO: this should be controlled by the migration context using a channel
	// Print to stderr immediately
	fmt.Fprintf(os.Stderr, "Error migrating %s: %s\n", location, err.Message)

	// Return FailedMigration placeholder
	return &gosrc.FailedMigration{
		ErrorMessage: err.Message,
		JavaSource:   err.JavaSource,
		SExpr:        err.SExpr,
		Location:     location,
	}
}
