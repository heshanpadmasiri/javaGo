package java

import (
	"fmt"
	"sort"
	"strings"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// classConversionResult holds the result of converting a class body
type classConversionResult struct {
	Fields    []gosrc.StructField
	Comments  []string
	Functions []gosrc.Function
	Methods   []gosrc.Method
}

func migrateClassDeclaration(ctx *MigrationContext, classNode *tree_sitter.Node) {
	var className string
	var modifiers modifiers
	var includes []gosrc.Type
	var implementedInterfaces []gosrc.Type
	isAbstract := false
	IterateChilden(classNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
			isAbstract = modifiers&ABSTRACT != 0
		case "identifier":
			className = child.Utf8Text(ctx.JavaSource)
		case "superclass":
			ty, ok := TryParseType(ctx, child.Child(1))
			if ok {
				includes = append(includes, ty)
			} else {
				UnhandledChild(ctx, child, "superclass")
			}
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
		case "class_body":
			if isAbstract {
				ctx.AbstractClasses[className] = true
				convertAbstractClass(ctx, className, modifiers, includes, child)
			} else {
				// Check if this class extends an abstract class
				var embeddedTypes []gosrc.Type
				extendsAbstract := false
				for _, include := range includes {
					baseName := string(include)
					if ctx.AbstractClasses[baseName] {
						// Embed FooBase and FooMethods
						embeddedTypes = append(embeddedTypes, gosrc.Type(gosrc.CapitalizeFirstLetter(baseName)+"Base"))
						embeddedTypes = append(embeddedTypes, gosrc.Type(gosrc.CapitalizeFirstLetter(baseName)+"Methods"))
						extendsAbstract = true
					} else {
						embeddedTypes = append(embeddedTypes, include)
					}
				}
				// Use capitalized name if extending abstract class, otherwise use gosrc.ToIdentifier
				var structName string
				if extendsAbstract {
					structName = gosrc.CapitalizeFirstLetter(className)
				} else {
					structName = gosrc.ToIdentifier(className, modifiers.isPublic())
				}
				isPublicClass := modifiers&PUBLIC != 0
				result := convertClassBody(ctx, structName, child, false, isPublicClass)
				ctx.Source.Functions = append(ctx.Source.Functions, result.Functions...)
				for i := range result.Methods {
					method := &result.Methods[i]
					// Capitalize method names if extending abstract class
					if extendsAbstract {
						method.Name = gosrc.CapitalizeFirstLetter(method.Name)
						method.Public = true
						// Update receiver type to use capitalized struct name
						method.Receiver.Ty = gosrc.Type("*" + structName)
						// Use single lowercase letter for receiver name (Go convention: first letter of type)
						receiverName := strings.ToLower(string(structName[0]))
						method.Receiver.Name = receiverName
					}
					ctx.Source.Methods = append(ctx.Source.Methods, *method)
				}
				ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
					Name:     structName,
					Fields:   result.Fields,
					Comments: result.Comments,
					Public:   extendsAbstract || (modifiers&PUBLIC != 0),
					Includes: embeddedTypes,
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
		// ignored
		case "class":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "class_declaration")
		}
	})
}

