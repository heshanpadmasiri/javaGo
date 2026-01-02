package java

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func convertArgumentList(ctx *MigrationContext, argList *tree_sitter.Node) []gosrc.Expression {
	var args []gosrc.Expression
	IterateChilden(argList, func(child *tree_sitter.Node) {
		switch child.Kind() {
		// ignored
		case "(":
		case ")":
		case ",":
		case "line_comment":
		case "block_comment":
		default:
			exp, init := convertExpression(ctx, child)
			if len(init) > 0 {
				diagnostics.Fatal(child.ToSexp(), errors.New("unexpected statements in argument list expression"))
			}
			args = append(args, exp)
		}
	})
	return args
}

func convertArrayInitializer(ctx *MigrationContext, initNode *tree_sitter.Node) []gosrc.Expression {
	var elements []gosrc.Expression
	IterateChilden(initNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "{", "}", ",":
			// Structural tokens - ignore
		case "line_comment":
		case "block_comment":
		default:
			// Any other node is an element expression
			exp, init := convertExpression(ctx, child)
			if len(init) > 0 {
				diagnostics.Fatal(child.ToSexp(), errors.New("unexpected statements in array initializer"))
			}
			elements = append(elements, exp)
		}
	})
	return elements
}

func convertAssignmentExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	// Check for compound assignment operators
	refNode := expression.ChildByFieldName("left")
	valueNode := expression.ChildByFieldName("right")

	// Check if this is a compound assignment by looking for operators like |=, &=, etc.
	var operator string
	IterateChilden(expression, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "|=", "&=", "^=", "<<=", ">>=", "+=", "-=", "*=", "/=", "%=":
			operator = child.Utf8Text(ctx.JavaSource)
		}
	})

	leftExp, leftInit := convertExpression(ctx, refNode)
	rightExp, rightInit := convertExpression(ctx, valueNode)
	stmts := append(leftInit, rightInit...)
	var valueExp gosrc.Expression
	if operator != "" {
		// This is a compound assignment: x op= y -> x = x op y

		// Extract the base operator (remove =)
		baseOp := operator[:len(operator)-1]

		// Convert >>>= to >>= (Go doesn't have >>>)
		if baseOp == ">>>" {
			baseOp = ">>"
		}

		valueExp = &gosrc.BinaryExpression{
			Left:     leftExp,
			Operator: baseOp,
			Right:    rightExp,
		}
	} else {
		// Regular assignment
		valueExp = rightExp
	}

	stmts = append(stmts, &gosrc.AssignStatement{
		Ref:   gosrc.VarRef{Ref: leftExp.ToSource()},
		Value: valueExp,
	})
	return nil, stmts
}

func convertArrayCreationExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	typeNode := expression.ChildByFieldName("type")
	ty, ok := TryParseType(ctx, typeNode)
	if !ok {
		diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in array_creation_expression"))
	}

	// Check for dimensions to make it an array type
	dimensionsNode := expression.ChildByFieldName("dimensions")
	if dimensionsNode != nil {
		// Add [] prefix to make it an array type
		ty = gosrc.Type("[]" + ty.ToSource())
	}

	valueNode := expression.ChildByFieldName("value")
	if valueNode == nil {
		// No initializer: return nil
		return &gosrc.GoExpression{Source: "nil"}, nil
	}

	// Has initializer: new gosrc.Type[] { ... }
	elements := convertArrayInitializer(ctx, valueNode)
	return &gosrc.ArrayLiteral{
		ElementType: ty,
		Elements:    elements,
	}, nil
}

func handleFailedToFindConstructor(ty gosrc.Type) (gosrc.Expression, []gosrc.Statement) {
	// Generate no-args constructor name
	// Assume constructor is always public: NewTypeName()
	typeName := ty.ToSource()
	constructorName := "New" + gosrc.CapitalizeFirstLetter(typeName)

	// Call the no-args constructor with a FIXME comment
	comment := fmt.Sprintf("FIXME: failed to find constructor for %s", ty)
	callExpr := &gosrc.CallExpression{
		Function: constructorName,
		Args:     []gosrc.Expression{},
	}
	return callExpr, []gosrc.Statement{
		&gosrc.CommentStmt{Comments: []string{comment}},
	}
}

