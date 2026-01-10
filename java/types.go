package java

import (
	"errors"
	"fmt"
	"strings"

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
	IterateChildren(methodNode, func(child *tree_sitter.Node) {
		if child.Kind() == "modifiers" {
			modText := child.Utf8Text(ctx.JavaSource)
			if strings.Contains(modText, modifier) {
				hasModifier = true
			}
		}
	})
	return hasModifier
}

// fatalTypeError handles a fatal type parsing error
// In strict mode, it exits immediately. In non-strict mode, it panics so the error can be recovered
func fatalTypeError(ctx *MigrationContext, node *tree_sitter.Node, err error) {
	FatalError(ctx, node, fmt.Sprintf("%v", err), "type parsing")
}

// TryParseType attempts to parse a tree-sitter node into a Go type
func TryParseType(ctx *MigrationContext, node *tree_sitter.Node) (gosrc.Type, bool) {
	switch node.Kind() {
	case "scoped_type_identifier":
		// For scoped types like Atom.Kind, we only use the second part (Kind)
		// since Go doesn't have nested types
		var typeName string
		// The last type_identifier child is the actual type we want
		IterateChildren(node, func(child *tree_sitter.Node) {
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
		goType = toGoType(ctx, typeName)
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
		goType = toGoType(ctx, typeName)
		return gosrc.Type(goType), true
	case "integral_type":
		return gosrc.TypeInt, true
	case "boolean_type":
		return gosrc.TypeBool, true
	case "floating_point_type":
		return gosrc.TypeFloat64, true
	case "array_type":
		typeNode := node.ChildByFieldName("element")
		ty, ok := TryParseType(ctx, typeNode)
		if !ok {
			fatalTypeError(ctx, typeNode, errors.New("unable to parse element type in array_type"))
		}
		return gosrc.Type("[]" + ty), true
	case "generic_type":
		var typeName string
		var typeParams []string
		IterateChildren(node, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "type_identifier":
				typeName = child.Utf8Text(ctx.JavaSource)
			case "type_arguments":
				IterateChildren(child, func(typeArg *tree_sitter.Node) {
					if typeArg.Kind() == "type_identifier" {
						typeParams = append(typeParams, typeArg.Utf8Text(ctx.JavaSource))
					}
				})
			}
		})
		switch typeName {
		// Array types
		case "ArrayDeque",
			"Deque",
			"Collection",
			"ArrayList",
			"List":
			Assert("List can have only one type param", len(typeParams) < 2)
			if len(typeParams) == 0 {
				return gosrc.Type("[]interface{}"), true
			}
			return gosrc.Type("[]" + typeParams[0]), true
		// Map types
		case "HashMap", "Map":
			Assert("Map can have at most two type params", len(typeParams) < 3)
			if len(typeParams) == 0 {
				return gosrc.Type("map[interface{}]interface{}"), true
			}
			if len(typeParams) == 1 {
				return gosrc.Type("map[" + typeParams[0] + "]interface{}"), true
			}
			return gosrc.Type("map[" + typeParams[0] + "]" + typeParams[1]), true
		default:
			fatalTypeError(ctx, node, errors.New("unhandled generic type : "+typeName))
		}
	}
	return "", false
}

func toGoType(ctx *MigrationContext, javaTy string) (goType string) {
	if configTy, ok := ctx.TypeMappings[javaTy]; ok {
		return configTy
	}
	switch javaTy {
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
	default:
		goType = javaTy
	}
	return goType
}

// IsArrayOrSliceType checks if a type is an array or slice
func IsArrayOrSliceType(ty gosrc.Type) bool {
	return strings.HasPrefix(string(ty), "[]")
}
