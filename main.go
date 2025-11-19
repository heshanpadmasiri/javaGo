package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

type MigrationContext struct {
	source     GoSource
	javaSource []byte
}

type GoSource struct {
	imports   []GoImport
	structs   []GoStruct
	functions []GoFunction
	methods   []GoMethod
}

func (s *GoSource) ToSource() string {
	sb := strings.Builder{}
	if len(s.imports) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range s.imports {
			sb.WriteString("    ")
			sb.WriteString(imp.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString(")\n\n")
	}
	for _, strct := range s.structs {
		sb.WriteString(strct.ToSource())
		sb.WriteString("\n")
	}
	for _, fn := range s.functions {
		sb.WriteString(fn.ToSource())
		sb.WriteString("\n")
	}
	for _, method := range s.methods {
		sb.WriteString(method.ToSource())
		sb.WriteString("\n")
	}
	return sb.String()
}

type GoImport struct {
	packagePath string
	alias       *string
}

func (imp *GoImport) ToSource() string {
	if imp.alias != nil {
		return fmt.Sprintf("%s \"%s\"", *imp.alias, imp.packagePath)
	}
	return fmt.Sprintf("\"%s\"", imp.packagePath)
}

type GoStruct struct {
	name     string
	fields   []GoStructField
	public   bool
	comments []string
}

func addComments(sb *strings.Builder, comments []string) {
	for _, comment := range comments {
		sb.WriteString("// ")
		sb.WriteString(comment)
		sb.WriteString("\n")
	}
}

func (s *GoStruct) ToSource() string {
	sb := strings.Builder{}
	addComments(&sb, s.comments)
	sb.WriteString("type ")
	sb.WriteString(toIdentifier(s.name, s.public))
	sb.WriteString(" struct {\n")
	for _, field := range s.fields {
		sb.WriteString("    ")
		sb.WriteString(field.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

type GoFunction struct {
	name       string
	params     []GoParam
	returnType *GoType
	body       []GoStatement
	comments   []string
	public     bool
}

type GoMethod struct {
	GoFunction
	receiver GoParam
}

// TODO: this is basically same as GoFunction.ToSource, refactor
func (f *GoMethod) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString("(")
	sb.WriteString(f.receiver.ToSource())
	sb.WriteString(") ")
	sb.WriteString(toIdentifier(f.name, f.public))
	return finishGoFunctionToSource(&sb, &f.GoFunction)
}

func (f *GoFunction) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString(toIdentifier(f.name, f.public))
	return finishGoFunctionToSource(&sb, f)
}

func finishGoFunctionToSource(sb *strings.Builder, f *GoFunction) string {
	sb.WriteString("(")
	for i, param := range f.params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(param.ToSource())
	}
	sb.WriteString(")")
	if f.returnType != nil {
		sb.WriteString(" ")
		sb.WriteString(f.returnType.ToSource())
	}
	sb.WriteString(" {\n")
	addComments(sb, f.comments)
	for _, stmt := range f.body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

type GoParam struct {
	name string
	ty   GoType
}

func (p *GoParam) ToSource() string {
	return fmt.Sprintf("%s %s", p.name, p.ty.ToSource())
}

type GoStructField struct {
	name     string
	ty       GoType
	public   bool
	comments []string
}

func (f *GoStructField) ToSource() string {
	sb := strings.Builder{}
	addComments(&sb, f.comments)
	sb.WriteString(fmt.Sprintf("%s %s", toIdentifier(f.name, f.public), f.ty.ToSource()))
	return sb.String()
}

func toIdentifier(name string, public bool) string {
	first := name[0]
	if first >= 'a' && first <= 'z' && public {
		first = first - 'a' + 'A'
	} else if first >= 'A' && first <= 'Z' && !public {
		first = first - 'A' + 'a'
	}
	return string(first) + name[1:]
}

func capitalizeFirstLetter(name string) string {
	first := name[0]
	if first >= 'a' && first <= 'z' {
		first = first - 'a' + 'A'
	}
	return string(first) + name[1:]
}

type GoType string

const (
	GoTypeInt     GoType = "int"
	GoTypeString  GoType = "string"
	GoTypeBool    GoType = "bool"
	GoTypeFloat64 GoType = "float64"
)

func (t *GoType) ToSource() string {
	return string(*t)
}

type GoStatement interface {
	SourceElement
}

type GoIfStatement struct {
	condition GoExpression
	body      []GoStatement
}

type GoVarDeclaration struct {
	name  string
	ty    GoType
	value GoExpression
}

func (s *GoVarDeclaration) ToSource() string {
	if s.value != nil {
		return fmt.Sprintf("%s := %s", s.name, s.value.ToSource())
	}
	return fmt.Sprintf("var %s %s", s.name, s.ty.ToSource())
}

func (s *GoIfStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("if ")
	sb.WriteString(s.condition.ToSource())
	sb.WriteString(" {\n")
	for _, stmt := range s.body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

type GoReturnStatement struct {
	value GoExpression
}

func (s *GoReturnStatement) ToSource() string {
	if s.value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", s.value.ToSource())
}

type GoCommentStmt struct {
	comments []string
}

func (s *GoCommentStmt) ToSource() string {
	sb := strings.Builder{}
	addComments(&sb, s.comments)
	return sb.String()
}

type GoForStatement struct {
	init      GoStatement
	condition GoExpression
	post      GoStatement
	body      []GoStatement
}

func (s *GoForStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("for ")
	if s.init != nil {
		sb.WriteString(s.init.ToSource())
	} else {
		sb.WriteString("; ")
	}
	if s.condition != nil {
		sb.WriteString(s.condition.ToSource())
		sb.WriteString("; ")
	} else {
		sb.WriteString("; ")
	}
	if s.post != nil {
		sb.WriteString(s.post.ToSource())
	} else {
		sb.WriteString(" ")
	}
	sb.WriteString(" {\n")
	for _, stmt := range s.body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

type GoCallStatement struct {
	exp GoExpression
}

type GoAssignStatement struct {
	ref   GoVarRef
	value GoExpression
}

func (s *GoAssignStatement) ToSource() string {
	return fmt.Sprintf("%s = %s", s.ref.ToSource(), s.value.ToSource())
}

func (s *GoCallStatement) ToSource() string {
	return s.exp.ToSource()
}

type GoExpression interface {
	SourceElement
}

type GoReturnExpression struct {
	value GoExpression
}

func (e *GoReturnExpression) ToSource() string {
	if e.value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", e.value.ToSource())
}

type GoCallExpression struct {
	function string
	args     []GoExpression
}

func (e *GoCallExpression) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString(e.function)
	sb.WriteString("(")
	for i, arg := range e.args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.ToSource())
	}
	sb.WriteString(")")
	return sb.String()
}

type GoVarRef struct {
	ref string
}

var GoNil = GoVarRef{ref: "nil"}

func (e *GoVarRef) ToSource() string {
	return e.ref
}

type StringLiteral struct {
	value string
}

type BooleanLiteral struct {
	value bool
}

type IntLiteral struct {
	value int
}

func (e *IntLiteral) ToSource() string {
	return fmt.Sprintf("%d", e.value)
}

func (e *BooleanLiteral) ToSource() string {
	return fmt.Sprintf("%t", e.value)
}

func (e *StringLiteral) ToSource() string {
	return fmt.Sprintf("\"%s\"", e.value)
}

type BinaryExpression struct {
	left     GoExpression
	operator string
	right    GoExpression
}

type UnaryExpression struct {
	operator string
	operand  GoExpression
}

func (e *UnaryExpression) ToSource() string {
	return fmt.Sprintf("(%s%s)", e.operator, e.operand.ToSource())
}

func (e *BinaryExpression) ToSource() string {
	return fmt.Sprintf("(%s %s %s)", e.left.ToSource(), e.operator, e.right.ToSource())
}

type UnhandledExpression struct {
	text string
}

func (e *UnhandledExpression) ToSource() string {
	return e.text
}

type SourceElement interface {
	ToSource() string
}

func main() {
	args := os.Args[1:]
	sourcePath := args[0]
	var destPath *string
	if len(args) > 1 {
		destPath = &args[1]
	}
	javaSource, err := os.ReadFile(sourcePath)
	Fatal("reading source file failed due to: ", err)

	tree := parseJava(javaSource)
	defer tree.Close()

	ctx := &MigrationContext{
		javaSource: javaSource,
	}
	migrateTree(ctx, tree)
	goSource := ctx.source.ToSource()
	if destPath != nil {
		// FIXME: use a proper mode
		err = os.WriteFile(*destPath, []byte(goSource), 0644)
	} else {
		fmt.Println(goSource)
	}
}

func migrateTree(ctx *MigrationContext, tree *tree_sitter.Tree) {
	root := tree.RootNode()
	migrateNode(ctx, root)
}

func migrateNode(ctx *MigrationContext, node *tree_sitter.Node) {
	switch node.Kind() {
	case "program":
		iterateChilden(node, func(child *tree_sitter.Node) {
			migrateNode(ctx, child)
		})
	case "class_declaration":
		migrateClassDeclaration(ctx, node)
	// Ignored
	case "block_comment":
	case "package_declaration":
	case "import_declaration":
	default:
		unhandledChild(ctx, node, "<root>")
	}
}

func migrateClassDeclaration(ctx *MigrationContext, classNode *tree_sitter.Node) {
	var className string
	var modifiers modifiers
	iterateChilden(classNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "identifier":
			className = child.Utf8Text(ctx.javaSource)
		case "class_body":
			result := convertClassBody(ctx, toIdentifier(className, modifiers.isPublic()), child)
			for _, function := range result.functions {
				ctx.source.functions = append(ctx.source.functions, function)
			}
			for _, method := range result.methods {
				ctx.source.methods = append(ctx.source.methods, method)
			}
			ctx.source.structs = append(ctx.source.structs, GoStruct{
				name:     className,
				fields:   result.fields,
				comments: result.comments,
				public:   modifiers&PUBLIC != 0,
			})
		// ignored
		case "class":
		default:
			unhandledChild(ctx, child, "class_declaration")
		}
	})
}

