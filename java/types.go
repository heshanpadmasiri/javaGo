package java

import (
	"errors"
	"strings"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Modifier bit flags
const (
	PUBLIC modifiers = 1 << iota
	PRIVATE
	PROTECTED
	STATIC
	FINAL
	ABSTRACT
)

// modifiers represents Java modifiers as a bitmask
type modifiers uint16

func (m modifiers) String() string {
	var parts []string
	if m&PUBLIC != 0 {
		parts = append(parts, "public")
	}
	if m&PRIVATE != 0 {
		parts = append(parts, "private")
	}
	if m&PROTECTED != 0 {
		parts = append(parts, "protected")
	}
	if m&STATIC != 0 {
		parts = append(parts, "static")
	}
	if m&FINAL != 0 {
		parts = append(parts, "final")
	}
	if m&ABSTRACT != 0 {
		parts = append(parts, "abstract")
	}
	return strings.Join(parts, " ")
}

func (m modifiers) isPublic() bool {
	return m&PUBLIC != 0
}

// ParseModifiers parses modifier string into a modifiers bitmask
func ParseModifiers(source string) modifiers {
	parts := strings.Fields(source)
	var mods modifiers
	for _, part := range parts {
		switch part {
		case "public":
			mods |= PUBLIC
		case "private":
			mods |= PRIVATE
		case "protected":
			mods |= PROTECTED
		case "static":
			mods |= STATIC
		case "final":
			mods |= FINAL
		case "abstract":
			mods |= ABSTRACT
		}
	}
	return mods
}

// HasModifier checks if a node has a specific modifier
func HasModifier(ctx *MigrationContext, methodNode *tree_sitter.Node, modifier string) bool {
	hasModifier := false
	IterateChilden(methodNode, func(child *tree_sitter.Node) {
		if child.Kind() == "modifiers" {
			modText := child.Utf8Text(ctx.JavaSource)
			if strings.Contains(modText, modifier) {
				hasModifier = true
			}
		}
	})
	return hasModifier
}

// TryParseType attempts to parse a tree-sitter node into a Go type
func TryParseType(ctx *MigrationContext, node *tree_sitter.Node) (gosrc.Type, bool) {
	switch node.Kind() {
	case "scoped_type_identifier":
		// For scoped types like Atom.Kind, we only use the second part (Kind)
		// since Go doesn't have nested types
		var typeName string
		// The last type_identifier child is the actual type we want
		IterateChilden(node, func(child *tree_sitter.Node) {
			if child.Kind() == "type_identifier" {
				typeName = child.Utf8Text(ctx.JavaSource)
			}
		})
		if typeName == "" {
			return "", false
		}
		// Process the type name the same way as a regular type_identifier
		var goType string
		unwantedPrefixes := []string{"Abstract", "LexerTerminals", "ST"}
		for _, prefix := range unwantedPrefixes {
			if strings.HasPrefix(typeName, prefix) {
				goType = typeName[len(prefix):]
				return gosrc.Type(goType), true
			}
		}
		if strings.HasPrefix(typeName, "ST") {
			goType = "internal." + typeName
			return gosrc.Type(goType), true
		}
		// FIXME: extract this to a method
		switch typeName {
		case "Object":
			goType = "interface{}"
		case "String":
			goType = "string"
		case "Integer":
			goType = "int"
		case "Long":
			goType = "int64"
		case "Boolean":
			goType = "bool"
		// TODO: instead of hardcoding these make is possible to supply them using config
		case "DiagnosticCode":
			goType = "diagnostics.DiagnosticCode"
		case "SyntaxKind":
			goType = "common.SyntaxKind"
		default:
			goType = typeName
		}
		return gosrc.Type(goType), true
	case "type_identifier":
		var goType string
		typeName := node.Utf8Text(ctx.JavaSource)
		unwantedPrefixes := []string{"Abstract", "LexerTerminals", "ST"}
		for _, prefix := range unwantedPrefixes {
			if strings.HasPrefix(typeName, prefix) {
				goType = typeName[len(prefix):]
				return gosrc.Type(goType), true
			}
		}
		if strings.HasPrefix(typeName, "ST") {
			goType = "internal." + typeName
			return gosrc.Type(goType), true
		}
		switch typeName {
		case "Object":
			goType = "interface{}"
		case "String":
			goType = "string"
		case "Integer":
			goType = "int"
		case "Long":
			goType = "int64"
		case "Boolean":
			goType = "bool"
		case "DiagnosticCode":
			goType = "diagnostics.DiagnosticCode"
		case "SyntaxKind":
			goType = "common.SyntaxKind"
		default:
			goType = typeName
		}
		return gosrc.Type(goType), true
	case "integral_type":
		return gosrc.TypeInt, true
	case "boolean_type":
		return gosrc.TypeBool, true
	case "array_type":
		typeNode := node.ChildByFieldName("element")
		ty, ok := TryParseType(ctx, typeNode)
		if !ok {
			diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse element type in array_type"))
		}
		return gosrc.Type("[]" + ty), true
	case "generic_type":
		var typeName string
		var typeParams []string
		IterateChilden(node, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "type_identifier":
				typeName = child.Utf8Text(ctx.JavaSource)
			case "type_arguments":
				IterateChilden(child, func(typeArg *tree_sitter.Node) {
					if typeArg.Kind() == "type_identifier" {
						typeParams = append(typeParams, typeArg.Utf8Text(ctx.JavaSource))
					}
				})
			}
		})
		switch typeName {
		// Array types
		case "ArrayDeque":
			fallthrough
		case "Deque":
			fallthrough
		case "Collection":
			fallthrough
		case "ArrayList":
			fallthrough
		case "List":
			Assert("List can have only one type param", len(typeParams) < 2)
			if len(typeParams) == 0 {
				return gosrc.Type("[]interface{}"), true
			}
			return gosrc.Type("[]" + typeParams[0]), true
		// Map types
		case "HashMap":
			fallthrough
		case "Map":
			Assert("Map can have at most two type params", len(typeParams) < 3)
			if len(typeParams) == 0 {
				return gosrc.Type("map[interface{}]interface{}"), true
			}
			if len(typeParams) == 1 {
				return gosrc.Type("map[" + typeParams[0] + "]interface{}"), true
			}
			return gosrc.Type("map[" + typeParams[0] + "]" + typeParams[1]), true
		default:
			diagnostics.Fatal(node.ToSexp(), errors.New("unhandled generic type : "+typeName))
		}
	}

	return "", false
}

// IsArrayOrSliceType checks if a type is an array or slice
func IsArrayOrSliceType(ty gosrc.Type) bool {
	return strings.HasPrefix(string(ty), "[]")
}
