package java

import (
	"fmt"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func migrateInterfaceDeclaration(ctx *MigrationContext, interfaceNode *tree_sitter.Node) {
	var interfaceName string
	var superInterfaces []gosrc.Type
	var regularMethods []gosrc.InterfaceMethod
	var defaultMethods []gosrc.Function
	var staticMethods []gosrc.Function

	IterateChildren(interfaceNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			// Interfaces are always public, so we don't need to parse modifiers
		case "identifier":
			interfaceName = child.Utf8Text(ctx.JavaSource)
		case "extends_interfaces":
			// Parse extends clause - iterate through children to find type_list
			IterateChildren(child, func(extendsChild *tree_sitter.Node) {
				if extendsChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					IterateChildren(extendsChild, func(typeChild *tree_sitter.Node) {
						ty, ok := TryParseType(ctx, typeChild)
						if ok {
							superInterfaces = append(superInterfaces, ty)
						}
					})
				}
			})
		case "interface_body":
			// Parse methods in interface body
			IterateChildren(child, func(bodyChild *tree_sitter.Node) {
				// Skip ignored tokens
				switch bodyChild.Kind() {
				case "{", "}", ";", "line_comment", "block_comment":
					return
				}

				// Wrap member migration in error recovery
				failed := tryMigrateMember(ctx, fmt.Sprintf("interface %s.%s", interfaceName, bodyChild.Kind()), bodyChild, func() {
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
					default:
						UnhandledChild(ctx, bodyChild, "interface_body")
					}
				})

				if failed != nil {
					// Add to source as failed migration
					ctx.Source.FailedMigrations = append(ctx.Source.FailedMigrations, *failed)
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
		Name:     gosrc.CapitalizeFirstLetter(interfaceName),
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
	// Use cached metadata
	metadata := getMethodMetadata(ctx, methodNode)

	return gosrc.InterfaceMethod{
		Name:       gosrc.CapitalizeFirstLetter(metadata.name),
		Params:     metadata.params,
		ReturnType: metadata.returnTy,
		Public:     true, // All interface methods are public
	}
}

func convertMethodDeclarationToFunction(ctx *MigrationContext, methodNode *tree_sitter.Node, isDefault bool, interfaceName string) gosrc.Function {
	// Use cached metadata for signature
	metadata := getMethodMetadata(ctx, methodNode)
	name := metadata.name
	params := metadata.params
	returnType := metadata.returnTy

	// Parse body using ChildByFieldName
	var body []gosrc.Statement
	blockNode := methodNode.ChildByFieldName("body")
	if blockNode != nil {
		if isDefault {
			// Set context for default method conversion
			oldInDefaultMethod := ctx.InDefaultMethod
			oldDefaultMethodSelf := ctx.DefaultMethodSelf
			ctx.InDefaultMethod = true
			ctx.DefaultMethodSelf = "this"

			// Convert block with empty field map (interfaces have no fields)
			rawBody := convertStatementBlock(ctx, blockNode)
			for _, stmt := range rawBody {
				body = append(body, convertStatementForDefaultMethod(ctx, stmt, interfaceName, make(map[string]bool)))
			}

			// Restore context
			ctx.InDefaultMethod = oldInDefaultMethod
			ctx.DefaultMethodSelf = oldDefaultMethodSelf
		} else {
			body = convertStatementBlock(ctx, blockNode)
		}
	}

	// If default method, prepend 'this' parameter
	if isDefault {
		thisParam := gosrc.Param{
			Name: "this",
			Ty:   gosrc.Type(gosrc.CapitalizeFirstLetter(interfaceName)),
		}
		params = append([]gosrc.Param{thisParam}, params...)
	}

	// Add migration comment
	migrationComment := getMigrationComment(ctx, methodNode)

	return gosrc.Function{
		Name:       gosrc.CapitalizeFirstLetter(name),
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Public:     true,
		Comments:   []string{migrationComment},
	}
}