type classConversionResult struct {
	fields    []GoStructField
	comments  []string
	functions []GoFunction
	methods   []GoMethod
}

func convertClassBody(ctx *MigrationContext, structName string, classBody *tree_sitter.Node) classConversionResult {
	var result classConversionResult
	fieldInitValues := map[string]GoExpression{}
	iterateChilden(classBody, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "field_declaration":
			field, initExpr := convertFieldDeclaration(ctx, child)
			if initExpr != nil {
				Assert("mutiple initializations for field"+field.name, fieldInitValues[field.name] == nil)
				fieldInitValues[field.name] = initExpr
			}
			result.fields = append(result.fields, field)
		case "constructor_declaration":
			constructor := convertConstructor(ctx, &fieldInitValues, structName, child)
			result.functions = append(result.functions, constructor)
		case "method_declaration":
			result.functions = append(result.functions, convertMethodDeclaration(ctx, child))
		// ignored
		case "{":
		case "}":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "class_body")
		}
	})
	return result
}

// TODO: this is very similar to constructor conversion, refactor
func convertMethodDeclaration(ctx *MigrationContext, methodNode *tree_sitter.Node) GoFunction {
	var modifiers modifiers
	var params []GoParam
	var body []GoStatement
	var name string
	var returnType *GoType
	iterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := tryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "identifier":
			name = child.Utf8Text(ctx.javaSource)
		case "void_type":
			returnType = nil
		case "block":
			body = append(body, convertStatementBlock(ctx, child)...)
		// ignored
		case ";":
		default:
			unhandledChild(ctx, child, "method_declaration")
		}
	})
	return GoFunction{
		name:       name,
		params:     params,
		returnType: returnType,
		body:       body,
		public:     modifiers&PUBLIC != 0,
	}
}