func convertAbstractClass(ctx *MigrationContext, className string, modifiers modifiers, includes []gosrc.Type, classBody *tree_sitter.Node) {
	// Extract fields and methods
	var fields []gosrc.StructField
	var abstractMethods []gosrc.Function
	var defaultMethods []gosrc.Function
	var comments []string
	fieldInitValues := map[string]gosrc.Expression{}

	IterateChilden(classBody, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "class_declaration":
			migrateClassDeclaration(ctx, child)
		case "record_declaration":
			migrateRecordDeclaration(ctx, child)
		case "enum_declaration":
			migrateEnumDeclaration(ctx, child)
		case "field_declaration":
			field, initExpr, mods := convertFieldDeclaration(ctx, child)
			if mods&STATIC != 0 {
				ctx.Source.Vars = append(ctx.Source.Vars, gosrc.ModuleVar{
					Name:  field.Name,
					Ty:    field.Ty,
					Value: initExpr,
				})
			} else {
				if initExpr != nil {
					Assert("mutiple initializations for field"+field.Name, fieldInitValues[field.Name] == nil)
					fieldInitValues[field.Name] = initExpr
				}
				fields = append(fields, field)
			}
		case "method_declaration":
			function, isStatic, isAbstract := convertMethodDeclarationWithAbstract(ctx, child)
			if !isStatic {
				if isAbstract {
					abstractMethods = append(abstractMethods, function)
				} else {
					defaultMethods = append(defaultMethods, function)
				}
			} else {
				ctx.Source.Functions = append(ctx.Source.Functions, function)
			}
		case "constructor_declaration":
			// Abstract classes can have constructors, but we'll skip them for now
		// ignored
		case "{":
		case "}":
		case "block_comment":
		case "line_comment":
		default:
			UnhandledChild(ctx, child, "class_body")
		}
	})

	// Generate FooData interface
	dataInterfaceName := gosrc.CapitalizeFirstLetter(className) + "Data"
	var dataMethods []gosrc.InterfaceMethod
	for _, field := range fields {
		fieldName := gosrc.CapitalizeFirstLetter(field.Name)
		getterName := "Get" + fieldName
		setterName := "Set" + fieldName
		dataMethods = append(dataMethods, gosrc.InterfaceMethod{
			Name:       getterName,
			Params:     []gosrc.Param{},
			ReturnType: &field.Ty,
			Public:     true,
		})
		dataMethods = append(dataMethods, gosrc.InterfaceMethod{
			Name:       setterName,
			Params:     []gosrc.Param{{Name: gosrc.ToIdentifier(field.Name, false), Ty: field.Ty}},
			ReturnType: nil,
			Public:     true,
		})
	}
	ctx.Source.Interfaces = append(ctx.Source.Interfaces, gosrc.Interface{
		Name:     dataInterfaceName,
		Embeds:   []gosrc.Type{},
		Methods:  dataMethods,
		Public:   true, // Interfaces for abstract classes are always public
		Comments: comments,
	})

	// Generate FooBase struct
	baseStructName := gosrc.CapitalizeFirstLetter(className) + "Base"
	// Capitalize field names in base struct
	var capitalizedFields []gosrc.StructField
	for _, field := range fields {
		capitalizedFields = append(capitalizedFields, gosrc.StructField{
			Name:     gosrc.CapitalizeFirstLetter(field.Name),
			Ty:       field.Ty,
			Public:   true,
			Comments: field.Comments,
		})
	}
	ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
		Name:     baseStructName,
		Includes: []gosrc.Type{},
		Fields:   capitalizedFields,
		Public:   true, // Base structs for abstract classes are always public
		Comments: comments,
	})

	// Generate getter/setter methods for FooBase
	for _, field := range fields {
		fieldName := gosrc.CapitalizeFirstLetter(field.Name)
		getterName := "Get" + fieldName
		setterName := "Set" + fieldName
		ctx.Source.Methods = append(ctx.Source.Methods, gosrc.Method{
			Function: gosrc.Function{
				Name:       getterName,
				Params:     []gosrc.Param{},
				ReturnType: &field.Ty,
				Body: []gosrc.Statement{
					&gosrc.ReturnStatement{Value: &gosrc.VarRef{Ref: "b." + gosrc.ToIdentifier(field.Name, true)}},
				},
				Public: true,
			},
			Receiver: gosrc.Param{
				Name: "b",
				Ty:   gosrc.Type("*" + baseStructName),
			},
		})
		ctx.Source.Methods = append(ctx.Source.Methods, gosrc.Method{
			Function: gosrc.Function{
				Name:       setterName,
				Params:     []gosrc.Param{{Name: gosrc.ToIdentifier(field.Name, false), Ty: field.Ty}},
				ReturnType: nil,
				Body: []gosrc.Statement{
					&gosrc.AssignStatement{
						Ref:   gosrc.VarRef{Ref: "b." + gosrc.ToIdentifier(field.Name, true)},
						Value: &gosrc.VarRef{Ref: gosrc.ToIdentifier(field.Name, false)},
					},
				},
				Public: true,
			},
			Receiver: gosrc.Param{
				Name: "b",
				Ty:   gosrc.Type("*" + baseStructName),
			},
		})
	}

	// Generate FooMethods struct
	methodsStructName := gosrc.CapitalizeFirstLetter(className) + "Methods"
	ctx.Source.Structs = append(ctx.Source.Structs, gosrc.Struct{
		Name:     methodsStructName,
		Includes: []gosrc.Type{},
		Fields: []gosrc.StructField{
			{
				Name:   "Self",
				Ty:     gosrc.Type(gosrc.CapitalizeFirstLetter(className)),
				Public: true,
			},
		},
		Public:   true, // Methods structs for abstract classes are always public
		Comments: comments,
	})

	// Convert default methods to use m.Self
	for _, method := range defaultMethods {
		// Convert method body to use m.Self
		convertedBody := convertMethodBodyForDefaultMethod(ctx, method.Body, className, fields)
		ctx.Source.Methods = append(ctx.Source.Methods, gosrc.Method{
			Function: gosrc.Function{
				Name:       gosrc.CapitalizeFirstLetter(method.Name),
				Params:     method.Params,
				ReturnType: method.ReturnType,
				Body:       convertedBody,
				Comments:   method.Comments,
				Public:     true, // Methods in FooMethods are always public
			},
			Receiver: gosrc.Param{
				Name: "m",
				Ty:   gosrc.Type("*" + methodsStructName),
			},
		})
	}

	// Generate Foo interface
	var interfaceMethods []gosrc.InterfaceMethod
	// Add abstract method signatures - always capitalize for abstract class interfaces
	for _, method := range abstractMethods {
		interfaceMethods = append(interfaceMethods, gosrc.InterfaceMethod{
			Name:       gosrc.CapitalizeFirstLetter(method.Name),
			Params:     method.Params,
			ReturnType: method.ReturnType,
			Public:     true,
		})
	}
	// Add default method signatures - always capitalize for abstract class interfaces
	for _, method := range defaultMethods {
		interfaceMethods = append(interfaceMethods, gosrc.InterfaceMethod{
			Name:       gosrc.CapitalizeFirstLetter(method.Name),
			Params:     method.Params,
			ReturnType: method.ReturnType,
			Public:     true,
		})
	}
	ctx.Source.Interfaces = append(ctx.Source.Interfaces, gosrc.Interface{
		Name:     gosrc.CapitalizeFirstLetter(className),
		Embeds:   []gosrc.Type{gosrc.Type(dataInterfaceName)},
		Methods:  interfaceMethods,
		Public:   true, // Main interface for abstract classes is always public
		Comments: comments,
	})
}

