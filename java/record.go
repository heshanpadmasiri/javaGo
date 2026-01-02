package java

import (
	"errors"
	"fmt"
	"strings"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func migrateRecordDeclaration(ctx *MigrationContext, recordNode *tree_sitter.Node) {
	var recordName string
	var modifiers modifiers
	var fields []gosrc.StructField
	var comments []string
	var implementedInterfaces []gosrc.Type

	IterateChilden(recordNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "identifier":
			recordName = child.Utf8Text(ctx.JavaSource)
		case "super_interfaces":
			// Parse implements clause - iterate through children to find type_list
			IterateChilden(child, func(superinterfacesChild *tree_sitter.Node) {
				if superinterfacesChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					IterateChilden(superinterfacesChild, func(typeChild *tree_sitter.Node) {
						ty, ok := TryParseType(ctx, typeChild)
						if ok {
							implementedInterfaces = append(implementedInterfaces, ty)
						}
					})
				}
			})
		case "formal_parameters":
			// Record components are in formal_parameters
			IterateChilden(child, func(paramChild *tree_sitter.Node) {
				switch paramChild.Kind() {
				case "formal_parameter":
					typeNode := paramChild.ChildByFieldName("type")
					if typeNode == nil {
						diagnostics.Fatal(paramChild.ToSexp(), errors.New("formal_parameter missing type field"))
					}
					nameNode := paramChild.ChildByFieldName("name")
					if nameNode == nil {
						diagnostics.Fatal(paramChild.ToSexp(), errors.New("formal_parameter missing name field"))
					}
					ty, ok := TryParseType(ctx, typeNode)
					if !ok {
						diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in formal_parameter"))
					}
					// For record fields, we don't convert arrays to pointers (unlike function parameters)
					// Record fields should be slices directly
					fieldName := nameNode.Utf8Text(ctx.JavaSource)
					fields = append(fields, gosrc.StructField{
						Name:     fieldName,
						Ty:       ty,
						Public:   true, // All record fields must be public
						Comments: []string{},
					})
				// ignored
				case "(":
				case ")":
				case ",":
				case "line_comment":
				case "block_comment":
				default:
					UnhandledChild(ctx, paramChild, "record formal_parameters")
				}
			})
		case "class_body":
			// Handle optional record body with additional methods/fields
			// Build field name mapping: original component name -> struct field name
			fieldNameMap := make(map[string]string)
			for _, field := range fields {
				originalName := field.Name
				structFieldName := gosrc.ToIdentifier(field.Name, true) // Always public for records
				fieldNameMap[originalName] = structFieldName
			}
			// Extract compact constructor before processing class body
			var compactConstructorNode *tree_sitter.Node
			IterateChilden(child, func(bodyChild *tree_sitter.Node) {
				if bodyChild.Kind() == "compact_constructor_declaration" {
					compactConstructorNode = bodyChild
				}
			})
			// Convert compact constructor if present
			if compactConstructorNode != nil {
				structName := gosrc.ToIdentifier(recordName, modifiers.isPublic())
				compactConstructor := convertCompactConstructor(ctx, fields, structName, compactConstructorNode)
				ctx.Source.Functions = append(ctx.Source.Functions, compactConstructor)
			}
			result := convertClassBody(ctx, recordName, child, false)
			// Add any additional fields from the body
			fields = append(fields, result.Fields...)
			// Add methods with the record as receiver, converting field references
			structName := gosrc.ToIdentifier(recordName, modifiers.isPublic())
			for i := range result.Methods {
				method := &result.Methods[i]
				method.Receiver = gosrc.Param{
					Name: gosrc.SelfRef,
					Ty:   gosrc.Type("*" + structName),
				}
				// Convert method body to use struct field names
				method.Body = convertMethodBodyForRecord(ctx, method.Body, fieldNameMap)
				ctx.Source.Methods = append(ctx.Source.Methods, *method)
			}
			// Add any functions (static methods)
			for _, function := range result.Functions {
				ctx.Source.Functions = append(ctx.Source.Functions, function)
			}
			// Note: Nested class_declaration and record_declaration are handled by convertClassBody
		// ignored
		case "record":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "record_declaration")
		}
	})

	// Create the struct with record components as fields
	structName := gosrc.ToIdentifier(recordName, modifiers.isPublic())
	ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
		Name:     structName,
		Fields:   fields,
		Comments: comments,
		Public:   modifiers&PUBLIC != 0,
		Includes: []gosrc.Type{}, // Records don't support extends, only implements
	})

	// Generate type assertions for implemented interfaces
	for _, ifaceType := range implementedInterfaces {
		// Create type assertion: var _ InterfaceName = &StructName{}
		ctx.Source.Vars = append(ctx.Source.Vars, gosrc.ModuleVar{
			Name:  "_",
			Ty:    ifaceType,
			Value: &gosrc.VarRef{Ref: "&" + structName + "{}"},
		})
	}
}