func convertStatementBlock(ctx *MigrationContext, blockNode *tree_sitter.Node) []GoStatement {
	var body []GoStatement
	iterateChilden(blockNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		//ignored
		case "{":
		case "}":
		default:
			body = append(body, convertStatement(ctx, child)...)
		}
	})
	return body
}

func convertConstructor(ctx *MigrationContext, fieldInitValues *map[string]GoExpression, structName string, constructorNode *tree_sitter.Node) GoFunction {
	var modifiers modifiers
	var params []GoParam
	var body []GoStatement
	iterateChilden(constructorNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "constructor_body":
			body = convertConstructorBody(ctx, fieldInitValues, structName, child)
		// ignored
		case "identifier":
		default:
			unhandledChild(ctx, child, "constructor_declaration")
		}
	})
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(toIdentifier("new", modifiers.isPublic()))
	nameBuilder.WriteString(capitalizeFirstLetter(structName))
	nameBuilder.WriteString("From")
	for _, param := range params {
		nameBuilder.WriteString(capitalizeFirstLetter(param.name))
	}
	name := nameBuilder.String()
	retTy := GoType(structName)
	return GoFunction{
		name:       name,
		params:     params,
		returnType: &retTy,
		body:       body,
		public:     modifiers&PUBLIC != 0,
	}
}

