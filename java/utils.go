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

// IterateChilden iterates over all children of a node and calls fn for each
func IterateChilden(node *tree_sitter.Node, fn func(child *tree_sitter.Node)) {
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		fn(&child)
	}
}

// IterateChildenWhile iterates over all children of a node while fn returns true
func IterateChildenWhile(node *tree_sitter.Node, fn func(child *tree_sitter.Node) bool) {
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		if !fn(&child) {
			return
		}
	}
}

func constructorName(ctx *MigrationContext, isPublic bool, ty gosrc.Type, params ...gosrc.Param) string {
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(gosrc.ToIdentifier("new", isPublic))
	nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(ty.ToSource()))
	if len(params) > 0 {
		nameBuilder.WriteString("From")
		for _, param := range params {
			nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(param.Ty.ToSource()))
		}
	}
	return nameBuilder.String()
}