func convertMethodBodyForRecord(ctx *MigrationContext, body []gosrc.Statement, fieldNameMap map[string]string) []gosrc.Statement {
	var converted []gosrc.Statement
	for _, stmt := range body {
		converted = append(converted, convertStatementForRecord(ctx, stmt, fieldNameMap))
	}
	return converted
}

func convertStatementForRecord(ctx *MigrationContext, stmt gosrc.Statement, fieldNameMap map[string]string) gosrc.Statement {
	switch s := stmt.(type) {
	case *gosrc.GoStatement:
		// Replace bare field references with this.FieldName
		// gosrc.GoStatement contains raw source, so we do simple string replacement
		// This is a simplified approach - in production you'd want AST-based replacement
		source := s.Source
		// Sort field names by length (longest first) to avoid partial matches
		type fieldPair struct {
			original string
			mapped   string
		}
		var fields []fieldPair
		for originalName, structFieldName := range fieldNameMap {
			fields = append(fields, fieldPair{original: originalName, mapped: structFieldName})
		}
		// Sort by length descending
		for i := 0; i < len(fields); i++ {
			for j := i + 1; j < len(fields); j++ {
				if len(fields[i].original) < len(fields[j].original) {
					fields[i], fields[j] = fields[j], fields[i]
				}
			}
		}
		// Replace field references, avoiding replacements that are already part of "this.field"
		for _, field := range fields {
			originalName := field.original
			structFieldName := field.mapped
			// Only replace if it's not already part of "this.field"
			// Simple heuristic: replace if not preceded by "this."
			replacement := gosrc.SelfRef + "." + structFieldName
			// Use word boundary-aware replacement
			// Replace standalone occurrences (not part of "this.field")
			beforePattern := gosrc.SelfRef + "." + originalName
			if !strings.Contains(source, beforePattern) {
				// Replace bare field name with this.FieldName
				// Be careful: only replace if it's a standalone identifier
				// Simple approach: replace and then fix if we created "this.this.Field"
				source = strings.ReplaceAll(source, originalName, replacement)
				source = strings.ReplaceAll(source, gosrc.SelfRef+"."+gosrc.SelfRef+".", gosrc.SelfRef+".")
			} else {
				// Already has "this.field", just capitalize the field name
				source = strings.ReplaceAll(source, beforePattern, gosrc.SelfRef+"."+structFieldName)
			}
		}
		return &gosrc.GoStatement{Source: source}
	case *gosrc.ReturnStatement:
		if s.Value != nil {
			return &gosrc.ReturnStatement{Value: convertExpressionForRecord(ctx, s.Value, fieldNameMap)}
		}
		return s
	case *gosrc.AssignStatement:
		refExpr := convertExpressionForRecord(ctx, &gosrc.VarRef{Ref: s.Ref.Ref}, fieldNameMap)
		var ref gosrc.VarRef
		if varRef, ok := refExpr.(*gosrc.VarRef); ok {
			ref = *varRef
		} else {
			// Fallback: use original ref
			ref = s.Ref
		}
		return &gosrc.AssignStatement{
			Ref:   ref,
			Value: convertExpressionForRecord(ctx, s.Value, fieldNameMap),
		}
	case *gosrc.IfStatement:
		return &gosrc.IfStatement{
			Condition: convertExpressionForRecord(ctx, s.Condition, fieldNameMap),
			Body:      convertMethodBodyForRecord(ctx, s.Body, fieldNameMap),
			ElseIf:    convertElseIfsForRecord(ctx, s.ElseIf, fieldNameMap),
			ElseStmts: convertMethodBodyForRecord(ctx, s.ElseStmts, fieldNameMap),
		}
	default:
		return stmt
	}
}

func convertElseIfsForRecord(ctx *MigrationContext, elseIfs []gosrc.IfStatement, fieldNameMap map[string]string) []gosrc.IfStatement {
	var converted []gosrc.IfStatement
	for _, elseIf := range elseIfs {
		converted = append(converted, gosrc.IfStatement{
			Condition: convertExpressionForRecord(ctx, elseIf.Condition, fieldNameMap),
			Body:      convertMethodBodyForRecord(ctx, elseIf.Body, fieldNameMap),
			ElseIf:    convertElseIfsForRecord(ctx, elseIf.ElseIf, fieldNameMap),
			ElseStmts: convertMethodBodyForRecord(ctx, elseIf.ElseStmts, fieldNameMap),
		})
	}
	return converted
}