func convertConstructorBody(ctx *MigrationContext, fieldInitValues *map[string]GoExpression, structName string, bodyNode *tree_sitter.Node) []GoStatement {
	var body []GoStatement
	for fieldName, initExpr := range *fieldInitValues {
		body = append(body, &GoAssignStatement{ref: GoVarRef{ref: fieldName}, value: initExpr})
	}
	if len(*fieldInitValues) > 0 {
		body = append(body, &GoCommentStmt{comments: []string{"Default field initializations"}})
	}
	iterateChilden(bodyNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "explicit_constructor_invocation":
			body = append(body, convertExplicitConstructorInvocation(ctx, child)...)
		case "expression_statement":
			body = append(body, convertStatement(ctx, child)...)
			// ignored
		case "{":
		case "}":
		default:
			unhandledChild(ctx, child, "constructor_body")
		}
	})
	return body
}

func convertStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []GoStatement {
	switch stmtNode.Kind() {
	case "line_comment":
		return nil
	case "expression_statement":
		var body []GoStatement
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "assignment_expression":
				refNode := child.ChildByFieldName("left")
				ref := GoVarRef{ref: refNode.Utf8Text(ctx.javaSource)}
				valueNode := child.ChildByFieldName("right")
				valueExp, initStmts := convertExpression(ctx, valueNode)
				if len(initStmts) > 0 {
					Fatal(valueNode.ToSexp(), errors.New("unexpected statements in assignment expression"))
				}
				body = append(body, &GoAssignStatement{
					ref:   ref,
					value: valueExp,
				})
			case "method_invocation":
				callExperession, initStmts := convertExpression(ctx, child)
				body = append(body, initStmts...)
				body = append(body, &GoCallStatement{exp: callExperession})
			//ignored
			case ";":
			default:
				unhandledChild(ctx, child, "expression_statement")
			}
		})
		return body
	case "return_statement":
		var initialStmts []GoStatement
		var value GoExpression
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			if child.Kind() == ";" {
				return
			}
			value, initialStmts = convertExpression(ctx, child)
		})
		return append(initialStmts, &GoReturnStatement{value: value})
	case "if_statement":
		conditionNode := stmtNode.ChildByFieldName("condition")
		conditionExp, stmts := convertExpression(ctx, conditionNode)
		bodyNode := stmtNode.ChildByFieldName("consequence")
		bodyStmts := convertStatementBlock(ctx, bodyNode)
		return append(stmts, &GoIfStatement{
			condition: conditionExp,
			body:      bodyStmts,
		})
	case "local_variable_declaration":
		typeNode := stmtNode.ChildByFieldName("type")
		ty, ok := tryParseType(ctx, typeNode)
		if !ok {
			Fatal(typeNode.ToSexp(), errors.New("unable to parse type in local_variable_declaration"))
		}
		declNode := stmtNode.ChildByFieldName("declarator")
		name := declNode.ChildByFieldName("name").Utf8Text(ctx.javaSource)
		valueNode := declNode.ChildByFieldName("value")
		if valueNode == nil {
			return []GoStatement{
				&GoVarDeclaration{
					name: name,
					ty:   ty,
				}}
		}
		valueExpr, initStmts := convertExpression(ctx, valueNode)
		return append(initStmts, &GoVarDeclaration{
			name:  name,
			ty:    ty,
			value: valueExpr,
		})
	case "while_statement":
		conditionNode := stmtNode.ChildByFieldName("condition")
		conditionExp, initStmts := convertExpression(ctx, conditionNode)
		bodyNode := stmtNode.ChildByFieldName("body")
		bodyStmts := convertStatementBlock(ctx, bodyNode)
		return append(initStmts, &GoForStatement{
			condition: conditionExp,
			body:      bodyStmts,
		})
	default:
		unhandledChild(ctx, stmtNode, "statement")
	}
	panic("unreachable")
}

