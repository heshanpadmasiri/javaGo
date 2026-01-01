package java

import (
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func migrateInterfaceDeclaration(ctx *MigrationContext, interfaceNode *tree_sitter.Node) {
	var interfaceName string
	var superInterfaces []gosrc.Type
	var regularMethods []gosrc.InterfaceMethod
	var defaultMethods []gosrc.Function
	var staticMethods []gosrc.Function

	IterateChilden(interfaceNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			// Interfaces are always public, so we don't need to parse modifiers
		case "identifier":
			interfaceName = child.Utf8Text(ctx.JavaSource)
		case "extends_interfaces":
			// Parse extends clause - iterate through children to find type_list
			IterateChilden(child, func(extendsChild *tree_sitter.Node) {
				if extendsChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					IterateChilden(extendsChild, func(typeChild *tree_sitter.Node) {
						ty, ok := TryParseType(ctx, typeChild)
						if ok {
							superInterfaces = append(superInterfaces, ty)
						}
					})
				}
			})
		case "interface_body":
			// Parse methods in interface body
			IterateChilden(child, func(bodyChild *tree_sitter.Node) {
				switch bodyChild.Kind() {
				case "class_declaration":
					migrateClassDeclaration(ctx, bodyChild)
				case "record_declaration":
					migrateRecordDeclaration(ctx, bodyChild)
				case "enum_declaration":
					migrateEnumDeclaration(ctx, bodyChild)
				case "method_declaration":
					isDefault := HasModifier(ctx, bodyChild, "default")
					isStatic := HasModifier(ctx, bodyChild, "static")

					if isDefault {
						// Default method - convert to standalone function with 'this' parameter
						function := convertMethodDeclarationToFunction(ctx, bodyChild, true, interfaceName)
						defaultMethods = append(defaultMethods, function)
					} else if isStatic {
						// Static method - convert to package-level function
						function := convertMethodDeclarationToFunction(ctx, bodyChild, false, "")
						staticMethods = append(staticMethods, function)
					} else {
						// Regular method - add to interface
						method := extractInterfaceMethodSignature(ctx, bodyChild)
						regularMethods = append(regularMethods, method)
					}
				// ignored
				case "{":
				case "}":
				case ";":
				case "line_comment":
				case "block_comment":
				default:
					UnhandledChild(ctx, bodyChild, "interface_body")
				}
			})
		// ignored
		case "interface":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "interface_declaration")
		}
	})

	// Generate Go interface with regular methods
	goInterface := gosrc.Interface{
		Name: gosrc.CapitalizeFirstLetter(interfaceName),
		Embeds:   superInterfaces,
		Methods:  regularMethods,
		Public:   true, // Java interfaces are always public
		Comments: []string{},
	}
	ctx.Source.Interfaces = append(ctx.Source.Interfaces, goInterface)

	// Generate standalone functions for default methods
	for _, defaultMethod := range defaultMethods {
		ctx.Source.Functions = append(ctx.Source.Functions, defaultMethod)
	}

	// Generate package-level functions for static methods
	for _, staticMethod := range staticMethods {
		ctx.Source.Functions = append(ctx.Source.Functions, staticMethod)
	}
}

func extractInterfaceMethodSignature(ctx *MigrationContext, methodNode *tree_sitter.Node) gosrc.InterfaceMethod {
	var name string
	var params []gosrc.Param
	var returnType *gosrc.Type
	var hasThrows bool

	IterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := TryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(ctx.JavaSource)
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "void_type":
			returnType = nil
		case "throws":
			hasThrows = true
		// ignored
		case "modifiers":
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "interface_method_signature")
		}
	})

	// Handle throws clause - convert to error return
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

	return gosrc.InterfaceMethod{
		Name: gosrc.CapitalizeFirstLetter(name),
		Params:     params,
		ReturnType: returnType,
		Public:     true, // All interface methods are public
	}
}

func convertMethodDeclarationToFunction(ctx *MigrationContext, methodNode *tree_sitter.Node, isDefault bool, interfaceName string) gosrc.Function {
	var name string
	var params []gosrc.Param
	var body []gosrc.Statement
	var returnType *gosrc.Type
	var hasThrows bool

	IterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := TryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(ctx.JavaSource)
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "void_type":
			returnType = nil
		case "block":
			// For default methods, we need to convert the body to capitalize method calls
			if isDefault {
				// Set context for default method conversion
				oldInDefaultMethod := ctx.InDefaultMethod
				oldDefaultMethodSelf := ctx.DefaultMethodSelf
				ctx.InDefaultMethod = true
				ctx.DefaultMethodSelf = "this"

				// Convert block with empty field map (interfaces have no fields)
				rawBody := convertStatementBlock(ctx, child)
				for _, stmt := range rawBody {
					body = append(body, convertStatementForDefaultMethod(ctx, stmt, interfaceName, make(map[string]bool)))
				}

				// Restore context
				ctx.InDefaultMethod = oldInDefaultMethod
				ctx.DefaultMethodSelf = oldDefaultMethodSelf
			} else {
				body = append(body, convertStatementBlock(ctx, child)...)
			}
		case "throws":
			hasThrows = true
		// ignored
		case "modifiers":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, child, "interface_default_method")
		}
	})

	// Handle throws clause
	if hasThrows {
		if returnType == nil {
			errorType := gosrc.Type("error")
			returnType = &errorType
		} else {
			tupleType := gosrc.Type("(" + returnType.ToSource() + ", error)")
			returnType = &tupleType
		}
	}

	// If default method, prepend 'this' parameter
	if isDefault {
		thisParam := gosrc.Param{
			Name: "this",
			Ty: gosrc.Type(gosrc.CapitalizeFirstLetter(interfaceName)),
		}
		params = append([]gosrc.Param{thisParam}, params...)
	}

	return gosrc.Function{
		Name: gosrc.CapitalizeFirstLetter(name),
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Public:     true,
	}
}

