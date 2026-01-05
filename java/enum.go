// Package java implement the logic for converting given java source to go source
package java

import (
	"fmt"
	"strings"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type EnumConstant struct {
	name      string
	arguments []gosrc.Expression
}

// TODO: this is mostly ai slop which is good enough for now. But we should be able to do better.

// extractEnumConstant extracts an EnumConstant from a tree-sitter node.
// It handles multiple node structures:
// 1. Nodes with a "name" field (and optional "arguments" field)
// 2. Nodes that are "identifier" directly
// 3. Nodes that contain "identifier" children
// Returns nil if no constant can be extracted.
func extractEnumConstant(ctx *MigrationContext, node *tree_sitter.Node) *EnumConstant {
	// First try: Get name field from node
	constantNameNode := node.ChildByFieldName("name")
	if constantNameNode != nil {
		constantName := constantNameNode.Utf8Text(ctx.JavaSource)
		var args []gosrc.Expression
		argsNode := node.ChildByFieldName("arguments")
		if argsNode != nil {
			args = convertArgumentList(ctx, argsNode)
		}
		return &EnumConstant{
			name:      constantName,
			arguments: args,
		}
	}

	// Second try: If node is identifier, use its text as name
	if node.Kind() == "identifier" {
		constantName := node.Utf8Text(ctx.JavaSource)
		return &EnumConstant{
			name:      constantName,
			arguments: []gosrc.Expression{},
		}
	}

	// Third try: Iterate children looking for identifier node
	var constantName string
	IterateChildren(node, func(child *tree_sitter.Node) {
		if child.Kind() == "identifier" && constantName == "" {
			constantName = child.Utf8Text(ctx.JavaSource)
		}
	})

	if constantName != "" {
		return &EnumConstant{
			name:      constantName,
			arguments: []gosrc.Expression{},
		}
	}

	// No constant could be extracted
	return nil
}

func migrateEnumDeclaration(ctx *MigrationContext, enumNode *tree_sitter.Node) {
	var enumName string
	var modifiers modifiers
	var enumConstants []EnumConstant
	var enumBody *tree_sitter.Node
	var hasFields bool

	IterateChildren(enumNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "identifier":
			enumName = child.Utf8Text(ctx.JavaSource)
		case "enum_constants":
			// Parse enum constants list
			IterateChildren(child, func(constantChild *tree_sitter.Node) {
				if enumConst := extractEnumConstant(ctx, constantChild); enumConst != nil {
					enumConstants = append(enumConstants, *enumConst)
				}
			})
		case "enum_constant":
			// Handle enum constant as direct child (fallback)
			if enumConst := extractEnumConstant(ctx, child); enumConst != nil {
				enumConstants = append(enumConstants, *enumConst)
			}
		case "enum_body":
			enumBody = child
			// Parse constants and check for fields in the body
			IterateChildren(child, func(bodyChild *tree_sitter.Node) {
				switch bodyChild.Kind() {
				case "field_declaration":
					hasFields = true
				case "enum_constant":
					// Constants might be in the body
					if enumConst := extractEnumConstant(ctx, bodyChild); enumConst != nil {
						enumConstants = append(enumConstants, *enumConst)
					}
				case "identifier":
					if len(enumConstants) == 0 {
						// Might be a constant name if we haven't found any constants yet
						constantName := bodyChild.Utf8Text(ctx.JavaSource)
						enumConstants = append(enumConstants, EnumConstant{
							name:      constantName,
							arguments: []gosrc.Expression{},
						})
					}
				}
				// Also check nested nodes for field_declaration (in case of nested structures)
				IterateChildrenWhile(bodyChild, func(nestedChild *tree_sitter.Node) bool {
					if nestedChild.Kind() == "field_declaration" {
						hasFields = true
						return false
					} else {
						return true
					}
				})
			})
		case "class_body":
			// Enum body might be represented as class_body
			if enumBody == nil {
				enumBody = child
				// Parse constants and check for fields in the body
				IterateChildren(child, func(bodyChild *tree_sitter.Node) {
					switch bodyChild.Kind() {
					case "field_declaration":
						hasFields = true
					case "enum_constant":
						constantNameNode := bodyChild.ChildByFieldName("name")
						if constantNameNode != nil {
							constantName := constantNameNode.Utf8Text(ctx.JavaSource)
							var args []gosrc.Expression
							argsNode := bodyChild.ChildByFieldName("arguments")
							if argsNode != nil {
								args = convertArgumentList(ctx, argsNode)
							}
							enumConstants = append(enumConstants, EnumConstant{
								name:      constantName,
								arguments: args,
							})
						}
					}
				})
			}
		case "block":
			// Enum body might be a block
			if enumBody == nil {
				enumBody = child
				// Parse constants and check for fields in the body
				IterateChildren(child, func(bodyChild *tree_sitter.Node) {
					if bodyChild.Kind() == "field_declaration" {
						hasFields = true
					} else if bodyChild.Kind() == "enum_constant" {
						constantNameNode := bodyChild.ChildByFieldName("name")
						if constantNameNode != nil {
							constantName := constantNameNode.Utf8Text(ctx.JavaSource)
							var args []gosrc.Expression
							argsNode := bodyChild.ChildByFieldName("arguments")
							if argsNode != nil {
								args = convertArgumentList(ctx, argsNode)
							}
							enumConstants = append(enumConstants, EnumConstant{
								name:      constantName,
								arguments: args,
							})
						}
					}
				})
			}
		// ignored
		case "enum":
		case "line_comment":
		case "block_comment":
		case ",":
			// Ignore commas in enum constant list
		case ";":
			// Ignore semicolons (separator between constants and body)
		case "{":
			// Opening brace - might contain enum body
		case "}":
			// Closing brace
		default:
			UnhandledChild(ctx, child, "enum_declaration")
		}
	})

	// Enums are public by default in Java (unless explicitly private/protected)
	// If no access modifier is present, default to public
	isPublic := modifiers.isPublic()
	hasAccessModifier := (modifiers&PUBLIC != 0) || (modifiers&PRIVATE != 0) || (modifiers&PROTECTED != 0)
	if !hasAccessModifier {
		isPublic = true
	}
	enumTypeName := gosrc.ToIdentifier(enumName, isPublic)

	// Re-check for fields in enum body if we have one (fields might come after constants)
	if enumBody != nil && !hasFields {
		IterateChildrenWhile(enumBody, func(bodyChild *tree_sitter.Node) bool {
			if bodyChild.Kind() == "field_declaration" {
				hasFields = true
				return false
			} else {
				return true
			}
		})
	}

	// Recalculate enumTypeName with correct public flag if needed
	if !hasAccessModifier {
		enumTypeName = gosrc.ToIdentifier(enumName, isPublic)
	}

	if hasFields {
		// Complex enum: generate struct and var declarations
		convertComplexEnum(ctx, enumTypeName, enumConstants, enumBody, modifiers, isPublic)
	} else {
		// Simple enum: generate int type and const with iota
		convertSimpleEnum(ctx, enumTypeName, enumConstants, enumBody, modifiers, isPublic)
	}
}