func convertMethodBodyForDefaultMethod(ctx *MigrationContext, body []gosrc.Statement, className string, fields []gosrc.StructField) []gosrc.Statement {
	var converted []gosrc.Statement
	oldInDefaultMethod := ctx.InDefaultMethod
	oldDefaultMethodSelf := ctx.DefaultMethodSelf
	ctx.InDefaultMethod = true
	ctx.DefaultMethodSelf = "m.Self"
	defer func() {
		ctx.InDefaultMethod = oldInDefaultMethod
		ctx.DefaultMethodSelf = oldDefaultMethodSelf
	}()
	// Build map of field names for quick lookup
	fieldMap := make(map[string]bool)
	for _, field := range fields {
		fieldMap[field.Name] = true
	}
	for _, stmt := range body {
		converted = append(converted, convertStatementForDefaultMethod(ctx, stmt, className, fieldMap))
	}
	return converted
}

func convertStatementForDefaultMethod(ctx *MigrationContext, stmt gosrc.Statement, className string, fieldMap map[string]bool) gosrc.Statement {
	switch s := stmt.(type) {
	case *gosrc.GoStatement:
		// Replace this.field with m.Self.GetField() and this.method() with m.Self.gosrc.Method()
		source := s.Source
		// Simple string replacement for common patterns
		// This is a simplified version - in production you'd want a more sophisticated AST-based approach
		source = strings.ReplaceAll(source, "this.", ctx.DefaultMethodSelf+".")
		return &gosrc.GoStatement{Source: source}
	case *gosrc.ReturnStatement:
		if s.Value != nil {
			return &gosrc.ReturnStatement{Value: convertExpressionForDefaultMethod(ctx, s.Value, className, fieldMap)}
		}
		return s
	case *gosrc.AssignStatement:
		// Convert field assignments: this.field = value -> m.Self.SetField(value)
		refStr := s.Ref.ToSource()
		if strings.HasPrefix(refStr, "this.") {
			// For now, keep as assignment - we'll need more sophisticated handling
			return &gosrc.AssignStatement{
				Ref:   gosrc.VarRef{Ref: strings.ReplaceAll(refStr, "this.", ctx.DefaultMethodSelf+".")},
				Value: convertExpressionForDefaultMethod(ctx, s.Value, className, fieldMap),
			}
		}
		return &gosrc.AssignStatement{
			Ref:   s.Ref,
			Value: convertExpressionForDefaultMethod(ctx, s.Value, className, fieldMap),
		}
	case *gosrc.IfStatement:
		return &gosrc.IfStatement{
			Condition: convertExpressionForDefaultMethod(ctx, s.Condition, className, fieldMap),
			Body:      convertStatementsForDefaultMethod(ctx, s.Body, className, fieldMap),
			ElseIf:    convertElseIfsForDefaultMethod(ctx, s.ElseIf, className, fieldMap),
			ElseStmts: convertStatementsForDefaultMethod(ctx, s.ElseStmts, className, fieldMap),
		}
	case *gosrc.ForStatement:
		var initStmt gosrc.Statement
		if s.Init != nil {
			initStmt = convertStatementForDefaultMethod(ctx, s.Init, className, fieldMap)
		}
		var postStmt gosrc.Statement
		if s.Post != nil {
			postStmt = convertStatementForDefaultMethod(ctx, s.Post, className, fieldMap)
		}
		return &gosrc.ForStatement{
			Init:      initStmt,
			Condition: convertExpressionForDefaultMethod(ctx, s.Condition, className, fieldMap),
			Post:      postStmt,
			Body:      convertStatementsForDefaultMethod(ctx, s.Body, className, fieldMap),
		}
	case *gosrc.CallStatement:
		return &gosrc.CallStatement{
			Exp: convertExpressionForDefaultMethod(ctx, s.Exp, className, fieldMap),
		}
	case *gosrc.VarDeclaration:
		if s.Value != nil {
			return &gosrc.VarDeclaration{
				Name:  s.Name,
				Ty:    s.Ty,
				Value: convertExpressionForDefaultMethod(ctx, s.Value, className, fieldMap),
			}
		}
		return s
	default:
		// For other statement types, try to convert recursively if possible
		return stmt
	}
}

