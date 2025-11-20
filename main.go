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

const SELF_REF = "this"
const PACKAGE_NAME = "converted"

type MigrationContext struct {
	source     GoSource
	javaSource []byte
}

// TODO: add constants and vars
type GoSource struct {
	imports   []Import
	structs   []Struct
	functions []Function
	methods   []Method
}

func (s *GoSource) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("package ")
	sb.WriteString(PACKAGE_NAME)
	sb.WriteString("\n\n")
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

type Import struct {
	packagePath string
	alias       *string
}

func (imp *Import) ToSource() string {
	if imp.alias != nil {
		return fmt.Sprintf("%s \"%s\"", *imp.alias, imp.packagePath)
	}
	return fmt.Sprintf("\"%s\"", imp.packagePath)
}

type Struct struct {
	name     string
	fields   []StructField
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

func (s *Struct) ToSource() string {
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

type Function struct {
	name       string
	params     []Param
	returnType *Type
	body       []Statement
	comments   []string
	public     bool
}

type Method struct {
	Function
	receiver Param
}

// TODO: this is basically same as GoFunction.ToSource, refactor
func (f *Method) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString("(")
	sb.WriteString(f.receiver.ToSource())
	sb.WriteString(") ")
	sb.WriteString(toIdentifier(f.name, f.public))
	return finishGoFunctionToSource(&sb, &f.Function)
}

func (f *Function) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString(toIdentifier(f.name, f.public))
	return finishGoFunctionToSource(&sb, f)
}