func convertSimpleEnum(ctx *MigrationContext, enumTypeName string, enumConstants []EnumConstant, enumBody *tree_sitter.Node, modifiers modifiers, isPublic bool) {
	// Generate type declaration: type EnumName uint
	ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
		Name:     enumTypeName,
		Fields:   []gosrc.StructField{},
		Comments: []string{fmt.Sprintf("type %s uint", enumTypeName)},
		Public:   isPublic,
		Includes: []gosrc.Type{},
	})

	// Generate const block with iota
	if len(enumConstants) > 0 {
		prefixedConstants := make([]string, len(enumConstants))
		for i, constant := range enumConstants {
			prefixedName := enumTypeName + "_" + constant.name
			prefixedConstants[i] = prefixedName
			// Track enum constant for later reference conversion
			ctx.EnumConstants[constant.name] = prefixedName
		}
		ctx.Source.ConstBlocks = append(ctx.Source.ConstBlocks, gosrc.ConstBlock{
			TypeName:  enumTypeName,
			Constants: prefixedConstants,
		})
	}

	// Parse and convert methods from enum body
	if enumBody != nil {
		// Recursively find all method_declaration nodes
		var findMethods func(node *tree_sitter.Node)
		findMethods = func(node *tree_sitter.Node) {
			IterateChildren(node, func(bodyChild *tree_sitter.Node) {
				switch bodyChild.Kind() {
				case "method_declaration":
					// Handle methods similar to class methods
					function, isStatic := convertMethodDeclaration(ctx, bodyChild)
					if isStatic {
						ctx.Source.Functions = append(ctx.Source.Functions, function)
					} else {
						ctx.Source.Methods = append(ctx.Source.Methods, gosrc.Method{
							Function: function,
							Receiver: gosrc.Param{
								Name: gosrc.SelfRef,
								Ty:   gosrc.Type("*" + enumTypeName),
							},
						})
					}
				case "enum_constant":
					// Skip enum constants - already parsed
				case "enum_declaration":
					// Handle nested enums
					migrateEnumDeclaration(ctx, bodyChild)
				default:
					// Recursively search nested structures
					findMethods(bodyChild)
				}
			})
		}
		findMethods(enumBody)
	}
}