func convertStatementsForDefaultMethod(ctx *MigrationContext, stmts []gosrc.Statement, className string, fieldMap map[string]bool) []gosrc.Statement {
	var converted []gosrc.Statement
	for _, stmt := range stmts {
		converted = append(converted, convertStatementForDefaultMethod(ctx, stmt, className, fieldMap))
	}
	return converted
}

func convertElseIfsForDefaultMethod(ctx *MigrationContext, elseIfs []gosrc.IfStatement, className string, fieldMap map[string]bool) []gosrc.IfStatement {
	var converted []gosrc.IfStatement
	for _, elseIf := range elseIfs {
		converted = append(converted, gosrc.IfStatement{
			Condition: convertExpressionForDefaultMethod(ctx, elseIf.Condition, className, fieldMap),
			Body:      convertStatementsForDefaultMethod(ctx, elseIf.Body, className, fieldMap),
			ElseIf:    convertElseIfsForDefaultMethod(ctx, elseIf.ElseIf, className, fieldMap),
			ElseStmts: convertStatementsForDefaultMethod(ctx, elseIf.ElseStmts, className, fieldMap),
		})
	}
	return converted
}

func convertExpressionForDefaultMethod(ctx *MigrationContext, expr gosrc.Expression, className string, fieldMap map[string]bool) gosrc.Expression {
	switch e := expr.(type) {
	case *gosrc.VarRef:
		ref := e.Ref

		fieldName, shouldConvertToGetter := strings.CutPrefix(ref, "this.")
		if shouldConvertToGetter {
			capitalized := gosrc.CapitalizeFirstLetter(fieldName)
			return &gosrc.VarRef{Ref: ctx.DefaultMethodSelf + ".Get" + capitalized + "()"}
		}
		// Check if this is a bare field reference
		if fieldMap[ref] {
			// Convert bare field reference to getter: field -> m.Self.GetField()
			capitalized := gosrc.CapitalizeFirstLetter(ref)
			return &gosrc.VarRef{Ref: ctx.DefaultMethodSelf + ".Get" + capitalized + "()"}
		}
		ref = strings.ReplaceAll(ref, "this.", ctx.DefaultMethodSelf+".")
		return &gosrc.VarRef{Ref: ref}
	case *gosrc.CallExpression:
		funcName := e.Function
		funcName, isSelfMethodRef := strings.CutPrefix(funcName, "this.")
		if isSelfMethodRef {
			funcName = ctx.DefaultMethodSelf + "." + gosrc.CapitalizeFirstLetter(funcName)
		} else if funcName == "this" {
			funcName = ctx.DefaultMethodSelf
		} else if !strings.Contains(funcName, ".") && !fieldMap[funcName] {
			// Bare method call (not a field) - assume it's a method on self
			funcName = ctx.DefaultMethodSelf + "." + gosrc.CapitalizeFirstLetter(funcName)
		}
		var convertedArgs []gosrc.Expression
		for _, arg := range e.Args {
			convertedArgs = append(convertedArgs, convertExpressionForDefaultMethod(ctx, arg, className, fieldMap))
		}
		return &gosrc.CallExpression{
			Function: funcName,
			Args:     convertedArgs,
		}
	case *gosrc.BinaryExpression:
		return &gosrc.BinaryExpression{
			Left:     convertExpressionForDefaultMethod(ctx, e.Left, className, fieldMap),
			Operator: e.Operator,
			Right:    convertExpressionForDefaultMethod(ctx, e.Right, className, fieldMap),
		}
	case *gosrc.UnaryExpression:
		return &gosrc.UnaryExpression{
			Operator: e.Operator,
			Operand:  convertExpressionForDefaultMethod(ctx, e.Operand, className, fieldMap),
		}
	case *gosrc.GoExpression:
		source := e.Source
		// Replace this.method() with m.Self.gosrc.Method() (capitalized)
		// Pattern: this.methodName( -> m.Self.MethodName(
		source = strings.ReplaceAll(source, "this.", ctx.DefaultMethodSelf+".")
		// Capitalize method names after m.Self.
		if strings.Contains(source, ctx.DefaultMethodSelf+".") {
			// Find method calls like m.Self.method( and capitalize method name
			parts := strings.Split(source, ctx.DefaultMethodSelf+".")
			if len(parts) > 1 {
				for i := 1; i < len(parts); i++ {
					// Find the method name (up to the opening parenthesis or end)
					methodPart := parts[i]
					methodEnd := strings.IndexAny(methodPart, "(")
					if methodEnd > 0 {
						methodName := methodPart[:methodEnd]
						capitalized := gosrc.CapitalizeFirstLetter(methodName)
						parts[i] = capitalized + methodPart[methodEnd:]
					} else {
						parts[i] = gosrc.CapitalizeFirstLetter(methodPart)
					}
				}
				source = strings.Join(parts, ctx.DefaultMethodSelf+".")
			}
		}
		return &gosrc.GoExpression{Source: source}
	default:
		return expr
	}
}