func convertExplicitConstructorInvocation(ctx *MigrationContext, invocationNode *tree_sitter.Node) []GoStatement {
	parentCall := "this"
	var argExp []GoExpression
	iterateChilden(invocationNode, func(args *tree_sitter.Node) {
		switch args.Kind() {
		case "this":
			parentCall = "this"
		case "argument_list":
			argExp = convertArgumentList(ctx, args)
		// ignored
		case ";":
		default:
			unhandledChild(ctx, args, "explicit_constructor_invocation")
		}
	})
	return []GoStatement{
		&GoCallStatement{
			exp: &GoCallExpression{
				function: parentCall,
				args:     argExp,
			}}}
}

func convertArgumentList(ctx *MigrationContext, argList *tree_sitter.Node) []GoExpression {
	var args []GoExpression
	iterateChilden(argList, func(child *tree_sitter.Node) {
		switch child.Kind() {
		// ignored
		case "(":
		case ")":
		case ",":
		default:
			exp, init := convertExpression(ctx, child)
			if len(init) > 0 {
				Fatal(child.ToSexp(), errors.New("unexpected statements in argument list expression"))
			}
			args = append(args, exp)
		}
	})
	return args
}

func convertExpression(ctx *MigrationContext, expression *tree_sitter.Node) (GoExpression, []GoStatement) {
	switch expression.Kind() {
	case "identifier":
		return &GoVarRef{
			ref: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "object_creation_expression":
		// TODO: properly initialize objects here
		return &GoNil, nil
	case "field_access":
		return &GoVarRef{
			ref: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "method_invocation":
		methodNameNode := expression.ChildByFieldName("name")
		methodName := methodNameNode.Utf8Text(ctx.javaSource)
		argListNode := expression.ChildByFieldName("arguments")
		argExps := convertArgumentList(ctx, argListNode)
		return &GoCallExpression{
			function: methodName,
			args:     argExps,
		}, nil
	case "return":
		var initStmts []GoStatement
		var value GoExpression
		if expression.ChildCount() == 1 {
			value, initStmts = convertExpression(ctx, expression.Child(0))
		}
		return &GoReturnExpression{
			value: value,
		}, initStmts
	case "parenthesized_expression":
		return convertExpression(ctx, expression.Child(1))
	case "binary_expression":
		leftNode := expression.ChildByFieldName("left")
		left, leftInit := convertExpression(ctx, leftNode)
		rightNode := expression.ChildByFieldName("right")
		rigth, rightInit := convertExpression(ctx, rightNode)
		stms := append(leftInit, rightInit...)
		var operator string
		iterateChilden(expression, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "||", "&&", "==", "!=", "<", "<=", ">", ">=", "+", "-", "*", "/", "%":
				operator = child.Utf8Text(ctx.javaSource)
			}
		})
		Assert("binary expression operator not found", operator != "")
		return &BinaryExpression{
			left:     left,
			operator: operator,
			right:    rigth,
		}, stms
	case "character_literal":
		return &StringLiteral{
			value: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "true":
		return &BooleanLiteral{
			value: true,
		}, nil
	case "false":
		return &BooleanLiteral{
			value: false,
		}, nil
	case "decimal_integer_literal":
		n, err := strconv.ParseInt(expression.Utf8Text(ctx.javaSource), 10, 64)
		if err != nil {
			Fatal(expression.ToSexp(), err)
		}
		return &IntLiteral{
			value: int(n),
		}, nil
	case "hex_integer_literal":
		n, err := strconv.ParseInt(expression.Utf8Text(ctx.javaSource), 0, 64)
		if err != nil {
			Fatal(expression.ToSexp(), err)
		}
		return &IntLiteral{
			value: int(n),
		}, nil
	case "unary_expression":
		operandNode := expression.ChildByFieldName("operand")
		operand, initStmts := convertExpression(ctx, operandNode)
		var operator string
		iterateChilden(expression, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "!":
				operator = child.Utf8Text(ctx.javaSource)
			}
		})
		Assert("unary expression operator not found", operator != "")
		return &UnaryExpression{
			operator: operator,
			operand:  operand,
		}, initStmts
	default:
		fmt.Println(expression.Utf8Text(ctx.javaSource))
		Fatal(expression.ToSexp(), errors.New("unhandled expression kind: "+expression.Kind()))
	}
	panic("unreachable")
}

func convertParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []GoParam {
	var params []GoParam
	iterateChilden(paramsNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "formal_parameters":
			params = append(params, convertFormalParameters(ctx, child)...)
		default:
			unhandledChild(ctx, child, "parameters")
		}
	})
	return params
}

func convertFormalParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []GoParam {
	var params []GoParam
	iterateChilden(paramsNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "formal_parameter":
			typeNode := child.ChildByFieldName("type")
			if typeNode == nil {
				Fatal(child.ToSexp(), errors.New("formal_parameter missing type field"))
			}
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				Fatal(child.ToSexp(), errors.New("formal_parameter missing name field"))
			}
			ty, ok := tryParseType(ctx, typeNode)
			if !ok {
				Fatal(typeNode.ToSexp(), errors.New("unable to parse type in formal_parameter"))
			}
			params = append(params, GoParam{
				name: nameNode.Utf8Text(ctx.javaSource),
				ty:   ty,
			})
		case "spread_parameter":
			var ty GoType
			var name string
			iterateChilden(child, func(spreadChild *tree_sitter.Node) {
				// ignored
				switch spreadChild.Kind() {
				case "...":
					return
				}
				goTy, ok := tryParseType(ctx, spreadChild)
				ty = goTy
				if ok {
					return
				}
				nameNode := spreadChild.ChildByFieldName("name")
				if nameNode == nil {
					Fatal(spreadChild.ToSexp(), errors.New("spread child missing name field"))
				}
				name = nameNode.Utf8Text(ctx.javaSource)
			})
			params = append(params, GoParam{
				name: name,
				ty:   "..." + ty,
			})
		// ignored
		case "(":
		case ")":
		case ",":
		default:
			unhandledChild(ctx, child, "formal_parameters")
		}
	})
	return params
}