func convertExpressionForRecord(ctx *MigrationContext, expr gosrc.Expression, fieldNameMap map[string]string) gosrc.Expression {
	switch e := expr.(type) {
	case *gosrc.VarRef:
		ref := e.Ref
		// Check if this is a bare field reference that needs conversion
		if structFieldName, ok := fieldNameMap[ref]; ok {
			// Convert bare field reference to this.FieldName
			return &gosrc.VarRef{Ref: gosrc.SelfRef + "." + structFieldName}
		}
		// If it's already this.field, check if the field name needs capitalization
		if strings.HasPrefix(ref, gosrc.SelfRef+".") {
			fieldName := strings.TrimPrefix(ref, gosrc.SelfRef+".")
			if structFieldName, ok := fieldNameMap[fieldName]; ok {
				return &gosrc.VarRef{Ref: gosrc.SelfRef + "." + structFieldName}
			}
		}
		return e
	case *gosrc.BinaryExpression:
		return &gosrc.BinaryExpression{
			Left:     convertExpressionForRecord(ctx, e.Left, fieldNameMap),
			Operator: e.Operator,
			Right:    convertExpressionForRecord(ctx, e.Right, fieldNameMap),
		}
	case *gosrc.UnaryExpression:
		return &gosrc.UnaryExpression{
			Operator: e.Operator,
			Operand:  convertExpressionForRecord(ctx, e.Operand, fieldNameMap),
		}
	case *gosrc.CallExpression:
		var convertedArgs []gosrc.Expression
		for _, arg := range e.Args {
			convertedArgs = append(convertedArgs, convertExpressionForRecord(ctx, arg, fieldNameMap))
		}
		return &gosrc.CallExpression{
			Function: e.Function,
			Args:     convertedArgs,
		}
	default:
		return expr
	}
}

// convertRecordComponentsToParams converts record components (gosrc.StructField) to function parameters (gosrc.Param)
// Applies the same array-to-pointer conversion as convertFormalParameters for consistency
func convertRecordComponentsToParams(components []gosrc.StructField) []gosrc.Param {
	var params []gosrc.Param
	for _, component := range components {
		ty := component.Ty
		// Convert array types to pointer-to-array for parameters (same as convertFormalParameters)
		if IsArrayOrSliceType(ty) {
			ty = gosrc.Type("*" + ty)
		}
		params = append(params, gosrc.Param{
			Name: component.Name,
			Ty:   ty,
		})
	}
	return params
}

func convertCompactConstructor(ctx *MigrationContext, recordComponents []gosrc.StructField, structName string, compactConstructorNode *tree_sitter.Node) gosrc.Function {
	var modifiers modifiers
	var body []gosrc.Statement
	// Convert record components to parameters
	params := convertRecordComponentsToParams(recordComponents)
	// Initialize struct
	body = append(body, &gosrc.GoStatement{Source: fmt.Sprintf("%s := %s{};", gosrc.SelfRef, structName)})
	// Process compact constructor body
	IterateChilden(compactConstructorNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "block":
			// Compact constructor body is a block
			body = append(body, convertCompactConstructorBody(ctx, recordComponents, structName, child)...)
		// ignored
		case "identifier":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "compact_constructor_declaration")
		}
	})
	// After body execution, assign parameters to struct fields
	for _, component := range recordComponents {
		structFieldName := gosrc.ToIdentifier(component.Name, true) // Always public for records
		paramName := component.Name
		body = append(body, &gosrc.AssignStatement{
			Ref:   gosrc.VarRef{Ref: gosrc.SelfRef + "." + structFieldName},
			Value: &gosrc.VarRef{Ref: paramName},
		})
	}
	body = append(body, &gosrc.ReturnStatement{Value: &gosrc.VarRef{Ref: gosrc.SelfRef}})
	// Generate function Name: newStructNameFromParam1Param2...
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(gosrc.ToIdentifier("new", modifiers.isPublic()))
	nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(structName))
	nameBuilder.WriteString("From")
	for _, param := range params {
		nameBuilder.WriteString(gosrc.CapitalizeFirstLetter(param.Name))
	}
	name := nameBuilder.String()
	retTy := gosrc.Type(structName)
	return gosrc.Function{
		Name:       name,
		Params:     params,
		ReturnType: &retTy,
		Body:       body,
		Public:     modifiers&PUBLIC != 0,
	}
}

func convertCompactConstructorBody(ctx *MigrationContext, recordComponents []gosrc.StructField, structName string, bodyNode *tree_sitter.Node) []gosrc.Statement {
	var body []gosrc.Statement
	// Process statements in the compact constructor body
	// Unlike regular constructors, compact constructors cannot have:
	// - Explicit constructor invocations (this(...) or super(...))
	// - Explicit assignments to component fields (they're implicit)
	// The body can contain validation/normalization logic that modifies parameters
	IterateChilden(bodyNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		// Handle all statement types by delegating to convertStatement
		case "if_statement", "expression_statement", "local_variable_declaration",
			"return_statement", "break_statement", "continue_statement",
			"while_statement", "for_statement", "enhanced_for_statement",
			"throw_statement", "try_statement", "assert_statement",
			"switch_expression", "yield_statement":
			statements := convertStatement(ctx, child)
			if statements != nil {
				body = append(body, statements...)
			}
		// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "compact_constructor_body")
		}
	})
	return body
}