func convertClassBody(ctx *MigrationContext, structName string, classBody *tree_sitter.Node, isAbstract bool, isPublicClass bool) classConversionResult {
	var result classConversionResult
	fieldInitValues := map[string]gosrc.Expression{}
	hasConstructor := false
	IterateChilden(classBody, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "class_declaration":
			migrateClassDeclaration(ctx, child)
		case "record_declaration":
			migrateRecordDeclaration(ctx, child)
		case "enum_declaration":
			migrateEnumDeclaration(ctx, child)
		case "field_declaration":
			field, initExpr, mods := convertFieldDeclaration(ctx, child)
			// If field is static final, add as module-level var
			if mods&STATIC != 0 {
				ctx.Source.Vars = append(ctx.Source.Vars, gosrc.ModuleVar{
					Name:  field.Name,
					Ty:    field.Ty,
					Value: initExpr,
				})
			} else {
				// Regular field
				if initExpr != nil {
					Assert("mutiple initializations for field"+field.Name, fieldInitValues[field.Name] == nil)
					fieldInitValues[field.Name] = initExpr
				}
				result.Fields = append(result.Fields, field)
			}
		case "constructor_declaration":
			result.Functions = append(result.Functions, convertConstructor(ctx, &fieldInitValues, structName, child, isPublicClass))
			hasConstructor = true
		case "compact_constructor_declaration":
			// Compact constructors are handled in migrateRecordDeclaration, skip here
		case "method_declaration":
			function, isStatic := convertMethodDeclaration(ctx, child)
			if isStatic {
				result.Functions = append(result.Functions, function)
			} else {
				result.Methods = append(result.Methods, gosrc.Method{
					Function: function,
					Receiver: gosrc.Param{
						Name: gosrc.SelfRef,
						Ty:   gosrc.Type("*" + structName),
					},
				})
			}
		// ignored
		case "{":
		case "}":
		case "block_comment":
		case "line_comment":
		default:
			UnhandledChild(ctx, child, "class_body")
		}
	})

	// Generate default no-arg constructor if none exists and class is not abstract
	if !hasConstructor && !isAbstract {
		result.Functions = append(result.Functions, convertConstructor(ctx, &fieldInitValues, structName, nil, isPublicClass))
	}

	return result
}