// extractTypeArguments extracts type arguments from a generic type node
// Returns a slice of Go type strings (e.g., ["string", "int"])
func extractTypeArguments(ctx *MigrationContext, expression *tree_sitter.Node) []string {
	var types []string
	typeArgsNode := expression.ChildByFieldName("type").ChildByFieldName("type_arguments")
	if typeArgsNode != nil {
		IterateChilden(typeArgsNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "type_identifier":
				childTy, ok := TryParseType(ctx, child)
				if ok {
					types = append(types, string(childTy))
				}
			case "integral_type":
				types = append(types, "int")
			case "boolean_type":
				types = append(types, "bool")
			}
		})
	}
	return types
}

// TODO: ai slop revist this later
func convertArrayListCreationExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	// Extract element type from generic if present: ArrayList<Type> -> Type
	elementType := "interface{}"
	types := extractTypeArguments(ctx, expression)
	if len(types) >= 1 {
		elementType = types[0]
	}

	// Convert to Go slice: make([]Type, 0)
	return &gosrc.GoExpression{
		Source: fmt.Sprintf("make([]%s, 0)", elementType),
	}, nil
}

// TODO: ai slop revist this later
func convertHashSetCreationExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	// Extract element type from generic if present: HashSet<Type> -> Type
	elementType := "interface{}"
	types := extractTypeArguments(ctx, expression)
	if len(types) >= 1 {
		elementType = types[0]
	}

	// Convert to Go map with bool values: make(map[Type]bool)
	return &gosrc.GoExpression{
		Source: fmt.Sprintf("make(map[%s]bool)", elementType),
	}, nil
}

// TODO: ai slop revist this later
func convertHashMapCreationExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	// Extract key and value types from generics if present
	keyType := "interface{}"
	valueType := "interface{}"
	types := extractTypeArguments(ctx, expression)
	if len(types) >= 1 {
		keyType = types[0]
	}
	if len(types) >= 2 {
		valueType = types[1]
	}

	// Convert to Go map: make(map[keyType]valueType)
	return &gosrc.GoExpression{
		Source: fmt.Sprintf("make(map[%s]%s)", keyType, valueType),
	}, nil
}

func convertObjectCreationExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	ty, isType := TryParseType(ctx, expression.ChildByFieldName("type"))
	if !isType {
		diagnostics.Fatal(expression.ToSexp(), errors.New("unable to parse type in object_creation_expression"))
	}
	if ty.IsArray() {
		return &gosrc.GoExpression{
			Source: fmt.Sprintf("make(%s, 0)", ty),
		}, nil
	}

	// Check for ArrayList creation: new ArrayList<>() or new ArrayList<Type>()
	typeText := expression.ChildByFieldName("type").Utf8Text(ctx.JavaSource)
	if strings.Contains(typeText, "ArrayList") {
		return convertArrayListCreationExpression(ctx, expression)
	}

	// Check for LinkedList creation: new LinkedList<>() or new LinkedList<Type>()
	// LinkedList is also converted to a slice in Go (same as ArrayList)
	if strings.Contains(typeText, "LinkedList") {
		return convertArrayListCreationExpression(ctx, expression)
	}

	// Check for HashSet creation: new HashSet<>() or new HashSet<Type>()
	if strings.Contains(typeText, "HashSet") {
		return convertHashSetCreationExpression(ctx, expression)
	}

	// Check for HashMap creation: new HashMap<>() or new HashMap<K, V>()
	if strings.Contains(typeText, "HashMap") {
		return convertHashMapCreationExpression(ctx, expression)
	}

	// Get arguments from the object creation expression
	argsNode := expression.ChildByFieldName("arguments")
	var args []gosrc.Expression
	if argsNode != nil {
		args = convertArgumentList(ctx, argsNode)
	}

	// Look up constructors for this type
	// Try with the type as-is first, then try with lowercase first letter (for non-public classes)
	constructors, hasConstructors := ctx.Constructors[ty]
	if !hasConstructors {
		// Try with lowercase first letter for non-public classes
		lowercaseTy := gosrc.Type(gosrc.LowercaseFirstLetter(string(ty)))
		constructors, hasConstructors = ctx.Constructors[lowercaseTy]
	}
	if !hasConstructors {
		// No constructors registered for this type
		return handleFailedToFindConstructor(ty)
	}

	// Try to find matching constructor by parameter count
	constructorName, found, multipleMatch := tryGuessOverloadedMethod(constructors, len(args))

	if !found {
		// No constructor with matching number of parameters
		return handleFailedToFindConstructor(ty)
	}

	// Generate constructor call
	callExpr := &gosrc.CallExpression{
		Function: constructorName,
		Args:     args,
	}

	if multipleMatch {
		// Multiple constructors match - add FIXME comment as init statement
		comment := fmt.Sprintf("FIXME: more than one possible constructor for %s", ty)
		return callExpr, []gosrc.Statement{
			&gosrc.CommentStmt{Comments: []string{comment}},
		}
	}

	// Exactly one constructor matches - return clean call
	return callExpr, nil
}