func finishGoFunctionToSource(sb *strings.Builder, f *Function) string {
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

type Param struct {
	name string
	ty   Type
}

func (p *Param) ToSource() string {
	return fmt.Sprintf("%s %s", p.name, p.ty.ToSource())
}

type StructField struct {
	name     string
	ty       Type
	public   bool
	comments []string
}

func (f *StructField) ToSource() string {
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

type Type string

const (
	TypeInt     Type = "int"
	TypeString  Type = "string"
	TypeBool    Type = "bool"
	TypeFloat64 Type = "float64"
)

func (t *Type) ToSource() string {
	return string(*t)
}

func (t *Type) isArray() bool {
	return strings.HasPrefix(string(*t), "[]")
}

type Statement interface {
	SourceElement
}

type GoStatement struct {
	source string
}

func (s *GoStatement) ToSource() string {
	return s.source
}

type IfStatement struct {
	condition Expression
	body      []Statement
}

type VarDeclaration struct {
	name  string
	ty    Type
	value Expression
}

func (s *VarDeclaration) ToSource() string {
	if s.value != nil {
		return fmt.Sprintf("%s := %s", s.name, s.value.ToSource())
	}
	return fmt.Sprintf("var %s %s", s.name, s.ty.ToSource())
}

func (s *IfStatement) ToSource() string {
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

type ReturnStatement struct {
	value Expression
}

func (s *ReturnStatement) ToSource() string {
	if s.value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", s.value.ToSource())
}

type CommentStmt struct {
	comments []string
}

func (s *CommentStmt) ToSource() string {
	sb := strings.Builder{}
	addComments(&sb, s.comments)
	return sb.String()
}

type ForStatement struct {
	init      Statement
	condition Expression
	post      Statement
	body      []Statement
}

func (s *ForStatement) ToSource() string {
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

type CallStatement struct {
	exp Expression
}

type AssignStatement struct {
	ref   VarRef
	value Expression
}

func (s *AssignStatement) ToSource() string {
	return fmt.Sprintf("%s = %s", s.ref.ToSource(), s.value.ToSource())
}

func (s *CallStatement) ToSource() string {
	return s.exp.ToSource()
}

type Expression interface {
	SourceElement
}

type CastExpression struct {
	ty    Type
	value Expression
}

func (e *CastExpression) ToSource() string {
	return fmt.Sprintf("%s(%s)", e.ty.ToSource(), e.value.ToSource())
}

type ReturnExpression struct {
	value Expression
}

func (e *ReturnExpression) ToSource() string {
	if e.value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", e.value.ToSource())
}

type CallExpression struct {
	function string
	args     []Expression
}

func (e *CallExpression) ToSource() string {
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

type VarRef struct {
	ref string
}

var NIL = VarRef{ref: "nil"}

func (e *VarRef) ToSource() string {
	return e.ref
}

type CharLiteral struct {
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

func (e *CharLiteral) ToSource() string {
	return fmt.Sprintf("%s", e.value)
}

type BinaryExpression struct {
	left     Expression
	operator string
	right    Expression
}

type UnaryExpression struct {
	operator string
	operand  Expression
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
			ctx.source.structs = append(ctx.source.structs, Struct{
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
	fields    []StructField
	comments  []string
	functions []Function
	methods   []Method
}

func convertClassBody(ctx *MigrationContext, structName string, classBody *tree_sitter.Node) classConversionResult {
	var result classConversionResult
	fieldInitValues := map[string]Expression{}
	iterateChilden(classBody, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "class_declaration":
			migrateClassDeclaration(ctx, child)
		case "field_declaration":
			// TODO: if the field is static make it a var
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
			function, isStatic := convertMethodDeclaration(ctx, child)
			if isStatic {
				result.functions = append(result.functions, function)
			} else {
				result.methods = append(result.methods, Method{
					Function: function,
					receiver: Param{
						name: SELF_REF,
						ty:   Type("*" + structName),
					},
				})
			}
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
func convertMethodDeclaration(ctx *MigrationContext, methodNode *tree_sitter.Node) (Function, bool) {
	var modifiers modifiers
	var params []Param
	var body []Statement
	var name string
	var returnType *Type
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
	return Function{
		name:       name,
		params:     params,
		returnType: returnType,
		body:       body,
		public:     modifiers&PUBLIC != 0,
	}, modifiers&STATIC != 0
}

func convertStatementBlock(ctx *MigrationContext, blockNode *tree_sitter.Node) []Statement {
	var body []Statement
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

func convertConstructor(ctx *MigrationContext, fieldInitValues *map[string]Expression, structName string, constructorNode *tree_sitter.Node) Function {
	var modifiers modifiers
	var params []Param
	var body []Statement
	body = append(body, &GoStatement{source: fmt.Sprintf("%s := %s{};", SELF_REF, structName)})
	iterateChilden(constructorNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "constructor_body":
			body = append(body, convertConstructorBody(ctx, fieldInitValues, structName, child)...)
		// ignored
		case "identifier":
		default:
			unhandledChild(ctx, child, "constructor_declaration")
		}
	})
	body = append(body, &ReturnStatement{value: &VarRef{ref: SELF_REF}})
	nameBuilder := strings.Builder{}
	nameBuilder.WriteString(toIdentifier("new", modifiers.isPublic()))
	nameBuilder.WriteString(capitalizeFirstLetter(structName))
	nameBuilder.WriteString("From")
	for _, param := range params {
		nameBuilder.WriteString(capitalizeFirstLetter(param.name))
	}
	name := nameBuilder.String()
	retTy := Type(structName)
	return Function{
		name:       name,
		params:     params,
		returnType: &retTy,
		body:       body,
		public:     modifiers&PUBLIC != 0,
	}
}

func convertConstructorBody(ctx *MigrationContext, fieldInitValues *map[string]Expression, structName string, bodyNode *tree_sitter.Node) []Statement {
	var body []Statement
	for fieldName, initExpr := range *fieldInitValues {
		body = append(body, &AssignStatement{ref: VarRef{ref: SELF_REF + "." + fieldName}, value: initExpr})
	}
	if len(*fieldInitValues) > 0 {
		body = append(body, &CommentStmt{comments: []string{"Default field initializations"}})
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

func convertStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []Statement {
	switch stmtNode.Kind() {
	case "line_comment":
		return nil
	case "expression_statement":
		var body []Statement
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "assignment_expression":
				refNode := child.ChildByFieldName("left")
				ref := VarRef{ref: refNode.Utf8Text(ctx.javaSource)}
				valueNode := child.ChildByFieldName("right")
				valueExp, initStmts := convertExpression(ctx, valueNode)
				if len(initStmts) > 0 {
					Fatal(valueNode.ToSexp(), errors.New("unexpected statements in assignment expression"))
				}
				body = append(body, &AssignStatement{
					ref:   ref,
					value: valueExp,
				})
			case "method_invocation":
				callExperession, initStmts := convertExpression(ctx, child)
				body = append(body, initStmts...)
				body = append(body, &CallStatement{exp: callExperession})
			//ignored
			case ";":
			default:
				unhandledChild(ctx, child, "expression_statement")
			}
		})
		return body
	case "return_statement":
		var initialStmts []Statement
		var value Expression
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case ";":
			case "return":
			default:
				value, initialStmts = convertExpression(ctx, child)
			}
		})
		return append(initialStmts, &ReturnStatement{value: value})
	case "if_statement":
		conditionNode := stmtNode.ChildByFieldName("condition")
		conditionExp, stmts := convertExpression(ctx, conditionNode)
		bodyNode := stmtNode.ChildByFieldName("consequence")
		bodyStmts := convertStatementBlock(ctx, bodyNode)
		return append(stmts, &IfStatement{
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
			return []Statement{
				&VarDeclaration{
					name: name,
					ty:   ty,
				}}
		}
		valueExpr, initStmts := convertExpression(ctx, valueNode)
		return append(initStmts, &VarDeclaration{
			name:  name,
			ty:    ty,
			value: valueExpr,
		})
	case "while_statement":
		conditionNode := stmtNode.ChildByFieldName("condition")
		conditionExp, initStmts := convertExpression(ctx, conditionNode)
		bodyNode := stmtNode.ChildByFieldName("body")
		bodyStmts := convertStatementBlock(ctx, bodyNode)
		return append(initStmts, &ForStatement{
			condition: conditionExp,
			body:      bodyStmts,
		})
	case "throw_statement":
		valueNode := stmtNode.Child(1)
		valueExp, initStmts := convertExpression(ctx, valueNode)
		return append(initStmts, &ReturnStatement{
			value: valueExp,
		})
	default:
		unhandledChild(ctx, stmtNode, "statement")
	}
	panic("unreachable")
}

func convertExplicitConstructorInvocation(ctx *MigrationContext, invocationNode *tree_sitter.Node) []Statement {
	parentCall := "this"
	var argExp []Expression
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
	return []Statement{
		&CallStatement{
			exp: &CallExpression{
				function: parentCall,
				args:     argExp,
			}}}
}

func convertArgumentList(ctx *MigrationContext, argList *tree_sitter.Node) []Expression {
	var args []Expression
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

func convertExpression(ctx *MigrationContext, expression *tree_sitter.Node) (Expression, []Statement) {
	switch expression.Kind() {
	case "identifier":
		return &VarRef{
			ref: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "object_creation_expression":
		ty, isType := tryParseType(ctx, expression.ChildByFieldName("type"))
		if !isType {
			Fatal(expression.ToSexp(), errors.New("unable to parse type in object_creation_expression"))
		}
		if ty.isArray() {
			return &GoStatement{
				source: fmt.Sprintf("make(%s, 0)", ty),
			}, nil
		}
		// TODO: properly initialize objects here
		return &NIL, nil
	case "field_access":
		return &VarRef{
			ref: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "method_invocation":
		methodNameNode := expression.ChildByFieldName("object")
		methodName := methodNameNode.Utf8Text(ctx.javaSource)
		argListNode := expression.ChildByFieldName("arguments")
		argExps := convertArgumentList(ctx, argListNode)
		return &CallExpression{
			function: SELF_REF + "." + methodName,
			args:     argExps,
		}, nil
	case "return":
		var initStmts []Statement
		var value Expression
		if expression.ChildCount() == 1 {
			value, initStmts = convertExpression(ctx, expression.Child(0))
		}
		return &ReturnExpression{
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
		return &CharLiteral{
			value: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "null_literal":
		return &NIL, nil
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
	case "cast_expression":
		typeNode := expression.ChildByFieldName("type")
		ty, ok := tryParseType(ctx, typeNode)
		if !ok {
			Fatal(typeNode.ToSexp(), errors.New("unable to parse type in cast_expression"))
		}
		valueNode := expression.ChildByFieldName("value")
		valueExp, initStmts := convertExpression(ctx, valueNode)
		return &CastExpression{
			ty:    ty,
			value: valueExp,
		}, initStmts
	default:
		fmt.Println(expression.Utf8Text(ctx.javaSource))
		Fatal(expression.ToSexp(), errors.New("unhandled expression kind: "+expression.Kind()))
	}
	panic("unreachable")
}

func convertParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []Param {
	var params []Param
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

func convertFormalParameters(ctx *MigrationContext, paramsNode *tree_sitter.Node) []Param {
	var params []Param
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
			params = append(params, Param{
				name: nameNode.Utf8Text(ctx.javaSource),
				ty:   ty,
			})
		case "spread_parameter":
			var ty Type
			var name string
			iterateChilden(child, func(spreadChild *tree_sitter.Node) {
				switch spreadChild.Kind() {
				case "variable_declarator":
					nameNode := spreadChild.ChildByFieldName("name")
					if nameNode == nil {
						Fatal(spreadChild.ToSexp(), errors.New("spread child missing name field"))
					}
					name = nameNode.Utf8Text(ctx.javaSource)
				case "...":
					return
				default:
					goTy, ok := tryParseType(ctx, spreadChild)
					ty = goTy
					if ok {
						return
					}
				}
			})
			params = append(params, Param{
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

func convertFieldDeclaration(ctx *MigrationContext, fieldNode *tree_sitter.Node) (StructField, Expression) {
	var modifiers modifiers
	var ty Type
	var name string
	var comments []string
	var initExpr Expression
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
	return StructField{
		name:     name,
		ty:       ty,
		public:   modifiers&PUBLIC != 0,
		comments: comments,
	}, initExpr
}

type variableDeclResult struct {
	name  string
	value Expression
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
		value, init := convertExpression(ctx, valueNode)
		Assert("unexpected statements in variable declaration", len(init) == 0)
		return variableDeclResult{
			name:  name,
			value: value,
		}
	}
	return variableDeclResult{
		name: name,
	}
}

func tryParseType(ctx *MigrationContext, node *tree_sitter.Node) (Type, bool) {
	switch node.Kind() {
	case "type_identifier":
		var goType string
		typeName := node.Utf8Text(ctx.javaSource)
		if strings.HasPrefix(typeName, "Abstract") {
			goType = typeName[len("Abstract"):]
			return Type(goType), true
		}
		if strings.HasPrefix(typeName, "ST") {
			goType = "internal." + typeName
			return Type(goType), true
		}
		switch typeName {
		case "Object":
			goType = "interface{}"
		case "DiagnosticCode":
			goType = "diagnostics.DiagnosticCode"
		default:
			goType = typeName
		}
		return Type(goType), true
	case "integral_type":
		return TypeInt, true
	case "boolean_type":
		return TypeBool, true
	case "array_type":
		typeNode := node.ChildByFieldName("element")
		ty, ok := tryParseType(ctx, typeNode)
		if !ok {
			Fatal(typeNode.ToSexp(), errors.New("unable to parse element type in array_type"))
		}
		return Type("[]" + ty), true
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
		case "Deque":
			fallthrough
		case "Collection":
			fallthrough
		case "ArrayList":
			fallthrough
		case "List":
			Assert("List can have only one type param", len(typeParams) < 2)
			if len(typeParams) == 0 {
				return Type("[]interface{}"), true
			}
			return Type("[]" + typeParams[0]), true
		default:
			Fatal(node.ToSexp(), errors.New("unhandled generic type : "+typeName))
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