func convertMethodDeclaration(ctx *MigrationContext, methodNode *tree_sitter.Node) (gosrc.Function, bool) {
	fn, isStatic, _ := convertMethodDeclarationWithAbstract(ctx, methodNode)
	return fn, isStatic
}

type methodMetadata struct {
	name       string
	params     []gosrc.Param
	returnTy   *gosrc.Type
	isPublic   bool
	isStatic   bool
	isAbstract bool
}

// getMethodMetadata retrieves cached method metadata.
// Panics if metadata is not in cache (programming error).
func getMethodMetadata(ctx *MigrationContext, methodNode *tree_sitter.Node) methodMetadata {
	nodeId := methodNode.Id()
	metadata, exists := ctx.MethodMetadataCache[nodeId]
	if !exists {
		panic(fmt.Sprintf("Method metadata not found in cache for node ID %d. This is a programming error - analyzeNode should have been called first.", nodeId))
	}
	return metadata
}

func parseMethodSignature(ctx *MigrationContext, methodNode *tree_sitter.Node) methodMetadata {
	var modifiers modifiers
	var params []gosrc.Param
	var name string
	var returnType *gosrc.Type
	var hasThrows bool
	IterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := TryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "identifier":
			name = child.Utf8Text(ctx.JavaSource)
		case "void_type":
			returnType = nil
		case "throws":
			hasThrows = true
		// ignored
		case "block":
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "method_declaration")
		}
	})

	// Modify return type if method throws exceptions
	if hasThrows {
		if returnType == nil {
			// void method with exception -> error
			errorType := gosrc.Type("error")
			returnType = &errorType
		} else {
			// non-void method with exception -> (T, error)
			tupleType := gosrc.Type("(" + returnType.ToSource() + ", error)")
			returnType = &tupleType
		}
	}

	isAbstract := modifiers&ABSTRACT != 0
	isStatic := modifiers&STATIC != 0

	return methodMetadata{
		name:       name,
		params:     params,
		returnTy:   returnType,
		isPublic:   modifiers.isPublic(),
		isStatic:   isStatic,
		isAbstract: isAbstract,
	}
}