func convertIdentifier(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	identName := expression.Utf8Text(ctx.JavaSource)
	// Check if this is an enum constant reference
	if prefixedName, ok := ctx.EnumConstants[identName]; ok {
		return &gosrc.VarRef{
			Ref: prefixedName,
		}, nil
	}
	return &gosrc.VarRef{
		Ref: identName,
	}, nil
}

func convertInstanceofExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	valueNode := expression.ChildByFieldName("left")
	valueExp, initStmts := convertExpression(ctx, valueNode)
	Assert("condition expression is expected to be simple", len(initStmts) == 0)
	typeNode := expression.ChildByFieldName("right")
	ty, ok := TryParseType(ctx, typeNode)
	if !ok {
		diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in instanceof_expression"))
	}
	return &gosrc.GoExpression{
		Source: fmt.Sprintf("%s.(%s)", valueExp.ToSource(), ty.ToSource()),
	}, nil
}

func convertCastExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	typeNode := expression.ChildByFieldName("type")
	ty, ok := TryParseType(ctx, typeNode)
	if !ok {
		diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in cast_expression"))
	}
	valueNode := expression.ChildByFieldName("value")
	valueExp, initStmts := convertExpression(ctx, valueNode)
	return &gosrc.CastExpression{
		Ty:    ty,
		Value: valueExp,
	}, initStmts
}

func convertUnaryExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	operandNode := expression.ChildByFieldName("operand")
	operand, initStmts := convertExpression(ctx, operandNode)
	var operator string
	IterateChilden(expression, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "!", "+", "-", "~":
			operator = child.Utf8Text(ctx.JavaSource)
		}
	})
	Assert("unary expression operator not found", operator != "")
	return &gosrc.UnaryExpression{
		Operator: operator,
		Operand:  operand,
	}, initStmts
}

func convertMethodReference(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	// Handle method references like gosrc.Type[]::new
	// This is typically used for array constructors: gosrc.Type[]::new -> make([]gosrc.Type, 0)
	objectNode := expression.ChildByFieldName("object")
	methodNode := expression.ChildByFieldName("method")

	if objectNode != nil && methodNode != nil {
		objectText := objectNode.Utf8Text(ctx.JavaSource)
		methodText := methodNode.Utf8Text(ctx.JavaSource)

		// Check if this is an array constructor: gosrc.Type[]::new
		if methodText == "new" && strings.HasSuffix(objectText, "[]") {
			// Extract the element type
			elementType := strings.TrimSuffix(objectText, "[]")
			// Convert to Go: make([]gosrc.Type, 0)
			return &gosrc.GoExpression{
				Source: fmt.Sprintf("make([]%s, 0)", elementType),
			}, nil
		}
	}

	// Fallback: return as-is (may need more sophisticated handling)
	return &gosrc.GoExpression{
		Source: expression.Utf8Text(ctx.JavaSource),
	}, nil
}

func convertFieldAccess(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	object := expression.ChildByFieldName("object")
	field := expression.ChildByFieldName("field")

	if object != nil && field != nil {
		objectText := object.Utf8Text(ctx.JavaSource)
		fieldText := field.Utf8Text(ctx.JavaSource)

		// Check if this looks like an enum constant (object is type name, field is uppercase)
		// Heuristic: if object starts with uppercase, it's likely a type/enum reference
		if len(objectText) > 0 && objectText[0] >= 'A' && objectText[0] <= 'Z' {
			// Enum constant: Foo.BAR → Foo_BAR
			return &gosrc.VarRef{
				Ref: objectText + "_" + fieldText,
			}, nil
		}
		// Regular field access: keep dot notation
		return &gosrc.VarRef{
			Ref: objectText + "." + fieldText,
		}, nil
	}

	// Fallback to original text
	return &gosrc.VarRef{
		Ref: expression.Utf8Text(ctx.JavaSource),
	}, nil
}

func convertBinaryExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	leftNode := expression.ChildByFieldName("left")
	left, leftInit := convertExpression(ctx, leftNode)
	rightNode := expression.ChildByFieldName("right")
	rigth, rightInit := convertExpression(ctx, rightNode)
	stms := append(leftInit, rightInit...)
	var operator string
	IterateChilden(expression, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "||", "&&", "==", "!=", "<", "<=", ">", ">=", "+", "-", "*", "/", "%":
			operator = child.Utf8Text(ctx.JavaSource)
		case "<<", ">>", ">>>":
			// Bit shift operators
			operator = child.Utf8Text(ctx.JavaSource)
			// Go uses >> for both signed and unsigned right shift
			if operator == ">>>" {
				operator = ">>"
			}
		case "|", "&", "^":
			// Bitwise operators
			operator = child.Utf8Text(ctx.JavaSource)
		}
	})
	Assert("binary expression operator not found", operator != "")
	return &gosrc.BinaryExpression{
		Left:     left,
		Operator: operator,
		Right:    rigth,
	}, stms
}

func convertMethodInvocation(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	name := expression.ChildByFieldName("name").Utf8Text(ctx.JavaSource)
	objectNode := expression.ChildByFieldName("object")
	objectText := ""
	if objectNode != nil {
		objectText = objectNode.Utf8Text(ctx.JavaSource)
	}

	switch name {
	case "equals":
		// String.equals(other) -> string == other
		argsNode := expression.ChildByFieldName("arguments")
		if argsNode != nil {
			args := convertArgumentList(ctx, argsNode)
			if len(args) > 0 {
				// Convert: "active".equals(s) -> "active" == s
				return &gosrc.BinaryExpression{
					Left:     &gosrc.VarRef{Ref: objectText},
					Operator: "==",
					Right:    args[0],
				}, nil
			}
		}
	case "size":
		return &gosrc.GoExpression{
			Source: fmt.Sprintf("len(%s)", objectText),
		}, nil
	case "asList":
		// Arrays.asList(...) -> []gosrc.Type{...}
		// Only handle if object is "Arrays"
		if objectText == "Arrays" {
			argsNode := expression.ChildByFieldName("arguments")
			if argsNode != nil {
				args := convertArgumentList(ctx, argsNode)
				if len(args) > 0 {
					// Convert arguments to slice literal
					// Use interface{} as element type (could be improved with type inference)
					return &gosrc.ArrayLiteral{
						ElementType: gosrc.Type("interface{}"),
						Elements:    args,
					}, nil
				}
			}
			return &gosrc.GoExpression{
				Source: "[]interface{}{}",
			}, nil
		}
	case "toArray":
		// list.toArray(gosrc.Type[]::new) -> convert to slice
		// The method reference is already handled, so this should work
		// For now, return the object as a slice (assuming it's already a slice)
		return &gosrc.GoExpression{
			Source: objectText,
		}, nil
	case "add":
		// list.add(item) -> list = append(list, item)
		// This needs to be handled as a statement, not an expression
		// For now, return as Go expression that can be used in statements
		argsNode := expression.ChildByFieldName("arguments")
		var initStmts []gosrc.Statement
		ref := gosrc.VarRef{Ref: objectText}
		if argsNode != nil {
			values := convertArgumentList(ctx, argsNode)
			if len(values) > 0 {
				var args []gosrc.Expression
				args = append(args, &ref)
				args = append(args, values...)
				appendCall := &gosrc.CallExpression{Function: "append", Args: args}
				initStmts = append(initStmts, &gosrc.AssignStatement{Ref: ref, Value: appendCall})
			}
		}
		return &ref, initStmts
	default:
		// Handle method calls on this or other objects
		if objectText == "this" || objectText == gosrc.SelfRef {
			// Special handling for Java enum name() method
			if name == "name" {
				// this.Name() -> this.Name() (will need a Name() method implementation)
				return &gosrc.GoExpression{
					Source: fmt.Sprintf("%s.Name()", gosrc.SelfRef),
				}, nil
			}
			// gosrc.Method call on this - just capitalize method name
			capitalizedName := gosrc.CapitalizeFirstLetter(name)
			argsNode := expression.ChildByFieldName("arguments")
			var argsStr string
			if argsNode != nil {
				args := convertArgumentList(ctx, argsNode)
				argStrs := make([]string, len(args))
				for i, arg := range args {
					argStrs[i] = arg.ToSource()
				}
				argsStr = strings.Join(argStrs, ", ")
			}
			return &gosrc.GoExpression{
				Source: fmt.Sprintf("%s.%s(%s)", gosrc.SelfRef, capitalizedName, argsStr),
			}, nil
		}
		// Handle method calls on enum constants
		if prefixedName, ok := ctx.EnumConstants[objectText]; ok {
			// If objectText is an enum constant, prepend its prefixed name
			return &gosrc.GoExpression{
				Source: fmt.Sprintf("%s.%s", prefixedName, gosrc.CapitalizeFirstLetter(name)),
			}, nil
		}
		// TODO: fix casts
		// Fallback: convert the expression and clean up any this.this patterns
		exprText := expression.Utf8Text(ctx.JavaSource)
		// If expression already starts with "this.", don't prepend another "this."
		if strings.HasPrefix(exprText, "this.") {
			// Clean up any this.this patterns
			exprText = strings.ReplaceAll(exprText, "this.this.", "this.")
			return &gosrc.GoExpression{
				Source: exprText,
			}, nil
		}
		return &gosrc.GoExpression{
			Source: gosrc.SelfRef + "." + exprText,
		}, nil
	}
	// Fallback
	return &gosrc.GoExpression{
		Source: expression.Utf8Text(ctx.JavaSource),
	}, nil
}

