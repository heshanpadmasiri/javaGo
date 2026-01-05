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

// UnhandledChild reports an unhandled child node and exits
func UnhandledChild(ctx *MigrationContext, node *tree_sitter.Node, parentName string) {
	msg := fmt.Sprintf("unhandled %s child node kind: %s\nS-expression: %s\nSource: %s",
		parentName,
		node.Kind(),
		node.ToSexp(),
		node.Utf8Text(ctx.JavaSource))
	fmt.Fprintf(os.Stderr, "Fatal: %s\n", msg)
	os.Exit(1)
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