func convertMethodDeclarationWithAbstract(ctx *MigrationContext, methodNode *tree_sitter.Node) (gosrc.Function, bool, bool) {
	methodMetadata := getMethodMetadata(ctx, methodNode)
	params := methodMetadata.params
	name := methodMetadata.name
	returnType := methodMetadata.returnTy
	isAbstract := methodMetadata.isAbstract
	isStatic := methodMetadata.isStatic
	isPublic := methodMetadata.isPublic

	var body []gosrc.Statement
	blockNode := methodNode.ChildByFieldName("body")
	if blockNode != nil {
		body = convertStatementBlock(ctx, blockNode)
	}

	// If method is abstract and has no body, add panic statement (for non-abstract class methods)
	if isAbstract && len(body) == 0 {
		body = append(body, &gosrc.GoStatement{Source: "panic(\"implemented in concrete class\")"})
	}
	return gosrc.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Public:     isPublic,
	}, isStatic, isAbstract
}

func convertConstructor(ctx *MigrationContext, fieldInitValues *map[string]gosrc.Expression, structName string, constructorNode *tree_sitter.Node, isPublicClass bool) gosrc.Function {
	var modifiers modifiers
	var params []gosrc.Param
	var body []gosrc.Statement
	body = append(body, &gosrc.GoStatement{Source: fmt.Sprintf("%s := %s{};", gosrc.SelfRef, structName)})
	IterateChilden(constructorNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = ParseModifiers(child.Utf8Text(ctx.JavaSource))
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "constructor_body":
			body = append(body, convertConstructorBody(ctx, fieldInitValues, child)...)
		// ignored
		case "identifier":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "constructor_declaration")
		}
	})
	if constructorNode == nil {
		// Default constructor - use class visibility
		if isPublicClass {
			modifiers = PUBLIC
		}
		body = append(body, fieldInitStmts(fieldInitValues)...)
	}
	body = append(body, &gosrc.ReturnStatement{Value: &gosrc.VarRef{Ref: gosrc.SelfRef}})
	name := constructorName(ctx, modifiers.isPublic(), gosrc.Type(structName), params...)
	retTy := gosrc.Type(structName)
	return gosrc.Function{
		Name:       name,
		Params:     params,
		ReturnType: &retTy,
		Body:       body,
		Public:     modifiers&PUBLIC != 0,
	}
}

func convertConstructorBody(ctx *MigrationContext, fieldInitValues *map[string]gosrc.Expression, bodyNode *tree_sitter.Node) []gosrc.Statement {
	body := fieldInitStmts(fieldInitValues)
	IterateChilden(bodyNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "explicit_constructor_invocation":
			body = append(body, convertExplicitConstructorInvocation(ctx, child)...)
		case "expression_statement":
			body = append(body, convertStatement(ctx, child)...)
			// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "constructor_body")
		}
	})
	return body
}

func fieldInitStmts(fieldInitValues *map[string]gosrc.Expression) []gosrc.Statement {
	if fieldInitValues == nil {
		return nil
	}
	var body []gosrc.Statement

	// Sort field names for consistent output
	fieldNames := make([]string, 0, len(*fieldInitValues))
	for fieldName := range *fieldInitValues {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		initExpr := (*fieldInitValues)[fieldName]
		body = append(body, &gosrc.AssignStatement{Ref: gosrc.VarRef{Ref: gosrc.SelfRef + "." + fieldName}, Value: initExpr})
	}
	if len(*fieldInitValues) > 0 {
		body = append(body, &gosrc.CommentStmt{Comments: []string{"Default field initializations"}})
	}
	return body
}