func convertExpression(ctx *MigrationContext, expression *tree_sitter.Node) (gosrc.Expression, []gosrc.Statement) {
	switch expression.Kind() {
	case "this":
		return &gosrc.GoExpression{Source: "this"}, nil
	case "assignment_expression":
		return convertAssignmentExpression(ctx, expression)
	case "ternary_expression":
		// TODO: do better
		return &gosrc.GoExpression{
			Source: expression.Utf8Text(ctx.JavaSource),
		}, nil
	case "array_creation_expression":
		return convertArrayCreationExpression(ctx, expression)
	case "instanceof_expression":
		return convertInstanceofExpression(ctx, expression)
	case "update_expression":
		return &gosrc.GoExpression{
			Source: expression.Utf8Text(ctx.JavaSource),
		}, nil
	case "switch_expression":
		switchStatement := convertSwitchStatement(ctx, expression)
		return &switchStatement, nil
	case "identifier":
		return convertIdentifier(ctx, expression)
	case "array_access":
		return &gosrc.GoExpression{
			Source: expression.Utf8Text(ctx.JavaSource),
		}, nil
	case "object_creation_expression":
		return convertObjectCreationExpression(ctx, expression)
	case "field_access":
		return convertFieldAccess(ctx, expression)
	case "method_invocation":
		return convertMethodInvocation(ctx, expression)
	case "return":
		var initStmts []gosrc.Statement
		var value gosrc.Expression
		if expression.ChildCount() == 1 {
			value, initStmts = convertExpression(ctx, expression.Child(0))
		}
		return &gosrc.ReturnExpression{
			Value: value,
		}, initStmts
	case "parenthesized_expression":
		return convertExpression(ctx, expression.Child(1))
	case "binary_expression":
		return convertBinaryExpression(ctx, expression)
	case "character_literal":
		return &gosrc.CharLiteral{
			Value: expression.Utf8Text(ctx.JavaSource),
		}, nil
	case "string_literal":
		return &gosrc.GoExpression{
			Source: expression.Utf8Text(ctx.JavaSource),
		}, nil
	case "null_literal":
		return &gosrc.NIL, nil
	case "true":
		return &gosrc.BooleanLiteral{
			Value: true,
		}, nil
	case "false":
		return &gosrc.BooleanLiteral{
			Value: false,
		}, nil
	case "decimal_integer_literal":
		n, err := strconv.ParseInt(expression.Utf8Text(ctx.JavaSource), 10, 64)
		if err != nil {
			diagnostics.Fatal(expression.ToSexp(), err)
		}
		return &gosrc.IntLiteral{
			Value: int(n),
		}, nil
	case "hex_integer_literal":
		n, err := strconv.ParseInt(expression.Utf8Text(ctx.JavaSource), 0, 64)
		if err != nil {
			diagnostics.Fatal(expression.ToSexp(), err)
		}
		return &gosrc.IntLiteral{
			Value: int(n),
		}, nil
	case "unary_expression":
		return convertUnaryExpression(ctx, expression)
	case "cast_expression":
		return convertCastExpression(ctx, expression)
	case "method_reference":
		return convertMethodReference(ctx, expression)
	default:
		fmt.Println(expression.Utf8Text(ctx.JavaSource))
		expression.Parent()
		diagnostics.Fatal(expression.ToSexp(), errors.New("unhandled expression kind: "+expression.Kind()))
	}
	panic("unreachable")
}