func convertFieldDeclaration(ctx *MigrationContext, fieldNode *tree_sitter.Node) (GoStructField, GoExpression) {
	var modifiers modifiers
	var ty GoType
	var name string
	var comments []string
	var initExpr GoExpression
	iterateChilden(fieldNode, func(child *tree_sitter.Node) {
		t, ok := tryParseType(ctx, child)
		if ok {
			ty = t
			return
		}
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "variable_declarator":
			result := convertVariableDecl(ctx, child)
			name = result.name
			initExpr = result.value
		// ignored
		case ";":
		default:
			unhandledChild(ctx, child, "field_declaration")
		}
	})
	return GoStructField{
		name:     name,
		ty:       ty,
		public:   modifiers&PUBLIC != 0,
		comments: comments,
	}, initExpr
}

type variableDeclResult struct {
	name  string
	value GoExpression
}

func convertVariableDecl(ctx *MigrationContext, declNode *tree_sitter.Node) variableDeclResult {
	var name string
	nameNode := declNode.ChildByFieldName("name")
	if nameNode != nil {
		name = nameNode.Utf8Text(ctx.javaSource)
	} else {
		Fatal(declNode.ToSexp(), errors.New("variable_declarator missing name field"))
	}
	valueNode := declNode.ChildByFieldName("value")
	if valueNode != nil {
		return variableDeclResult{
			name: name,
			value: &UnhandledExpression{
				text: valueNode.Utf8Text(ctx.javaSource),
			},
		}
	}
	return variableDeclResult{
		name: name,
	}
}

func tryParseType(ctx *MigrationContext, node *tree_sitter.Node) (GoType, bool) {
	switch node.Kind() {
	case "type_identifier":
		return GoType(node.Utf8Text(ctx.javaSource)), true
	case "integral_type":
		return GoTypeInt, true
	case "boolean_type":
		return GoTypeBool, true
	case "generic_type":
		var typeName string
		var typeParams []string
		iterateChilden(node, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "type_identifier":
				typeName = child.Utf8Text(ctx.javaSource)
			case "type_arguments":
				iterateChilden(child, func(typeArg *tree_sitter.Node) {
					if typeArg.Kind() == "type_identifier" {
						typeParams = append(typeParams, typeArg.Utf8Text(ctx.javaSource))
					}
				})
			}
		})
		switch typeName {
		// Array types
		case "ArrayDeque":
			fallthrough
		case "Collection":
			fallthrough
		case "List":
			Assert("List can have only one type param", len(typeParams) < 2)
			if len(typeParams) == 0 {
				return GoType("[]interface{}"), true
			}
			return GoType("[]" + typeParams[0]), true
		}
	}

	return "", false
}

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

func parseModifiers(source string) modifiers {
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

const (
	PUBLIC modifiers = 1 << iota
	PRIVATE
	PROTECTED
	STATIC
	FINAL
	ABSTRACT
)

func iterateChilden(node *tree_sitter.Node, fn func(child *tree_sitter.Node)) {
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		fn(&child)
	}
}

func parseJava(source []byte) *tree_sitter.Tree {
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_java.Language()))
	tree := parser.Parse(source, nil)
	return tree
}

func Fatal(msg string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Fatal: %s: %v\n", msg, err)
	os.Exit(1)
}

func unhandledChild(ctx *MigrationContext, node *tree_sitter.Node, parentName string) {
	msg := fmt.Sprintf("unhandled %s child node kind: %s\nS-expression: %s\nSource: %s",
		parentName,
		node.Kind(),
		node.ToSexp(),
		node.Utf8Text(ctx.javaSource))
	fmt.Fprintf(os.Stderr, "Fatal: %s\n", msg)
	os.Exit(1)
}

func Assert(msg string, condition bool) {
	if condition {
		return
	}
	fmt.Fprintf(os.Stderr, "Assertion failed: %s\n", msg)
	os.Exit(1)
}
