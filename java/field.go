package java

import (
	"errors"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func convertParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []gosrc.Param {
	var params []gosrc.Param
	IterateChilden(paramsNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "formal_parameters":
			params = append(params, convertFormalParameters(ctx, child)...)
		default:
			UnhandledChild(ctx, child, "parameters")
		}
	})
	return params
}

func convertFormalParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []gosrc.Param {
	var params []gosrc.Param
	IterateChilden(paramsNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "formal_parameter":
			typeNode := child.ChildByFieldName("type")
			if typeNode == nil {
				diagnostics.Fatal(child.ToSexp(), errors.New("formal_parameter missing type field"))
			}
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				diagnostics.Fatal(child.ToSexp(), errors.New("formal_parameter missing name field"))
			}
			ty, ok := TryParseType(ctx, typeNode)
			if !ok {
				diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in formal_parameter"))
			}
			// Convert array types to pointer-to-array for parameters
			if IsArrayOrSliceType(ty) {
				ty = gosrc.Type("*" + ty)
			}
			params = append(params, gosrc.Param{
				Name: nameNode.Utf8Text(ctx.JavaSource),
				Ty: ty,
			})
		case "spread_parameter":
			var ty gosrc.Type
			var name string
			IterateChilden(child, func(spreadChild *tree_sitter.Node) {
				switch spreadChild.Kind() {
				case "variable_declarator":
					nameNode := spreadChild.ChildByFieldName("name")
					if nameNode == nil {
						diagnostics.Fatal(spreadChild.ToSexp(), errors.New("spread child missing name field"))
					}
					name = nameNode.Utf8Text(ctx.JavaSource)
				case "...":
					return
				default:
					goTy, ok := TryParseType(ctx, spreadChild)
					ty = goTy
					if ok {
						return
					}
				}
			})
			params = append(params, gosrc.Param{
				Name: name,
				Ty: "..." + ty,
			})
		// ignored
		case "(":
		case ")":
		case ",":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "formal_parameters")
		}
	})
	return params
}

func convertFieldDeclaration(ctx *MigrationContext, fieldNode *tree_sitter.Node) (gosrc.StructField, gosrc.Expression, modifiers) {
	var mods modifiers
	var ty gosrc.Type
	var name string
	var comments []string
	var initExpr gosrc.Expression
	IterateChilden(fieldNode, func(child *tree_sitter.Node) {
		t, ok := TryParseType(ctx, child)
		if ok {
			ty = t
			return
		}
		switch child.Kind() {
		case "modifiers":
			mods = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "variable_declarator":
			result := convertVariableDecl(ctx, child)
			name = result.name
			initExpr = result.value

			// Handle shorthand array initializer: { 1, 2, 3 }
			// Check if the value node was array_initializer
			valueNode := child.ChildByFieldName("value")
			if valueNode != nil && valueNode.Kind() == "array_initializer" {
				// convertVariableDecl couldn't handle this (no type info)
				// Parse it here with type context
				elements := convertArrayInitializer(ctx, valueNode)
				initExpr = &gosrc.ArrayLiteral{ElementType: ty, Elements: elements}
			}
		// ignored
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "field_declaration")
		}
	})
	return gosrc.StructField{
		Name: name,
		Ty: ty,
		Public:   mods&PUBLIC != 0,
		Comments: comments,
	}, initExpr, mods
}

type variableDeclResult struct {
	name  string
	value gosrc.Expression
}

func convertVariableDecl(ctx *MigrationContext, declNode *tree_sitter.Node) variableDeclResult {
	var name string
	nameNode := declNode.ChildByFieldName("name")
	if nameNode != nil {
		name = nameNode.Utf8Text(ctx.JavaSource)
	} else {
		diagnostics.Fatal(declNode.ToSexp(), errors.New("variable_declarator missing name field"))
	}
	valueNode := declNode.ChildByFieldName("value")
	if valueNode != nil {
		// Skip array_initializer - parent will handle with type context
		if valueNode.Kind() == "array_initializer" {
			return variableDeclResult{
				name: name,
				value: nil, // Signal to parent to handle
			}
		}

		value, init := convertExpression(ctx, valueNode)
		Assert("unexpected statements in variable declaration", len(init) == 0)
		return variableDeclResult{
			name: name,
			value: value,
		}
	}
	return variableDeclResult{
		name: name,
	}
}