func convertComplexEnum(ctx *MigrationContext, enumTypeName string, enumConstants []EnumConstant, enumBody *tree_sitter.Node, modifiers modifiers, isPublic bool) {
	// First, track enum constants so they can be referenced in method bodies
	for _, constant := range enumConstants {
		prefixedName := enumTypeName + "_" + constant.name
		// Track enum constant for later reference conversion
		ctx.EnumConstants[constant.name] = prefixedName
	}

	// Parse fields from enum body
	var fields []gosrc.StructField

	// Recursively find all field_declaration and method_declaration nodes
	var findFieldsAndMethods func(node *tree_sitter.Node)
	findFieldsAndMethods = func(node *tree_sitter.Node) {
		IterateChildren(node, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "field_declaration":
				field, _, _ := convertFieldDeclaration(ctx, child)
				fields = append(fields, field)
			case "method_declaration":
				// Handle methods similar to class methods
				function, isStatic := convertMethodDeclaration(ctx, child)
				if isStatic {
					ctx.Source.Functions = append(ctx.Source.Functions, function)
				} else {
					ctx.Source.Methods = append(ctx.Source.Methods, gosrc.Method{
						Function: function,
						Receiver: gosrc.Param{
							Name: gosrc.SelfRef,
							Ty:   gosrc.Type("*" + enumTypeName),
						},
					})
				}
			case "constructor_declaration":
				// Enum constructors are private and used for initialization
				// We'll handle them in the var declarations
			case "enum_constant":
				// Skip enum constants - already parsed
			case "enum_declaration":
				// Handle nested enums
				migrateEnumDeclaration(ctx, child)
			default:
				// Recursively search nested structures
				findFieldsAndMethods(child)
			}
		})
	}

	if enumBody != nil {
		findFieldsAndMethods(enumBody)
	}

	// Generate struct type
	ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
		Name:     enumTypeName,
		Fields:   fields,
		Comments: []string{},
		Public:   isPublic,
		Includes: []gosrc.Type{},
	})

	// Generate var declarations for each enum constant
	// Parse field names to create struct literal
	fieldNames := make([]string, len(fields))
	for i, field := range fields {
		fieldNames[i] = gosrc.ToIdentifier(field.Name, field.Public)
	}

	for _, constant := range enumConstants {
		prefixedName := enumTypeName + "_" + constant.name
		// Create struct literal with constructor arguments
		var structLiteral gosrc.Expression
		if len(constant.arguments) > 0 && len(constant.arguments) == len(fieldNames) {
			// Create struct literal with field names and values
			sb := strings.Builder{}
			sb.WriteString(enumTypeName)
			sb.WriteString("{")
			for i, arg := range constant.arguments {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fieldNames[i])
				sb.WriteString(": ")
				sb.WriteString(arg.ToSource())
			}
			sb.WriteString("}")
			structLiteral = &gosrc.VarRef{Ref: sb.String()}
		} else {
			// Empty struct or mismatch - use empty struct
			structLiteral = &gosrc.VarRef{Ref: enumTypeName + "{}"}
		}
		ctx.Source.Vars = append(ctx.Source.Vars, gosrc.ModuleVar{
			Name:  prefixedName,
			Ty:    gosrc.Type(enumTypeName),
			Value: structLiteral,
		})
	}
}
