package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

const (
	SELF_REF     = "this"
	PACKAGE_NAME = "converted"
)

type Config struct {
	PackageName   string `toml:"package_name"`
	LicenseHeader string `toml:"license_header"`
}

type MigrationContext struct {
	source            GoSource
	javaSource        []byte
	inReturn          bool
	abstractClasses   map[string]bool
	inDefaultMethod   bool
	defaultMethodSelf string
	enumConstants     map[string]string // Maps enum constant name to prefixed name (e.g., "ACTIVE" -> "Status_ACTIVE")
}

type GoSource struct {
	imports     []Import
	interfaces  []Interface
	structs     []Struct
	constants   []ModuleConst
	constBlocks []ConstBlock
	vars        []ModuleVar
	functions   []Function
	methods     []Method
}

type ModuleConst struct {
	name  string
	ty    Type
	value Expression
}

func (c *ModuleConst) ToSource() string {
	if c.value != nil {
		return fmt.Sprintf("const %s %s = %s", c.name, c.ty.ToSource(), c.value.ToSource())
	}
	if c.ty != "" {
		return fmt.Sprintf("const %s %s", c.name, c.ty.ToSource())
	}
	return fmt.Sprintf("const %s", c.name)
}

type ConstBlock struct {
	typeName  string
	constants []string
}

func (cb *ConstBlock) ToSource() string {
	if len(cb.constants) == 0 {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("const (\n")
	for i, constName := range cb.constants {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    %s %s = iota\n", constName, cb.typeName))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", constName))
		}
	}
	sb.WriteString(")\n")
	return sb.String()
}

type ModuleVar struct {
	name  string
	ty    Type
	value Expression
}

func (v *ModuleVar) ToSource() string {
	if v.value != nil {
		// If name is "_" and both type and value are present, include type annotation (needed for type assertions)
		// Otherwise, use type inference (existing behavior for regular vars)
		if v.name == "_" && v.ty != "" {
			return fmt.Sprintf("var %s %s = %s", v.name, v.ty.ToSource(), v.value.ToSource())
		}
		return fmt.Sprintf("var %s = %s", v.name, v.value.ToSource())
	}
	return fmt.Sprintf("var %s %s", v.name, v.ty.ToSource())
}

func (s *GoSource) ToSource(config Config) string {
	sb := strings.Builder{}
	if config.LicenseHeader != "" {
		sb.WriteString(config.LicenseHeader)
		if !strings.HasSuffix(config.LicenseHeader, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("package ")
	sb.WriteString(config.PackageName)
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
	for _, iface := range s.interfaces {
		sb.WriteString(iface.ToSource())
		sb.WriteString("\n")
	}
	for _, strct := range s.structs {
		sb.WriteString(strct.ToSource())
		sb.WriteString("\n")
	}
	for _, cb := range s.constBlocks {
		sb.WriteString(cb.ToSource())
		sb.WriteString("\n")
	}
	for _, c := range s.constants {
		sb.WriteString(c.ToSource())
		sb.WriteString("\n")
	}
	for _, v := range s.vars {
		sb.WriteString(v.ToSource())
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

type Interface struct {
	name     string
	embeds   []Type
	methods  []InterfaceMethod
	public   bool
	comments []string
}

type InterfaceMethod struct {
	name       string
	params     []Param
	returnType *Type
	public     bool
}

func (i *Interface) ToSource() string {
	sb := strings.Builder{}
	addComments(&sb, i.comments)
	sb.WriteString("type ")
	sb.WriteString(toIdentifier(i.name, i.public))
	sb.WriteString(" interface {\n")
	for _, embed := range i.embeds {
		sb.WriteString("    ")
		sb.WriteString(embed.ToSource())
		sb.WriteString("\n")
	}
	for _, method := range i.methods {
		sb.WriteString("    ")
		sb.WriteString(toIdentifier(method.name, method.public))
		sb.WriteString("(")
		for j, param := range method.params {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(param.ToSource())
		}
		sb.WriteString(")")
		if method.returnType != nil {
			sb.WriteString(" ")
			sb.WriteString(method.returnType.ToSource())
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

type Struct struct {
	name     string
	includes []Type
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
	// Check if this is a type alias (empty fields and comment starting with "type")
	if len(s.fields) == 0 && len(s.includes) == 0 && len(s.comments) > 0 {
		// Check if first comment is a type alias declaration
		firstComment := strings.TrimSpace(s.comments[0])
		if strings.HasPrefix(firstComment, "type ") {
			// Output as type alias (skip the comment, output directly)
			sb.WriteString("type ")
			sb.WriteString(toIdentifier(s.name, s.public))
			// Extract the type from comment: "type EnumName int" -> "int"
			parts := strings.Fields(firstComment)
			if len(parts) >= 3 {
				sb.WriteString(" ")
				sb.WriteString(parts[2])
			}
			sb.WriteString("\n")
			return sb.String()
		}
	}
	addComments(&sb, s.comments)
	sb.WriteString("type ")
	sb.WriteString(toIdentifier(s.name, s.public))
	sb.WriteString(" struct {\n")
	for _, include := range s.includes {
		sb.WriteString("    ")
		sb.WriteString(include.ToSource())
		sb.WriteString("\n")
	}
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

type SwitchStatement struct {
	condition   Expression
	cases       []SwitchCase
	defaultBody []Statement
}

type SwitchCase struct {
	condition Expression
	body      []Statement
}

func (s *SwitchStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("switch ")
	sb.WriteString(s.condition.ToSource())
	sb.WriteString(" {\n")
	for _, cs := range s.cases {
		conditionStr := cs.condition.ToSource()
		if conditionStr == "default" {
			sb.WriteString("default:\n")
			for _, stmt := range cs.body {
				sb.WriteString(stmt.ToSource())
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("case ")
			if strings.HasPrefix(conditionStr, "case ") {
				conditionStr = strings.TrimPrefix(conditionStr, "case ")
			}
			sb.WriteString(conditionStr)
			if len(cs.body) > 0 {
				sb.WriteString(":\n")
				for _, stmt := range cs.body {
					sb.WriteString(stmt.ToSource())
					sb.WriteString("\n")
				}
			} else {
				sb.WriteString(",\n")
			}
		}
	}
	if len(s.defaultBody) > 0 {
		sb.WriteString("default:\n")
		for _, stmt := range s.defaultBody {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
	}
	sb.WriteString("}")
	return sb.String()
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
	elseIf    []IfStatement
	elseStmts []Statement
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

func (s *IfStatement) writeElseIfChain(sb *strings.Builder, elseIfs []IfStatement) {
	for _, elseIf := range elseIfs {
		sb.WriteString("else if ")
		sb.WriteString(elseIf.condition.ToSource())
		sb.WriteString(" {\n")
		for _, stmt := range elseIf.body {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString("}")
		// Recursively handle nested else-if chains
		s.writeElseIfChain(sb, elseIf.elseIf)
		// Handle the final else block at this level
		if len(elseIf.elseStmts) > 0 {
			sb.WriteString("else {\n")
			for _, stmt := range elseIf.elseStmts {
				sb.WriteString(stmt.ToSource())
				sb.WriteString("\n")
			}
			sb.WriteString("}")
		}
	}
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
	// Write all else-if chains recursively
	s.writeElseIfChain(&sb, s.elseIf)
	// Handle the final else block at the top level
	if len(s.elseStmts) > 0 {
		sb.WriteString("else {\n")
		for _, stmt := range s.elseStmts {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString("}")
	}
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

type RangeForStatement struct {
	indexVar       string
	valueVar       string
	collectionExpr Expression
	body           []Statement
}

func (s *RangeForStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("for ")
	if s.indexVar != "" {
		sb.WriteString(s.indexVar)
	} else {
		sb.WriteString("_")
	}
	sb.WriteString(", ")
	if s.valueVar != "" {
		sb.WriteString(s.valueVar)
	} else {
		sb.WriteString("_")
	}
	sb.WriteString(" := range ")
	sb.WriteString(s.collectionExpr.ToSource())
	sb.WriteString(" {\n")
	for _, stmt := range s.body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
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

type CatchClause struct {
	exceptionType string
	exceptionVar  string
	body          []Statement
}

type TryStatement struct {
	tryBody      []Statement
	catchClauses []CatchClause
	finallyBody  []Statement
}

func (s *TryStatement) ToSource() string {
	sb := strings.Builder{}
	// Wrap try body in an IIFE with defer/recover
	sb.WriteString("func() {\n")
	// Add defer with recover
	sb.WriteString("    defer func() {\n")
	sb.WriteString("        if r := recover(); r != nil {\n")
	// Handle catch clauses
	if len(s.catchClauses) > 0 {
		for i, catch := range s.catchClauses {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("            if _, ok := r.(%s); ok {\n", catch.exceptionType))
			} else {
				sb.WriteString(fmt.Sprintf("            } else if _, ok := r.(%s); ok {\n", catch.exceptionType))
			}
			// Write catch body
			for _, stmt := range catch.body {
				stmtSource := stmt.ToSource()
				// Indent each line
				lines := strings.Split(stmtSource, "\n")
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						sb.WriteString("                ")
						sb.WriteString(line)
						sb.WriteString("\n")
					}
				}
			}
		}
		sb.WriteString("            } else {\n")
		sb.WriteString("                panic(r) // re-panic if it's not a handled exception\n")
		sb.WriteString("            }\n")
	} else {
		// No catch clauses, just re-panic
		sb.WriteString("            panic(r)\n")
	}
	sb.WriteString("        }\n")
	sb.WriteString("    }()\n")
	// Write try body
	for _, stmt := range s.tryBody {
		stmtSource := stmt.ToSource()
		// Indent each line
		lines := strings.Split(stmtSource, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				sb.WriteString("    ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}
	sb.WriteString("}()\n")
	// Write finally block if present
	if len(s.finallyBody) > 0 {
		for _, stmt := range s.finallyBody {
			stmtSource := stmt.ToSource()
			lines := strings.Split(stmtSource, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					sb.WriteString(line)
					sb.WriteString("\n")
				}
			}
		}
	}
	return sb.String()
}

type Expression interface {
	SourceElement
}

type GoExpression struct {
	source string
}

func (e *GoExpression) ToSource() string {
	return e.source
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

type ArrayLiteral struct {
	elementType Type
	elements    []Expression
}

func (e *ArrayLiteral) ToSource() string {
	sb := strings.Builder{}
	// Ensure elementType has [] prefix for slice literals
	elementTypeStr := e.elementType.ToSource()
	if !strings.HasPrefix(elementTypeStr, "[]") {
		sb.WriteString("[]")
	}
	sb.WriteString(elementTypeStr)
	sb.WriteString("{")
	for i, elem := range e.elements {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(elem.ToSource())
	}
	sb.WriteString("}")
	return sb.String()
}

type SourceElement interface {
	ToSource() string
}

func loadConfig() Config {
	config := Config{
		PackageName:   PACKAGE_NAME,
		LicenseHeader: "",
	}

	wd, err := os.Getwd()
	if err != nil {
		return config
	}

	configPath := filepath.Join(wd, "Config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file doesn't exist, return defaults
		return config
	}

	var fileConfig Config
	if err := toml.Unmarshal(data, &fileConfig); err != nil {
		// Invalid TOML, return defaults
		return config
	}

	// Use values from file if provided, otherwise keep defaults
	if fileConfig.PackageName != "" {
		config.PackageName = fileConfig.PackageName
	}
	if fileConfig.LicenseHeader != "" {
		config.LicenseHeader = fileConfig.LicenseHeader
	}

	return config
}

func main() {
	config := loadConfig()
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
		javaSource:      javaSource,
		abstractClasses: make(map[string]bool),
		enumConstants:   make(map[string]string),
	}
	migrateTree(ctx, tree)
	goSource := ctx.source.ToSource(config)
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
	case "record_declaration":
		migrateRecordDeclaration(ctx, node)
	case "interface_declaration":
		migrateInterfaceDeclaration(ctx, node)
	case "enum_declaration":
		migrateEnumDeclaration(ctx, node)
	// Ignored
	case "block_comment":
	case "line_comment":
	case "package_declaration":
	case "import_declaration":
	default:
		unhandledChild(ctx, node, "<root>")
	}
}

func migrateClassDeclaration(ctx *MigrationContext, classNode *tree_sitter.Node) {
	var className string
	var modifiers modifiers
	var includes []Type
	var implementedInterfaces []Type
	isAbstract := false
	iterateChilden(classNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
			isAbstract = modifiers&ABSTRACT != 0
		case "identifier":
			className = child.Utf8Text(ctx.javaSource)
		case "superclass":
			ty, ok := tryParseType(ctx, child.Child(1))
			if ok {
				includes = append(includes, ty)
			} else {
				unhandledChild(ctx, child, "superclass")
			}
		case "super_interfaces":
			// Parse implements clause - iterate through children to find type_list
			iterateChilden(child, func(superinterfacesChild *tree_sitter.Node) {
				if superinterfacesChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					iterateChilden(superinterfacesChild, func(typeChild *tree_sitter.Node) {
						ty, ok := tryParseType(ctx, typeChild)
						if ok {
							implementedInterfaces = append(implementedInterfaces, ty)
						}
					})
				}
			})
		case "class_body":
			if isAbstract {
				ctx.abstractClasses[className] = true
				convertAbstractClass(ctx, className, modifiers, includes, child)
			} else {
				// Check if this class extends an abstract class
				var embeddedTypes []Type
				extendsAbstract := false
				for _, include := range includes {
					baseName := string(include)
					if ctx.abstractClasses[baseName] {
						// Embed FooBase and FooMethods
						embeddedTypes = append(embeddedTypes, Type(capitalizeFirstLetter(baseName)+"Base"))
						embeddedTypes = append(embeddedTypes, Type(capitalizeFirstLetter(baseName)+"Methods"))
						extendsAbstract = true
					} else {
						embeddedTypes = append(embeddedTypes, include)
					}
				}
				// Use capitalized name if extending abstract class, otherwise use toIdentifier
				structName := className
				if extendsAbstract {
					structName = capitalizeFirstLetter(className)
				} else {
					structName = toIdentifier(className, modifiers.isPublic())
				}
				result := convertClassBody(ctx, structName, child, false)
				for _, function := range result.functions {
					ctx.source.functions = append(ctx.source.functions, function)
				}
				for i := range result.methods {
					method := &result.methods[i]
					// Capitalize method names if extending abstract class
					if extendsAbstract {
						method.name = capitalizeFirstLetter(method.name)
						method.public = true
						// Update receiver type to use capitalized struct name
						method.receiver.ty = Type("*" + structName)
						// Use single lowercase letter for receiver name (Go convention: first letter of type)
						receiverName := strings.ToLower(string(structName[0]))
						method.receiver.name = receiverName
					}
					ctx.source.methods = append(ctx.source.methods, *method)
				}
				ctx.source.structs = append(ctx.source.structs, Struct{
					name:     structName,
					fields:   result.fields,
					comments: result.comments,
					public:   extendsAbstract || (modifiers&PUBLIC != 0),
					includes: embeddedTypes,
				})
				// Generate type assertions for implemented interfaces
				for _, ifaceType := range implementedInterfaces {
					// Create type assertion: var _ InterfaceName = &StructName{}
					ctx.source.vars = append(ctx.source.vars, ModuleVar{
						name:  "_",
						ty:    ifaceType,
						value: &VarRef{ref: "&" + structName + "{}"},
					})
				}
			}
		// ignored
		case "class":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "class_declaration")
		}
	})
}

func migrateRecordDeclaration(ctx *MigrationContext, recordNode *tree_sitter.Node) {
	var recordName string
	var modifiers modifiers
	var fields []StructField
	var comments []string
	var implementedInterfaces []Type

	iterateChilden(recordNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "identifier":
			recordName = child.Utf8Text(ctx.javaSource)
		case "super_interfaces":
			// Parse implements clause - iterate through children to find type_list
			iterateChilden(child, func(superinterfacesChild *tree_sitter.Node) {
				if superinterfacesChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					iterateChilden(superinterfacesChild, func(typeChild *tree_sitter.Node) {
						ty, ok := tryParseType(ctx, typeChild)
						if ok {
							implementedInterfaces = append(implementedInterfaces, ty)
						}
					})
				}
			})
		case "formal_parameters":
			// Record components are in formal_parameters
			iterateChilden(child, func(paramChild *tree_sitter.Node) {
				switch paramChild.Kind() {
				case "formal_parameter":
					typeNode := paramChild.ChildByFieldName("type")
					if typeNode == nil {
						Fatal(paramChild.ToSexp(), errors.New("formal_parameter missing type field"))
					}
					nameNode := paramChild.ChildByFieldName("name")
					if nameNode == nil {
						Fatal(paramChild.ToSexp(), errors.New("formal_parameter missing name field"))
					}
					ty, ok := tryParseType(ctx, typeNode)
					if !ok {
						Fatal(typeNode.ToSexp(), errors.New("unable to parse type in formal_parameter"))
					}
					// For record fields, we don't convert arrays to pointers (unlike function parameters)
					// Record fields should be slices directly
					fieldName := nameNode.Utf8Text(ctx.javaSource)
					fields = append(fields, StructField{
						name:     fieldName,
						ty:       ty,
						public:   true, // All record fields must be public
						comments: []string{},
					})
				// ignored
				case "(":
				case ")":
				case ",":
				case "line_comment":
				case "block_comment":
				default:
					unhandledChild(ctx, paramChild, "record formal_parameters")
				}
			})
		case "class_body":
			// Handle optional record body with additional methods/fields
			// Build field name mapping: original component name -> struct field name
			fieldNameMap := make(map[string]string)
			for _, field := range fields {
				originalName := field.name
				structFieldName := toIdentifier(field.name, true) // Always public for records
				fieldNameMap[originalName] = structFieldName
			}
			// Extract compact constructor before processing class body
			var compactConstructorNode *tree_sitter.Node
			iterateChilden(child, func(bodyChild *tree_sitter.Node) {
				if bodyChild.Kind() == "compact_constructor_declaration" {
					compactConstructorNode = bodyChild
				}
			})
			// Convert compact constructor if present
			if compactConstructorNode != nil {
				structName := toIdentifier(recordName, modifiers.isPublic())
				compactConstructor := convertCompactConstructor(ctx, fields, structName, compactConstructorNode)
				ctx.source.functions = append(ctx.source.functions, compactConstructor)
			}
			result := convertClassBody(ctx, recordName, child, false)
			// Add any additional fields from the body
			fields = append(fields, result.fields...)
			// Add methods with the record as receiver, converting field references
			structName := toIdentifier(recordName, modifiers.isPublic())
			for i := range result.methods {
				method := &result.methods[i]
				method.receiver = Param{
					name: SELF_REF,
					ty:   Type("*" + structName),
				}
				// Convert method body to use struct field names
				method.body = convertMethodBodyForRecord(ctx, method.body, fieldNameMap)
				ctx.source.methods = append(ctx.source.methods, *method)
			}
			// Add any functions (static methods)
			for _, function := range result.functions {
				ctx.source.functions = append(ctx.source.functions, function)
			}
			// Note: Nested class_declaration and record_declaration are handled by convertClassBody
		// ignored
		case "record":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "record_declaration")
		}
	})

	// Create the struct with record components as fields
	structName := toIdentifier(recordName, modifiers.isPublic())
	ctx.source.structs = append(ctx.source.structs, Struct{
		name:     structName,
		fields:   fields,
		comments: comments,
		public:   modifiers&PUBLIC != 0,
		includes: []Type{}, // Records don't support extends, only implements
	})

	// Generate type assertions for implemented interfaces
	for _, ifaceType := range implementedInterfaces {
		// Create type assertion: var _ InterfaceName = &StructName{}
		ctx.source.vars = append(ctx.source.vars, ModuleVar{
			name:  "_",
			ty:    ifaceType,
			value: &VarRef{ref: "&" + structName + "{}"},
		})
	}
}

func migrateInterfaceDeclaration(ctx *MigrationContext, interfaceNode *tree_sitter.Node) {
	var interfaceName string
	var superInterfaces []Type
	var regularMethods []InterfaceMethod
	var defaultMethods []Function
	var staticMethods []Function

	iterateChilden(interfaceNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			// Interfaces are always public, so we don't need to parse modifiers
		case "identifier":
			interfaceName = child.Utf8Text(ctx.javaSource)
		case "extends_interfaces":
			// Parse extends clause - iterate through children to find type_list
			iterateChilden(child, func(extendsChild *tree_sitter.Node) {
				if extendsChild.Kind() == "type_list" {
					// Iterate through the type_list to get individual types
					iterateChilden(extendsChild, func(typeChild *tree_sitter.Node) {
						ty, ok := tryParseType(ctx, typeChild)
						if ok {
							superInterfaces = append(superInterfaces, ty)
						}
					})
				}
			})
		case "interface_body":
			// Parse methods in interface body
			iterateChilden(child, func(bodyChild *tree_sitter.Node) {
				switch bodyChild.Kind() {
				case "class_declaration":
					migrateClassDeclaration(ctx, bodyChild)
				case "record_declaration":
					migrateRecordDeclaration(ctx, bodyChild)
				case "enum_declaration":
					migrateEnumDeclaration(ctx, bodyChild)
				case "method_declaration":
					isDefault := hasModifier(ctx, bodyChild, "default")
					isStatic := hasModifier(ctx, bodyChild, "static")

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
					unhandledChild(ctx, bodyChild, "interface_body")
				}
			})
		// ignored
		case "interface":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "interface_declaration")
		}
	})

	// Generate Go interface with regular methods
	goInterface := Interface{
		name:     capitalizeFirstLetter(interfaceName),
		embeds:   superInterfaces,
		methods:  regularMethods,
		public:   true, // Java interfaces are always public
		comments: []string{},
	}
	ctx.source.interfaces = append(ctx.source.interfaces, goInterface)

	// Generate standalone functions for default methods
	for _, defaultMethod := range defaultMethods {
		ctx.source.functions = append(ctx.source.functions, defaultMethod)
	}

	// Generate package-level functions for static methods
	for _, staticMethod := range staticMethods {
		ctx.source.functions = append(ctx.source.functions, staticMethod)
	}
}

type EnumConstant struct {
	name      string
	arguments []Expression
}

func migrateEnumDeclaration(ctx *MigrationContext, enumNode *tree_sitter.Node) {
	var enumName string
	var modifiers modifiers
	var enumConstants []EnumConstant
	var enumBody *tree_sitter.Node
	var hasFields bool

	iterateChilden(enumNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "identifier":
			enumName = child.Utf8Text(ctx.javaSource)
		case "enum_constants":
			// Parse enum constants list
			iterateChilden(child, func(constantChild *tree_sitter.Node) {
				if constantChild.Kind() == "enum_constant" {
					// Extract constant name
					constantNameNode := constantChild.ChildByFieldName("name")
					if constantNameNode != nil {
						constantName := constantNameNode.Utf8Text(ctx.javaSource)
						// Parse constructor arguments if present
						var args []Expression
						argsNode := constantChild.ChildByFieldName("arguments")
						if argsNode != nil {
							args = convertArgumentList(ctx, argsNode)
						}
						enumConstants = append(enumConstants, EnumConstant{
							name:      constantName,
							arguments: args,
						})
					}
				} else if constantChild.Kind() == "identifier" {
					// Might be a constant name directly
					constantName := constantChild.Utf8Text(ctx.javaSource)
					enumConstants = append(enumConstants, EnumConstant{
						name:      constantName,
						arguments: []Expression{},
					})
				}
			})
		case "enum_constant":
			// Handle enum constant as direct child (fallback)
			constantNameNode := child.ChildByFieldName("name")
			if constantNameNode != nil {
				constantName := constantNameNode.Utf8Text(ctx.javaSource)
				var args []Expression
				argsNode := child.ChildByFieldName("arguments")
				if argsNode != nil {
					args = convertArgumentList(ctx, argsNode)
				}
				enumConstants = append(enumConstants, EnumConstant{
					name:      constantName,
					arguments: args,
				})
			} else {
				// Try to get name from identifier child
				iterateChilden(child, func(constantChild *tree_sitter.Node) {
					if constantChild.Kind() == "identifier" {
						constantName := constantChild.Utf8Text(ctx.javaSource)
						enumConstants = append(enumConstants, EnumConstant{
							name:      constantName,
							arguments: []Expression{},
						})
					}
				})
			}
		case "enum_body":
			enumBody = child
			// Parse constants and check for fields in the body
			iterateChilden(child, func(bodyChild *tree_sitter.Node) {
				if bodyChild.Kind() == "field_declaration" {
					hasFields = true
				} else if bodyChild.Kind() == "enum_constant" {
					// Constants might be in the body
					constantNameNode := bodyChild.ChildByFieldName("name")
					if constantNameNode != nil {
						constantName := constantNameNode.Utf8Text(ctx.javaSource)
						var args []Expression
						argsNode := bodyChild.ChildByFieldName("arguments")
						if argsNode != nil {
							args = convertArgumentList(ctx, argsNode)
						}
						enumConstants = append(enumConstants, EnumConstant{
							name:      constantName,
							arguments: args,
						})
					}
				} else if bodyChild.Kind() == "identifier" && len(enumConstants) == 0 {
					// Might be a constant name if we haven't found any constants yet
					constantName := bodyChild.Utf8Text(ctx.javaSource)
					enumConstants = append(enumConstants, EnumConstant{
						name:      constantName,
						arguments: []Expression{},
					})
				}
				// Also check nested nodes for field_declaration (in case of nested structures)
				iterateChilden(bodyChild, func(nestedChild *tree_sitter.Node) {
					if nestedChild.Kind() == "field_declaration" {
						hasFields = true
					}
				})
			})
		case "class_body":
			// Enum body might be represented as class_body
			if enumBody == nil {
				enumBody = child
				// Parse constants and check for fields in the body
				iterateChilden(child, func(bodyChild *tree_sitter.Node) {
					if bodyChild.Kind() == "field_declaration" {
						hasFields = true
					} else if bodyChild.Kind() == "enum_constant" {
						constantNameNode := bodyChild.ChildByFieldName("name")
						if constantNameNode != nil {
							constantName := constantNameNode.Utf8Text(ctx.javaSource)
							var args []Expression
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
				iterateChilden(child, func(bodyChild *tree_sitter.Node) {
					if bodyChild.Kind() == "field_declaration" {
						hasFields = true
					} else if bodyChild.Kind() == "enum_constant" {
						constantNameNode := bodyChild.ChildByFieldName("name")
						if constantNameNode != nil {
							constantName := constantNameNode.Utf8Text(ctx.javaSource)
							var args []Expression
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
			unhandledChild(ctx, child, "enum_declaration")
		}
	})

	// Enums are public by default in Java (unless explicitly private/protected)
	// If no access modifier is present, default to public
	isPublic := modifiers.isPublic()
	hasAccessModifier := (modifiers&PUBLIC != 0) || (modifiers&PRIVATE != 0) || (modifiers&PROTECTED != 0)
	if !hasAccessModifier {
		isPublic = true
	}
	enumTypeName := toIdentifier(enumName, isPublic)

	// Re-check for fields in enum body if we have one (fields might come after constants)
	if enumBody != nil && !hasFields {
		iterateChilden(enumBody, func(bodyChild *tree_sitter.Node) {
			if bodyChild.Kind() == "field_declaration" {
				hasFields = true
			}
		})
	}

	// Recalculate enumTypeName with correct public flag if needed
	if !hasAccessModifier {
		enumTypeName = toIdentifier(enumName, isPublic)
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
	ctx.source.structs = append(ctx.source.structs, Struct{
		name:     enumTypeName,
		fields:   []StructField{},
		comments: []string{fmt.Sprintf("type %s uint", enumTypeName)},
		public:   isPublic,
		includes: []Type{},
	})

	// Generate const block with iota
	if len(enumConstants) > 0 {
		prefixedConstants := make([]string, len(enumConstants))
		for i, constant := range enumConstants {
			prefixedName := enumTypeName + "_" + constant.name
			prefixedConstants[i] = prefixedName
			// Track enum constant for later reference conversion
			ctx.enumConstants[constant.name] = prefixedName
		}
		ctx.source.constBlocks = append(ctx.source.constBlocks, ConstBlock{
			typeName:  enumTypeName,
			constants: prefixedConstants,
		})
	}

	// Parse and convert methods from enum body
	if enumBody != nil {
		// Recursively find all method_declaration nodes
		var findMethods func(node *tree_sitter.Node)
		findMethods = func(node *tree_sitter.Node) {
			iterateChilden(node, func(bodyChild *tree_sitter.Node) {
				switch bodyChild.Kind() {
				case "method_declaration":
					// Handle methods similar to class methods
					function, isStatic := convertMethodDeclaration(ctx, bodyChild)
					if isStatic {
						ctx.source.functions = append(ctx.source.functions, function)
					} else {
						ctx.source.methods = append(ctx.source.methods, Method{
							Function: function,
							receiver: Param{
								name: SELF_REF,
								ty:   Type("*" + enumTypeName),
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
		ctx.enumConstants[constant.name] = prefixedName
	}

	// Parse fields from enum body
	var fields []StructField

	// Recursively find all field_declaration and method_declaration nodes
	var findFieldsAndMethods func(node *tree_sitter.Node)
	findFieldsAndMethods = func(node *tree_sitter.Node) {
		iterateChilden(node, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "field_declaration":
				field, _, _ := convertFieldDeclaration(ctx, child)
				fields = append(fields, field)
			case "method_declaration":
				// Handle methods similar to class methods
				function, isStatic := convertMethodDeclaration(ctx, child)
				if isStatic {
					ctx.source.functions = append(ctx.source.functions, function)
				} else {
					ctx.source.methods = append(ctx.source.methods, Method{
						Function: function,
						receiver: Param{
							name: SELF_REF,
							ty:   Type("*" + enumTypeName),
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
	ctx.source.structs = append(ctx.source.structs, Struct{
		name:     enumTypeName,
		fields:   fields,
		comments: []string{},
		public:   isPublic,
		includes: []Type{},
	})

	// Generate var declarations for each enum constant
	// Parse field names to create struct literal
	fieldNames := make([]string, len(fields))
	for i, field := range fields {
		fieldNames[i] = toIdentifier(field.name, field.public)
	}

	for _, constant := range enumConstants {
		prefixedName := enumTypeName + "_" + constant.name
		// Create struct literal with constructor arguments
		var structLiteral Expression
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
			structLiteral = &VarRef{ref: sb.String()}
		} else {
			// Empty struct or mismatch - use empty struct
			structLiteral = &VarRef{ref: enumTypeName + "{}"}
		}
		ctx.source.vars = append(ctx.source.vars, ModuleVar{
			name:  prefixedName,
			ty:    Type(enumTypeName),
			value: structLiteral,
		})
	}
}

func hasModifier(ctx *MigrationContext, methodNode *tree_sitter.Node, modifier string) bool {
	hasModifier := false
	iterateChilden(methodNode, func(child *tree_sitter.Node) {
		if child.Kind() == "modifiers" {
			modText := child.Utf8Text(ctx.javaSource)
			if strings.Contains(modText, modifier) {
				hasModifier = true
			}
		}
	})
	return hasModifier
}

func extractInterfaceMethodSignature(ctx *MigrationContext, methodNode *tree_sitter.Node) InterfaceMethod {
	var name string
	var params []Param
	var returnType *Type
	var hasThrows bool

	iterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := tryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(ctx.javaSource)
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
			unhandledChild(ctx, child, "interface_method_signature")
		}
	})

	// Handle throws clause - convert to error return
	if hasThrows {
		if returnType == nil {
			// void method with exception -> error
			errorType := Type("error")
			returnType = &errorType
		} else {
			// non-void method with exception -> (T, error)
			tupleType := Type("(" + returnType.ToSource() + ", error)")
			returnType = &tupleType
		}
	}

	return InterfaceMethod{
		name:       capitalizeFirstLetter(name),
		params:     params,
		returnType: returnType,
		public:     true, // All interface methods are public
	}
}

func convertMethodDeclarationToFunction(ctx *MigrationContext, methodNode *tree_sitter.Node, isDefault bool, interfaceName string) Function {
	var name string
	var params []Param
	var body []Statement
	var returnType *Type
	var hasThrows bool

	iterateChilden(methodNode, func(child *tree_sitter.Node) {
		ty, isType := tryParseType(ctx, child)
		if isType {
			returnType = &ty
			return
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(ctx.javaSource)
		case "formal_parameters":
			params = convertFormalParameters(ctx, child)
		case "void_type":
			returnType = nil
		case "block":
			// For default methods, we need to convert the body to capitalize method calls
			if isDefault {
				// Set context for default method conversion
				oldInDefaultMethod := ctx.inDefaultMethod
				oldDefaultMethodSelf := ctx.defaultMethodSelf
				ctx.inDefaultMethod = true
				ctx.defaultMethodSelf = "this"

				// Convert block with empty field map (interfaces have no fields)
				rawBody := convertStatementBlock(ctx, child)
				for _, stmt := range rawBody {
					body = append(body, convertStatementForDefaultMethod(ctx, stmt, interfaceName, make(map[string]bool)))
				}

				// Restore context
				ctx.inDefaultMethod = oldInDefaultMethod
				ctx.defaultMethodSelf = oldDefaultMethodSelf
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
			unhandledChild(ctx, child, "interface_default_method")
		}
	})

	// Handle throws clause
	if hasThrows {
		if returnType == nil {
			errorType := Type("error")
			returnType = &errorType
		} else {
			tupleType := Type("(" + returnType.ToSource() + ", error)")
			returnType = &tupleType
		}
	}

	// If default method, prepend 'this' parameter
	if isDefault {
		thisParam := Param{
			name: "this",
			ty:   Type(capitalizeFirstLetter(interfaceName)),
		}
		params = append([]Param{thisParam}, params...)
	}

	return Function{
		name:       capitalizeFirstLetter(name),
		params:     params,
		returnType: returnType,
		body:       body,
		public:     true,
	}
}

func convertAbstractClass(ctx *MigrationContext, className string, modifiers modifiers, includes []Type, classBody *tree_sitter.Node) {
	// Extract fields and methods
	var fields []StructField
	var abstractMethods []Function
	var defaultMethods []Function
	var comments []string
	fieldInitValues := map[string]Expression{}

	iterateChilden(classBody, func(child *tree_sitter.Node) {
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
				ctx.source.vars = append(ctx.source.vars, ModuleVar{
					name:  field.name,
					ty:    field.ty,
					value: initExpr,
				})
			} else {
				if initExpr != nil {
					Assert("mutiple initializations for field"+field.name, fieldInitValues[field.name] == nil)
					fieldInitValues[field.name] = initExpr
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
				ctx.source.functions = append(ctx.source.functions, function)
			}
		case "constructor_declaration":
			// Abstract classes can have constructors, but we'll skip them for now
		// ignored
		case "{":
		case "}":
		case "block_comment":
		case "line_comment":
		default:
			unhandledChild(ctx, child, "class_body")
		}
	})

	// Generate FooData interface
	dataInterfaceName := capitalizeFirstLetter(className) + "Data"
	var dataMethods []InterfaceMethod
	for _, field := range fields {
		fieldName := capitalizeFirstLetter(field.name)
		getterName := "Get" + fieldName
		setterName := "Set" + fieldName
		dataMethods = append(dataMethods, InterfaceMethod{
			name:       getterName,
			params:     []Param{},
			returnType: &field.ty,
			public:     true,
		})
		dataMethods = append(dataMethods, InterfaceMethod{
			name:       setterName,
			params:     []Param{{name: toIdentifier(field.name, false), ty: field.ty}},
			returnType: nil,
			public:     true,
		})
	}
	ctx.source.interfaces = append(ctx.source.interfaces, Interface{
		name:     dataInterfaceName,
		embeds:   []Type{},
		methods:  dataMethods,
		public:   true, // Interfaces for abstract classes are always public
		comments: comments,
	})

	// Generate FooBase struct
	baseStructName := capitalizeFirstLetter(className) + "Base"
	// Capitalize field names in base struct
	var capitalizedFields []StructField
	for _, field := range fields {
		capitalizedFields = append(capitalizedFields, StructField{
			name:     capitalizeFirstLetter(field.name),
			ty:       field.ty,
			public:   true,
			comments: field.comments,
		})
	}
	ctx.source.structs = append(ctx.source.structs, Struct{
		name:     baseStructName,
		includes: []Type{},
		fields:   capitalizedFields,
		public:   true, // Base structs for abstract classes are always public
		comments: comments,
	})

	// Generate getter/setter methods for FooBase
	for _, field := range fields {
		fieldName := capitalizeFirstLetter(field.name)
		getterName := "Get" + fieldName
		setterName := "Set" + fieldName
		ctx.source.methods = append(ctx.source.methods, Method{
			Function: Function{
				name:       getterName,
				params:     []Param{},
				returnType: &field.ty,
				body: []Statement{
					&ReturnStatement{value: &VarRef{ref: "b." + toIdentifier(field.name, true)}},
				},
				public: true,
			},
			receiver: Param{
				name: "b",
				ty:   Type("*" + baseStructName),
			},
		})
		ctx.source.methods = append(ctx.source.methods, Method{
			Function: Function{
				name:       setterName,
				params:     []Param{{name: toIdentifier(field.name, false), ty: field.ty}},
				returnType: nil,
				body: []Statement{
					&AssignStatement{
						ref:   VarRef{ref: "b." + toIdentifier(field.name, true)},
						value: &VarRef{ref: toIdentifier(field.name, false)},
					},
				},
				public: true,
			},
			receiver: Param{
				name: "b",
				ty:   Type("*" + baseStructName),
			},
		})
	}

	// Generate FooMethods struct
	methodsStructName := capitalizeFirstLetter(className) + "Methods"
	ctx.source.structs = append(ctx.source.structs, Struct{
		name:     methodsStructName,
		includes: []Type{},
		fields: []StructField{
			{
				name:   "Self",
				ty:     Type(capitalizeFirstLetter(className)),
				public: true,
			},
		},
		public:   true, // Methods structs for abstract classes are always public
		comments: comments,
	})

	// Convert default methods to use m.Self
	for _, method := range defaultMethods {
		// Convert method body to use m.Self
		convertedBody := convertMethodBodyForDefaultMethod(ctx, method.body, className, fields)
		ctx.source.methods = append(ctx.source.methods, Method{
			Function: Function{
				name:       capitalizeFirstLetter(method.name),
				params:     method.params,
				returnType: method.returnType,
				body:       convertedBody,
				comments:   method.comments,
				public:     true, // Methods in FooMethods are always public
			},
			receiver: Param{
				name: "m",
				ty:   Type("*" + methodsStructName),
			},
		})
	}

	// Generate Foo interface
	var interfaceMethods []InterfaceMethod
	// Add abstract method signatures - always capitalize for abstract class interfaces
	for _, method := range abstractMethods {
		interfaceMethods = append(interfaceMethods, InterfaceMethod{
			name:       capitalizeFirstLetter(method.name),
			params:     method.params,
			returnType: method.returnType,
			public:     true,
		})
	}
	// Add default method signatures - always capitalize for abstract class interfaces
	for _, method := range defaultMethods {
		interfaceMethods = append(interfaceMethods, InterfaceMethod{
			name:       capitalizeFirstLetter(method.name),
			params:     method.params,
			returnType: method.returnType,
			public:     true,
		})
	}
	ctx.source.interfaces = append(ctx.source.interfaces, Interface{
		name:     capitalizeFirstLetter(className),
		embeds:   []Type{Type(dataInterfaceName)},
		methods:  interfaceMethods,
		public:   true, // Main interface for abstract classes is always public
		comments: comments,
	})
}

func convertMethodBodyForDefaultMethod(ctx *MigrationContext, body []Statement, className string, fields []StructField) []Statement {
	var converted []Statement
	oldInDefaultMethod := ctx.inDefaultMethod
	oldDefaultMethodSelf := ctx.defaultMethodSelf
	ctx.inDefaultMethod = true
	ctx.defaultMethodSelf = "m.Self"
	defer func() {
		ctx.inDefaultMethod = oldInDefaultMethod
		ctx.defaultMethodSelf = oldDefaultMethodSelf
	}()
	// Build map of field names for quick lookup
	fieldMap := make(map[string]bool)
	for _, field := range fields {
		fieldMap[field.name] = true
	}
	for _, stmt := range body {
		converted = append(converted, convertStatementForDefaultMethod(ctx, stmt, className, fieldMap))
	}
	return converted
}

func convertStatementForDefaultMethod(ctx *MigrationContext, stmt Statement, className string, fieldMap map[string]bool) Statement {
	switch s := stmt.(type) {
	case *GoStatement:
		// Replace this.field with m.Self.GetField() and this.method() with m.Self.Method()
		source := s.source
		// Simple string replacement for common patterns
		// This is a simplified version - in production you'd want a more sophisticated AST-based approach
		source = strings.ReplaceAll(source, "this.", ctx.defaultMethodSelf+".")
		return &GoStatement{source: source}
	case *ReturnStatement:
		if s.value != nil {
			return &ReturnStatement{value: convertExpressionForDefaultMethod(ctx, s.value, className, fieldMap)}
		}
		return s
	case *AssignStatement:
		// Convert field assignments: this.field = value -> m.Self.SetField(value)
		refStr := s.ref.ToSource()
		if strings.HasPrefix(refStr, "this.") {
			// For now, keep as assignment - we'll need more sophisticated handling
			return &AssignStatement{
				ref:   VarRef{ref: strings.ReplaceAll(refStr, "this.", ctx.defaultMethodSelf+".")},
				value: convertExpressionForDefaultMethod(ctx, s.value, className, fieldMap),
			}
		}
		return &AssignStatement{
			ref:   s.ref,
			value: convertExpressionForDefaultMethod(ctx, s.value, className, fieldMap),
		}
	case *IfStatement:
		return &IfStatement{
			condition: convertExpressionForDefaultMethod(ctx, s.condition, className, fieldMap),
			body:      convertStatementsForDefaultMethod(ctx, s.body, className, fieldMap),
			elseIf:    convertElseIfsForDefaultMethod(ctx, s.elseIf, className, fieldMap),
			elseStmts: convertStatementsForDefaultMethod(ctx, s.elseStmts, className, fieldMap),
		}
	case *ForStatement:
		var initStmt Statement
		if s.init != nil {
			initStmt = convertStatementForDefaultMethod(ctx, s.init, className, fieldMap)
		}
		var postStmt Statement
		if s.post != nil {
			postStmt = convertStatementForDefaultMethod(ctx, s.post, className, fieldMap)
		}
		return &ForStatement{
			init:      initStmt,
			condition: convertExpressionForDefaultMethod(ctx, s.condition, className, fieldMap),
			post:      postStmt,
			body:      convertStatementsForDefaultMethod(ctx, s.body, className, fieldMap),
		}
	case *CallStatement:
		return &CallStatement{
			exp: convertExpressionForDefaultMethod(ctx, s.exp, className, fieldMap),
		}
	case *VarDeclaration:
		if s.value != nil {
			return &VarDeclaration{
				name:  s.name,
				ty:    s.ty,
				value: convertExpressionForDefaultMethod(ctx, s.value, className, fieldMap),
			}
		}
		return s
	default:
		// For other statement types, try to convert recursively if possible
		return stmt
	}
}

func convertStatementsForDefaultMethod(ctx *MigrationContext, stmts []Statement, className string, fieldMap map[string]bool) []Statement {
	var converted []Statement
	for _, stmt := range stmts {
		converted = append(converted, convertStatementForDefaultMethod(ctx, stmt, className, fieldMap))
	}
	return converted
}

func convertElseIfsForDefaultMethod(ctx *MigrationContext, elseIfs []IfStatement, className string, fieldMap map[string]bool) []IfStatement {
	var converted []IfStatement
	for _, elseIf := range elseIfs {
		converted = append(converted, IfStatement{
			condition: convertExpressionForDefaultMethod(ctx, elseIf.condition, className, fieldMap),
			body:      convertStatementsForDefaultMethod(ctx, elseIf.body, className, fieldMap),
			elseIf:    convertElseIfsForDefaultMethod(ctx, elseIf.elseIf, className, fieldMap),
			elseStmts: convertStatementsForDefaultMethod(ctx, elseIf.elseStmts, className, fieldMap),
		})
	}
	return converted
}

func convertMethodBodyForRecord(ctx *MigrationContext, body []Statement, fieldNameMap map[string]string) []Statement {
	var converted []Statement
	for _, stmt := range body {
		converted = append(converted, convertStatementForRecord(ctx, stmt, fieldNameMap))
	}
	return converted
}

func convertStatementForRecord(ctx *MigrationContext, stmt Statement, fieldNameMap map[string]string) Statement {
	switch s := stmt.(type) {
	case *GoStatement:
		// Replace bare field references with this.FieldName
		// GoStatement contains raw source, so we do simple string replacement
		// This is a simplified approach - in production you'd want AST-based replacement
		source := s.source
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
			replacement := SELF_REF + "." + structFieldName
			// Use word boundary-aware replacement
			// Replace standalone occurrences (not part of "this.field")
			beforePattern := SELF_REF + "." + originalName
			if !strings.Contains(source, beforePattern) {
				// Replace bare field name with this.FieldName
				// Be careful: only replace if it's a standalone identifier
				// Simple approach: replace and then fix if we created "this.this.Field"
				source = strings.ReplaceAll(source, originalName, replacement)
				source = strings.ReplaceAll(source, SELF_REF+"."+SELF_REF+".", SELF_REF+".")
			} else {
				// Already has "this.field", just capitalize the field name
				source = strings.ReplaceAll(source, beforePattern, SELF_REF+"."+structFieldName)
			}
		}
		return &GoStatement{source: source}
	case *ReturnStatement:
		if s.value != nil {
			return &ReturnStatement{value: convertExpressionForRecord(ctx, s.value, fieldNameMap)}
		}
		return s
	case *AssignStatement:
		refExpr := convertExpressionForRecord(ctx, &VarRef{ref: s.ref.ref}, fieldNameMap)
		var ref VarRef
		if varRef, ok := refExpr.(*VarRef); ok {
			ref = *varRef
		} else {
			// Fallback: use original ref
			ref = s.ref
		}
		return &AssignStatement{
			ref:   ref,
			value: convertExpressionForRecord(ctx, s.value, fieldNameMap),
		}
	case *IfStatement:
		return &IfStatement{
			condition: convertExpressionForRecord(ctx, s.condition, fieldNameMap),
			body:      convertMethodBodyForRecord(ctx, s.body, fieldNameMap),
			elseIf:    convertElseIfsForRecord(ctx, s.elseIf, fieldNameMap),
			elseStmts: convertMethodBodyForRecord(ctx, s.elseStmts, fieldNameMap),
		}
	default:
		return stmt
	}
}

func convertElseIfsForRecord(ctx *MigrationContext, elseIfs []IfStatement, fieldNameMap map[string]string) []IfStatement {
	var converted []IfStatement
	for _, elseIf := range elseIfs {
		converted = append(converted, IfStatement{
			condition: convertExpressionForRecord(ctx, elseIf.condition, fieldNameMap),
			body:      convertMethodBodyForRecord(ctx, elseIf.body, fieldNameMap),
			elseIf:    convertElseIfsForRecord(ctx, elseIf.elseIf, fieldNameMap),
			elseStmts: convertMethodBodyForRecord(ctx, elseIf.elseStmts, fieldNameMap),
		})
	}
	return converted
}

func convertExpressionForRecord(ctx *MigrationContext, expr Expression, fieldNameMap map[string]string) Expression {
	switch e := expr.(type) {
	case *VarRef:
		ref := e.ref
		// Check if this is a bare field reference that needs conversion
		if structFieldName, ok := fieldNameMap[ref]; ok {
			// Convert bare field reference to this.FieldName
			return &VarRef{ref: SELF_REF + "." + structFieldName}
		}
		// If it's already this.field, check if the field name needs capitalization
		if strings.HasPrefix(ref, SELF_REF+".") {
			fieldName := strings.TrimPrefix(ref, SELF_REF+".")
			if structFieldName, ok := fieldNameMap[fieldName]; ok {
				return &VarRef{ref: SELF_REF + "." + structFieldName}
			}
		}
		return e
	case *BinaryExpression:
		return &BinaryExpression{
			left:     convertExpressionForRecord(ctx, e.left, fieldNameMap),
			operator: e.operator,
			right:    convertExpressionForRecord(ctx, e.right, fieldNameMap),
		}
	case *UnaryExpression:
		return &UnaryExpression{
			operator: e.operator,
			operand:  convertExpressionForRecord(ctx, e.operand, fieldNameMap),
		}
	case *CallExpression:
		var convertedArgs []Expression
		for _, arg := range e.args {
			convertedArgs = append(convertedArgs, convertExpressionForRecord(ctx, arg, fieldNameMap))
		}
		return &CallExpression{
			function: e.function,
			args:     convertedArgs,
		}
	default:
		return expr
	}
}

func convertExpressionForDefaultMethod(ctx *MigrationContext, expr Expression, className string, fieldMap map[string]bool) Expression {
	switch e := expr.(type) {
	case *VarRef:
		ref := e.ref
		if strings.HasPrefix(ref, "this.") {
			fieldName := strings.TrimPrefix(ref, "this.")
			// Convert to getter: this.field -> m.Self.GetField()
			capitalized := capitalizeFirstLetter(fieldName)
			return &VarRef{ref: ctx.defaultMethodSelf + ".Get" + capitalized + "()"}
		}
		// Check if this is a bare field reference
		if fieldMap[ref] {
			// Convert bare field reference to getter: field -> m.Self.GetField()
			capitalized := capitalizeFirstLetter(ref)
			return &VarRef{ref: ctx.defaultMethodSelf + ".Get" + capitalized + "()"}
		}
		ref = strings.ReplaceAll(ref, "this.", ctx.defaultMethodSelf+".")
		return &VarRef{ref: ref}
	case *CallExpression:
		funcName := e.function
		if strings.HasPrefix(funcName, "this.") {
			funcName = strings.TrimPrefix(funcName, "this.")
			funcName = ctx.defaultMethodSelf + "." + capitalizeFirstLetter(funcName)
		} else if funcName == "this" {
			funcName = ctx.defaultMethodSelf
		} else if !strings.Contains(funcName, ".") && !fieldMap[funcName] {
			// Bare method call (not a field) - assume it's a method on self
			funcName = ctx.defaultMethodSelf + "." + capitalizeFirstLetter(funcName)
		}
		var convertedArgs []Expression
		for _, arg := range e.args {
			convertedArgs = append(convertedArgs, convertExpressionForDefaultMethod(ctx, arg, className, fieldMap))
		}
		return &CallExpression{
			function: funcName,
			args:     convertedArgs,
		}
	case *BinaryExpression:
		return &BinaryExpression{
			left:     convertExpressionForDefaultMethod(ctx, e.left, className, fieldMap),
			operator: e.operator,
			right:    convertExpressionForDefaultMethod(ctx, e.right, className, fieldMap),
		}
	case *UnaryExpression:
		return &UnaryExpression{
			operator: e.operator,
			operand:  convertExpressionForDefaultMethod(ctx, e.operand, className, fieldMap),
		}
	case *GoExpression:
		source := e.source
		// Replace this.method() with m.Self.Method() (capitalized)
		// Pattern: this.methodName( -> m.Self.MethodName(
		source = strings.ReplaceAll(source, "this.", ctx.defaultMethodSelf+".")
		// Capitalize method names after m.Self.
		if strings.Contains(source, ctx.defaultMethodSelf+".") {
			// Find method calls like m.Self.method( and capitalize method name
			parts := strings.Split(source, ctx.defaultMethodSelf+".")
			if len(parts) > 1 {
				for i := 1; i < len(parts); i++ {
					// Find the method name (up to the opening parenthesis or end)
					methodPart := parts[i]
					methodEnd := strings.IndexAny(methodPart, "(")
					if methodEnd > 0 {
						methodName := methodPart[:methodEnd]
						capitalized := capitalizeFirstLetter(methodName)
						parts[i] = capitalized + methodPart[methodEnd:]
					} else {
						parts[i] = capitalizeFirstLetter(methodPart)
					}
				}
				source = strings.Join(parts, ctx.defaultMethodSelf+".")
			}
		}
		return &GoExpression{source: source}
	default:
		return expr
	}
}

type classConversionResult struct {
	fields    []StructField
	comments  []string
	functions []Function
	methods   []Method
}

func convertClassBody(ctx *MigrationContext, structName string, classBody *tree_sitter.Node, isAbstract bool) classConversionResult {
	var result classConversionResult
	fieldInitValues := map[string]Expression{}
	iterateChilden(classBody, func(child *tree_sitter.Node) {
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
				ctx.source.vars = append(ctx.source.vars, ModuleVar{
					name:  field.name,
					ty:    field.ty,
					value: initExpr,
				})
			} else {
				// Regular field
				if initExpr != nil {
					Assert("mutiple initializations for field"+field.name, fieldInitValues[field.name] == nil)
					fieldInitValues[field.name] = initExpr
				}
				result.fields = append(result.fields, field)
			}
		case "constructor_declaration":
			constructor := convertConstructor(ctx, &fieldInitValues, structName, child)
			result.functions = append(result.functions, constructor)
		case "compact_constructor_declaration":
			// Compact constructors are handled in migrateRecordDeclaration, skip here
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
		case "line_comment":
		default:
			unhandledChild(ctx, child, "class_body")
		}
	})
	return result
}

// TODO: this is very similar to constructor conversion, refactor
func convertMethodDeclaration(ctx *MigrationContext, methodNode *tree_sitter.Node) (Function, bool) {
	fn, isStatic, _ := convertMethodDeclarationWithAbstract(ctx, methodNode)
	return fn, isStatic
}

func convertMethodDeclarationWithAbstract(ctx *MigrationContext, methodNode *tree_sitter.Node) (Function, bool, bool) {
	var modifiers modifiers
	var params []Param
	var body []Statement
	var name string
	var returnType *Type
	var hasThrows bool
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
		case "throws":
			hasThrows = true
		// ignored
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "method_declaration")
		}
	})

	// Modify return type if method throws exceptions
	if hasThrows {
		if returnType == nil {
			// void method with exception -> error
			errorType := Type("error")
			returnType = &errorType
		} else {
			// non-void method with exception -> (T, error)
			tupleType := Type("(" + returnType.ToSource() + ", error)")
			returnType = &tupleType
		}
	}

	isAbstract := modifiers&ABSTRACT != 0 && len(body) == 0
	// If method is abstract and has no body, add panic statement (for non-abstract class methods)
	if isAbstract && len(body) == 0 {
		body = append(body, &GoStatement{source: "panic(\"implemented in concrete class\")"})
	}
	isStatic := modifiers&STATIC != 0
	return Function{
		name:       name,
		params:     params,
		returnType: returnType,
		body:       body,
		public:     modifiers&PUBLIC != 0,
	}, isStatic, isAbstract
}

func convertStatementBlock(ctx *MigrationContext, blockNode *tree_sitter.Node) []Statement {
	var body []Statement
	iterateChilden(blockNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			body = append(body, convertStatement(ctx, child)...)
		}
	})
	return body
}

func convertSwitchStatement(ctx *MigrationContext, switchNode *tree_sitter.Node) SwitchStatement {
	condition, conditionInit := convertExpression(ctx, switchNode.ChildByFieldName("condition"))
	Assert("condition expression is expected to be simple", len(conditionInit) == 0)
	bodyNode := switchNode.ChildByFieldName("body")
	var cases []SwitchCase
	var defaultBody []Statement
	iterateChilden(bodyNode, func(switchBlockStatementGroup *tree_sitter.Node) {
		switch switchBlockStatementGroup.Kind() {
		case "switch_block_statement_group":
			var caseBody []Statement
			var caseCondition Expression
			var isDefault bool
			iterateChilden(switchBlockStatementGroup, func(child *tree_sitter.Node) {
				switch child.Kind() {
				case "switch_label":
					if child.Utf8Text(ctx.javaSource) == "default" {
						isDefault = true
					} else {
						caseCondition, conditionInit = convertExpression(ctx, child.Child(1))
						Assert("condition expression is expected to be simple", len(conditionInit) == 0)
					}
				// ignored
				case ":":
				case "{":
				case "}":
				case "->":
				case "line_comment":
				case "block_comment":
				default:
					var stmts []Statement
					if child.Kind() == "block" {
						stmts = convertStatementBlock(ctx, child)
					} else {
						stmts = convertStatement(ctx, child)
					}
					if isDefault {
						defaultBody = append(defaultBody, stmts...)
					} else {
						caseBody = append(caseBody, stmts...)
					}
				}
			})
			if !isDefault {
				cases = append(cases, SwitchCase{
					condition: caseCondition,
					body:      caseBody,
				})
			}
		case "switch_rule":
			caseConditionNode := switchBlockStatementGroup.Child(0)
			caseCondition := GoExpression{source: caseConditionNode.Utf8Text(ctx.javaSource)}
			bodyNode := switchBlockStatementGroup.Child(2)
			for bodyNode.Kind() == "line_comment" || bodyNode.Kind() == ":" || bodyNode.Kind() == "->" {
				bodyNode = bodyNode.NextSibling()
			}
			var caseBody []Statement
			if bodyNode.Kind() == "block" {
				caseBody = convertStatementBlock(ctx, bodyNode)
			} else {
				caseBody = convertStatement(ctx, bodyNode)
			}
			cases = append(cases, SwitchCase{
				condition: &caseCondition,
				body:      caseBody,
			})
			// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, switchBlockStatementGroup, "switch_block_statement_group")
		}
	})
	// TODO: if in return properly detect value points and add returns
	return SwitchStatement{
		condition:   condition,
		cases:       cases,
		defaultBody: defaultBody,
	}
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
		case "line_comment":
		case "block_comment":
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
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "constructor_body")
		}
	})
	return body
}

// convertRecordComponentsToParams converts record components (StructField) to function parameters (Param)
// Applies the same array-to-pointer conversion as convertFormalParameters for consistency
func convertRecordComponentsToParams(components []StructField) []Param {
	var params []Param
	for _, component := range components {
		ty := component.ty
		// Convert array types to pointer-to-array for parameters (same as convertFormalParameters)
		if isArrayOrSliceType(ty) {
			ty = Type("*" + ty)
		}
		params = append(params, Param{
			name: component.name,
			ty:   ty,
		})
	}
	return params
}

func convertCompactConstructor(ctx *MigrationContext, recordComponents []StructField, structName string, compactConstructorNode *tree_sitter.Node) Function {
	var modifiers modifiers
	var body []Statement
	// Convert record components to parameters
	params := convertRecordComponentsToParams(recordComponents)
	// Initialize struct
	body = append(body, &GoStatement{source: fmt.Sprintf("%s := %s{};", SELF_REF, structName)})
	// Process compact constructor body
	iterateChilden(compactConstructorNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "modifiers":
			modifiers = parseModifiers(child.Utf8Text(ctx.javaSource))
		case "block":
			// Compact constructor body is a block
			body = append(body, convertCompactConstructorBody(ctx, recordComponents, structName, child)...)
		// ignored
		case "identifier":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "compact_constructor_declaration")
		}
	})
	// After body execution, assign parameters to struct fields
	for _, component := range recordComponents {
		structFieldName := toIdentifier(component.name, true) // Always public for records
		paramName := component.name
		body = append(body, &AssignStatement{
			ref:   VarRef{ref: SELF_REF + "." + structFieldName},
			value: &VarRef{ref: paramName},
		})
	}
	body = append(body, &ReturnStatement{value: &VarRef{ref: SELF_REF}})
	// Generate function name: newStructNameFromParam1Param2...
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

func convertCompactConstructorBody(ctx *MigrationContext, recordComponents []StructField, structName string, bodyNode *tree_sitter.Node) []Statement {
	var body []Statement
	// Process statements in the compact constructor body
	// Unlike regular constructors, compact constructors cannot have:
	// - Explicit constructor invocations (this(...) or super(...))
	// - Explicit assignments to component fields (they're implicit)
	// The body can contain validation/normalization logic that modifies parameters
	iterateChilden(bodyNode, func(child *tree_sitter.Node) {
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
			unhandledChild(ctx, child, "compact_constructor_body")
		}
	})
	return body
}

func convertStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []Statement {
	switch stmtNode.Kind() {
	case "line_comment":
		return nil
	case "block_comment":
		return nil
	case "switch_expression":
		switchStatement := convertSwitchStatement(ctx, stmtNode)
		return []Statement{&switchStatement}
	case "assert_statement":
		conditionNode := stmtNode.Child(1)
		conditionExp, initStmts := convertExpression(ctx, conditionNode)
		Assert("condition expression is expected to be simple", len(initStmts) == 0)
		return append(initStmts, &IfStatement{
			condition: conditionExp,
			body:      []Statement{&GoStatement{source: "panic(\"assertion failed\")"}},
		})
	case "expression_statement":
		var body []Statement
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "assignment_expression":
				// Check for compound assignment operators
				refNode := child.ChildByFieldName("left")
				valueNode := child.ChildByFieldName("right")

				var operator string
				iterateChilden(child, func(grandChild *tree_sitter.Node) {
					switch grandChild.Kind() {
					case "|=", "&=", "^=", "<<=", ">>=", "+=", "-=", "*=", "/=", "%=":
						operator = grandChild.Utf8Text(ctx.javaSource)
					}
				})

				if operator != "" {
					// Compound assignment: x op= y -> x = x op y
					leftExp, leftInit := convertExpression(ctx, refNode)
					rightExp, rightInit := convertExpression(ctx, valueNode)
					body = append(body, leftInit...)
					body = append(body, rightInit...)

					// Extract the base operator (remove =)
					baseOp := operator[:len(operator)-1]

					// Convert >>>= to >>= (Go doesn't have >>>)
					if baseOp == ">>>" {
						baseOp = ">>"
					}

					// Create: x = (x op y)
					body = append(body, &AssignStatement{
						ref: VarRef{ref: leftExp.ToSource()},
						value: &BinaryExpression{
							left:     leftExp,
							operator: baseOp,
							right:    rightExp,
						},
					})
				} else {
					// Regular assignment
					ref := VarRef{ref: refNode.Utf8Text(ctx.javaSource)}
					valueExp, initStmts := convertExpression(ctx, valueNode)
					if len(initStmts) > 0 {
						Fatal(valueNode.ToSexp(), errors.New("unexpected statements in assignment expression"))
					}
					body = append(body, &AssignStatement{
						ref:   ref,
						value: valueExp,
					})
				}
			case "method_invocation":
				// Check if this is a .add() call that should be converted to append
				methodName := child.ChildByFieldName("name").Utf8Text(ctx.javaSource)
				objectNode := child.ChildByFieldName("object")

				if methodName == "add" && objectNode != nil {
					// Convert list.add(item) to list = append(list, item)
					objectText := objectNode.Utf8Text(ctx.javaSource)
					argsNode := child.ChildByFieldName("arguments")
					if argsNode != nil {
						args := convertArgumentList(ctx, argsNode)
						if len(args) > 0 {
							// Create: list = append(list, item)
							body = append(body, &AssignStatement{
								ref: VarRef{ref: objectText},
								value: &CallExpression{
									function: "append",
									args:     append([]Expression{&VarRef{ref: objectText}}, args...),
								},
							})
						} else {
							// Fall through to regular method call handling
							callExperession, initStmts := convertExpression(ctx, child)
							body = append(body, initStmts...)
							body = append(body, &CallStatement{exp: callExperession})
						}
					} else {
						// Fall through to regular method call handling
						callExperession, initStmts := convertExpression(ctx, child)
						body = append(body, initStmts...)
						body = append(body, &CallStatement{exp: callExperession})
					}
				} else {
					callExperession, initStmts := convertExpression(ctx, child)
					body = append(body, initStmts...)
					body = append(body, &CallStatement{exp: callExperession})
				}
			// ignored
			case ";":
			default:
				expr, initStmts := convertExpression(ctx, child)
				body = append(body, initStmts...)
				body = append(body, &GoStatement{source: expr.ToSource() + ";"})
			}
		})
		return body
	case "return_statement":
		var initialStmts []Statement
		var value Expression
		ctx.inReturn = true
		iterateChilden(stmtNode, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case ";":
			case "return":
			default:
				value, initialStmts = convertExpression(ctx, child)
			}
		})
		ctx.inReturn = true
		// Check if value is a SwitchStatement
		if switchStmt, ok := value.(*SwitchStatement); ok {
			// If value is a SwitchStatement, flatten to its switch form
			// Not conventional return, treat as statement
			return append(initialStmts, switchStmt)
		}
		return append(initialStmts, &ReturnStatement{value: value})
	case "if_statement":
		ifStatement := convertIfStatement(ctx, stmtNode, false)
		return []Statement{&ifStatement}
	case "break_statement":
		return []Statement{&GoStatement{source: "break;"}}
	case "continue_statement":
		return []Statement{&GoStatement{source: "continue;"}}
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
				},
			}
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
	case "for_statement":
		initNode := stmtNode.ChildByFieldName("init")
		var initStmts []Statement
		if initNode != nil {
			initStmts = convertStatement(ctx, initNode)
		}
		conditionNode := stmtNode.ChildByFieldName("condition")
		conditionExp, s := convertExpression(ctx, conditionNode)
		initStmts = append(initStmts, s...)
		updateNode := stmtNode.ChildByFieldName("update")
		var updateExp Expression
		if updateNode != nil {
			var updateStmts []Statement
			updateExp, updateStmts = convertExpression(ctx, updateNode)
			initStmts = append(initStmts, updateStmts...)
		}
		bodyNode := stmtNode.ChildByFieldName("body")
		bodyStmts := convertStatementBlock(ctx, bodyNode)
		return append(initStmts, &ForStatement{
			condition: conditionExp,
			post:      updateExp,
			body:      bodyStmts,
		})
	case "enhanced_for_statement":
		varName := stmtNode.ChildByFieldName("name").Utf8Text(ctx.javaSource)
		valueExpr, stmts := convertExpression(ctx, stmtNode.ChildByFieldName("value"))
		bodyStmts := convertStatementBlock(ctx, stmtNode.ChildByFieldName("body"))
		return append(stmts, &RangeForStatement{
			valueVar:       varName,
			collectionExpr: valueExpr,
			body:           bodyStmts,
		})
	case "throw_statement":
		valueNode := stmtNode.Child(1)
		exception := valueNode.ChildByFieldName("type").Utf8Text(ctx.javaSource)
		arguments := valueNode.ChildByFieldName("arguments").Utf8Text(ctx.javaSource)
		switch exception {
		case "IllegalArgumentException":
			return []Statement{
				&GoStatement{
					source: fmt.Sprintf("panic(%s)", arguments),
				},
			}
		default:
			return []Statement{
				&GoStatement{
					source: stmtNode.Utf8Text(ctx.javaSource),
				},
			}
		}
	case ";":
		return nil
	case "yield_statement":
		expr, init := convertExpression(ctx, stmtNode.Child(1))
		init = append(init, &GoStatement{source: expr.ToSource() + ";"})
		return init
	case "try_statement":
		tryStatement := convertTryStatement(ctx, stmtNode)
		return []Statement{&tryStatement}
	default:
		expr, init := convertExpression(ctx, stmtNode)
		init = append(init, &GoStatement{source: expr.ToSource() + ";"})
		return init
	}
}

func convertTryStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) TryStatement {
	var tryBody []Statement
	var catchClauses []CatchClause
	var finallyBody []Statement

	// Get try body
	bodyNode := stmtNode.ChildByFieldName("body")
	if bodyNode != nil {
		tryBody = convertStatementBlock(ctx, bodyNode)
	}

	// Check for finally using field name
	finallyNode := stmtNode.ChildByFieldName("finally")
	if finallyNode != nil {
		finallyBodyNode := finallyNode.ChildByFieldName("body")
		if finallyBodyNode != nil {
			finallyBody = convertStatementBlock(ctx, finallyBodyNode)
		} else if finallyNode.Kind() == "block" {
			finallyBody = convertStatementBlock(ctx, finallyNode)
		}
	}

	// Iterate through children to find catch clauses and finally
	iterateChilden(stmtNode, func(child *tree_sitter.Node) {
		if child.Kind() == "catch_clause" {
			var exceptionType string
			var exceptionVar string
			var catchBody []Statement

			// Find catch_formal_parameter
			iterateChilden(child, func(catchChild *tree_sitter.Node) {
				if catchChild.Kind() == "catch_formal_parameter" {
					// Find catch_type
					iterateChilden(catchChild, func(paramChild *tree_sitter.Node) {
						if paramChild.Kind() == "catch_type" {
							// Get the type identifier from catch_type
							iterateChilden(paramChild, func(typeChild *tree_sitter.Node) {
								if typeChild.Kind() == "type_identifier" || typeChild.Kind() == "scoped_type_identifier" {
									exceptionType = typeChild.Utf8Text(ctx.javaSource)
								}
							})
						}
					})
					// Get name field
					nameNode := catchChild.ChildByFieldName("name")
					if nameNode != nil {
						exceptionVar = nameNode.Utf8Text(ctx.javaSource)
					}
				}
			})
			// Get catch body
			catchBodyNode := child.ChildByFieldName("body")
			if catchBodyNode != nil {
				catchBody = convertStatementBlock(ctx, catchBodyNode)
			}

			if exceptionType != "" {
				catchClauses = append(catchClauses, CatchClause{
					exceptionType: exceptionType,
					exceptionVar:  exceptionVar,
					body:          catchBody,
				})
			}
		} else if child.Kind() == "finally_clause" {
			// Get finally body
			finallyBodyNode := child.ChildByFieldName("body")
			if finallyBodyNode != nil {
				finallyBody = convertStatementBlock(ctx, finallyBodyNode)
			} else {
				// Look for block as direct child
				iterateChilden(child, func(fc *tree_sitter.Node) {
					if fc.Kind() == "block" {
						finallyBody = convertStatementBlock(ctx, fc)
					}
				})
			}
		}
	})

	return TryStatement{
		tryBody:      tryBody,
		catchClauses: catchClauses,
		finallyBody:  finallyBody,
	}
}

func convertIfStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node, inner bool) IfStatement {
	conditionNode := stmtNode.ChildByFieldName("condition")
	conditionExp, stmts := convertExpression(ctx, conditionNode)
	Assert("condition expression is expected to be simple", len(stmts) == 0)
	bodyNode := stmtNode.ChildByFieldName("consequence")
	bodyStmts := convertStatementBlock(ctx, bodyNode)
	ifStatement := &IfStatement{
		condition: conditionExp,
		body:      bodyStmts,
	}
	cursor := stmtNode.Walk()
	elseIf := stmtNode.ChildrenByFieldName("alternative", cursor)
	for _, elseIfNode := range elseIf {
		switch elseIfNode.Kind() {
		case "if_statement":
			ifStatement.elseIf = append(ifStatement.elseIf, convertIfStatement(ctx, &elseIfNode, true))
		case "block":
			elseBodyStmts := convertStatementBlock(ctx, &elseIfNode)
			ifStatement.elseStmts = append(ifStatement.elseStmts, elseBodyStmts...)
		default:
			unhandledChild(ctx, &elseIfNode, "else_if_statement")
		}
	}
	return *ifStatement
}

func convertExplicitConstructorInvocation(ctx *MigrationContext, invocationNode *tree_sitter.Node) []Statement {
	parentCall := "this"
	var argExp []Expression
	iterateChilden(invocationNode, func(args *tree_sitter.Node) {
		switch args.Kind() {
		case "this":
			parentCall = "this"
		case "super":
			parentCall = "super"
		case "argument_list":
			argExp = convertArgumentList(ctx, args)
		// ignored
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, args, "explicit_constructor_invocation")
		}
	})
	return []Statement{
		&CallStatement{
			exp: &CallExpression{
				function: parentCall,
				args:     argExp,
			},
		},
	}
}

func convertArgumentList(ctx *MigrationContext, argList *tree_sitter.Node) []Expression {
	var args []Expression
	iterateChilden(argList, func(child *tree_sitter.Node) {
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
				Fatal(child.ToSexp(), errors.New("unexpected statements in argument list expression"))
			}
			args = append(args, exp)
		}
	})
	return args
}

func convertArrayInitializer(ctx *MigrationContext, initNode *tree_sitter.Node) []Expression {
	var elements []Expression
	iterateChilden(initNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "{", "}", ",":
			// Structural tokens - ignore
		case "line_comment":
		case "block_comment":
		default:
			// Any other node is an element expression
			exp, init := convertExpression(ctx, child)
			if len(init) > 0 {
				Fatal(child.ToSexp(), errors.New("unexpected statements in array initializer"))
			}
			elements = append(elements, exp)
		}
	})
	return elements
}

func tryGetChildByFieldName(node *tree_sitter.Node, fieldName string) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.FieldNameForNamedChild(uint32(i)) == fieldName {
			return child
		}
	}
	return nil
}

func convertExpression(ctx *MigrationContext, expression *tree_sitter.Node) (Expression, []Statement) {
	switch expression.Kind() {
	case "this":
		return &GoExpression{source: "this"}, nil
	case "assignment_expression":
		// Check for compound assignment operators
		leftNode := expression.ChildByFieldName("left")
		rightNode := expression.ChildByFieldName("right")

		// Check if this is a compound assignment by looking for operators like |=, &=, etc.
		var operator string
		iterateChilden(expression, func(child *tree_sitter.Node) {
			switch child.Kind() {
			case "|=", "&=", "^=", "<<=", ">>=", "+=", "-=", "*=", "/=", "%=":
				operator = child.Utf8Text(ctx.javaSource)
			}
		})

		if operator != "" {
			// This is a compound assignment: x op= y -> x = x op y
			leftExp, leftInit := convertExpression(ctx, leftNode)
			rightExp, rightInit := convertExpression(ctx, rightNode)

			// Extract the base operator (remove =)
			baseOp := operator[:len(operator)-1]

			// Convert >>>= to >>= (Go doesn't have >>>)
			if baseOp == ">>>" {
				baseOp = ">>"
			}

			// Create: left = (left op right)
			result := &BinaryExpression{
				left:     leftExp,
				operator: "=",
				right: &BinaryExpression{
					left:     leftExp,
					operator: baseOp,
					right:    rightExp,
				},
			}
			return result, append(leftInit, rightInit...)
		}

		// Regular assignment
		leftExp, leftInit := convertExpression(ctx, leftNode)
		rightExp, rightInit := convertExpression(ctx, rightNode)
		return &BinaryExpression{
			left:     leftExp,
			operator: "=",
			right:    rightExp,
		}, append(leftInit, rightInit...)
	case "ternary_expression":
		// TODO: do better
		return &GoExpression{
			source: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "array_creation_expression":
		typeNode := expression.ChildByFieldName("type")
		ty, ok := tryParseType(ctx, typeNode)
		if !ok {
			Fatal(typeNode.ToSexp(), errors.New("unable to parse type in array_creation_expression"))
		}

		// Check for dimensions to make it an array type
		dimensionsNode := expression.ChildByFieldName("dimensions")
		if dimensionsNode != nil {
			// Add [] prefix to make it an array type
			ty = Type("[]" + ty.ToSource())
		}

		valueNode := expression.ChildByFieldName("value")
		if valueNode == nil {
			// No initializer: return nil
			return &GoExpression{source: "nil"}, nil
		}

		// Has initializer: new Type[] { ... }
		elements := convertArrayInitializer(ctx, valueNode)
		return &ArrayLiteral{
			elementType: ty,
			elements:    elements,
		}, nil
	case "instanceof_expression":
		valueNode := expression.ChildByFieldName("left")
		valueExp, initStmts := convertExpression(ctx, valueNode)
		Assert("condition expression is expected to be simple", len(initStmts) == 0)
		typeNode := expression.ChildByFieldName("right")
		ty, ok := tryParseType(ctx, typeNode)
		if !ok {
			Fatal(typeNode.ToSexp(), errors.New("unable to parse type in instanceof_expression"))
		}
		return &GoExpression{
			source: fmt.Sprintf("%s.(%s)", valueExp.ToSource(), ty.ToSource()),
		}, nil
	case "update_expression":
		return &GoExpression{
			source: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "switch_expression":
		switchStatement := convertSwitchStatement(ctx, expression)
		return &switchStatement, nil
	case "identifier":
		identName := expression.Utf8Text(ctx.javaSource)
		// Check if this is an enum constant reference
		if prefixedName, ok := ctx.enumConstants[identName]; ok {
			return &VarRef{
				ref: prefixedName,
			}, nil
		}
		return &VarRef{
			ref: identName,
		}, nil
	case "array_access":
		return &GoExpression{
			source: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "object_creation_expression":
		ty, isType := tryParseType(ctx, expression.ChildByFieldName("type"))
		if !isType {
			Fatal(expression.ToSexp(), errors.New("unable to parse type in object_creation_expression"))
		}
		if ty.isArray() {
			return &GoExpression{
				source: fmt.Sprintf("make(%s, 0)", ty),
			}, nil
		}

		// Check for ArrayList creation: new ArrayList<>() or new ArrayList<Type>()
		typeText := expression.ChildByFieldName("type").Utf8Text(ctx.javaSource)
		if strings.HasPrefix(typeText, "ArrayList") {
			// Extract element type from generic if present: ArrayList<Type> -> Type
			// For now, use interface{} as default
			elementType := "interface{}"

			// Try to find type arguments
			typeArgsNode := expression.ChildByFieldName("type").ChildByFieldName("type_arguments")
			if typeArgsNode != nil {
				iterateChilden(typeArgsNode, func(child *tree_sitter.Node) {
					if child.Kind() == "type_identifier" {
						elementType = child.Utf8Text(ctx.javaSource)
					}
				})
			}

			// Convert to Go slice: make([]Type, 0)
			return &GoExpression{
				source: fmt.Sprintf("make([]%s, 0)", elementType),
			}, nil
		}

		// TODO: properly initialize objects here
		return &NIL, nil
	case "field_access":
		object := expression.ChildByFieldName("object")
		field := expression.ChildByFieldName("field")

		if object != nil && field != nil {
			objectText := object.Utf8Text(ctx.javaSource)
			fieldText := field.Utf8Text(ctx.javaSource)

			// Check if this looks like an enum constant (object is type name, field is uppercase)
			// Heuristic: if object starts with uppercase, it's likely a type/enum reference
			if len(objectText) > 0 && objectText[0] >= 'A' && objectText[0] <= 'Z' {
				// Enum constant: Foo.BAR → Foo_BAR
				return &VarRef{
					ref: objectText + "_" + fieldText,
				}, nil
			}
			// Regular field access: keep dot notation
			return &VarRef{
				ref: objectText + "." + fieldText,
			}, nil
		}

		// Fallback to original text
		return &VarRef{
			ref: expression.Utf8Text(ctx.javaSource),
		}, nil
	case "method_invocation":
		name := expression.ChildByFieldName("name").Utf8Text(ctx.javaSource)
		objectNode := expression.ChildByFieldName("object")
		objectText := ""
		if objectNode != nil {
			objectText = objectNode.Utf8Text(ctx.javaSource)
		}

		switch name {
		case "equals":
			// String.equals(other) -> string == other
			argsNode := expression.ChildByFieldName("arguments")
			if argsNode != nil {
				args := convertArgumentList(ctx, argsNode)
				if len(args) > 0 {
					// Convert: "active".equals(s) -> "active" == s
					return &BinaryExpression{
						left:     &VarRef{ref: objectText},
						operator: "==",
						right:    args[0],
					}, nil
				}
			}
		case "size":
			return &GoExpression{
				source: fmt.Sprintf("len(%s)", objectText),
			}, nil
		case "asList":
			// Arrays.asList(...) -> []Type{...}
			// Only handle if object is "Arrays"
			if objectText == "Arrays" {
				argsNode := expression.ChildByFieldName("arguments")
				if argsNode != nil {
					args := convertArgumentList(ctx, argsNode)
					if len(args) > 0 {
						// Convert arguments to slice literal
						// Use interface{} as element type (could be improved with type inference)
						return &ArrayLiteral{
							elementType: Type("interface{}"),
							elements:    args,
						}, nil
					}
				}
				return &GoExpression{
					source: "[]interface{}{}",
				}, nil
			}
		case "toArray":
			// list.toArray(Type[]::new) -> convert to slice
			// The method reference is already handled, so this should work
			// For now, return the object as a slice (assuming it's already a slice)
			return &GoExpression{
				source: objectText,
			}, nil
		case "add":
			// list.add(item) -> list = append(list, item)
			// This needs to be handled as a statement, not an expression
			// For now, return as Go expression that can be used in statements
			argsNode := expression.ChildByFieldName("arguments")
			if argsNode != nil {
				args := convertArgumentList(ctx, argsNode)
				if len(args) > 0 {
					// Return: append(list, item)
					return &GoExpression{
						source: fmt.Sprintf("append(%s, %s)", objectText, args[0].ToSource()),
					}, nil
				}
			}
			return &GoExpression{
				source: SELF_REF + "." + expression.Utf8Text(ctx.javaSource),
			}, nil
		default:
			// Handle method calls on this or other objects
			if objectText == "this" || objectText == SELF_REF {
				// Special handling for Java enum name() method
				if name == "name" {
					// this.name() -> this.Name() (will need a Name() method implementation)
					return &GoExpression{
						source: fmt.Sprintf("%s.Name()", SELF_REF),
					}, nil
				}
				// Method call on this - just capitalize method name
				capitalizedName := capitalizeFirstLetter(name)
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
				return &GoExpression{
					source: fmt.Sprintf("%s.%s(%s)", SELF_REF, capitalizedName, argsStr),
				}, nil
			}
			// Handle method calls on enum constants
			if prefixedName, ok := ctx.enumConstants[objectText]; ok {
				// If objectText is an enum constant, prepend its prefixed name
				return &GoExpression{
					source: fmt.Sprintf("%s.%s", prefixedName, capitalizeFirstLetter(name)),
				}, nil
			}
			// TODO: fix casts
			// Fallback: convert the expression and clean up any this.this patterns
			exprText := expression.Utf8Text(ctx.javaSource)
			// If expression already starts with "this.", don't prepend another "this."
			if strings.HasPrefix(exprText, "this.") {
				// Clean up any this.this patterns
				exprText = strings.ReplaceAll(exprText, "this.this.", "this.")
				return &GoExpression{
					source: exprText,
				}, nil
			}
			return &GoExpression{
				source: SELF_REF + "." + exprText,
			}, nil
		}
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
			case "<<", ">>", ">>>":
				// Bit shift operators
				operator = child.Utf8Text(ctx.javaSource)
				// Go uses >> for both signed and unsigned right shift
				if operator == ">>>" {
					operator = ">>"
				}
			case "|", "&", "^":
				// Bitwise operators
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
	case "string_literal":
		return &GoExpression{
			source: expression.Utf8Text(ctx.javaSource),
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
			case "!", "+", "-", "~":
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
	case "method_reference":
		// Handle method references like Type[]::new
		// This is typically used for array constructors: Type[]::new -> make([]Type, 0)
		objectNode := expression.ChildByFieldName("object")
		methodNode := expression.ChildByFieldName("method")

		if objectNode != nil && methodNode != nil {
			objectText := objectNode.Utf8Text(ctx.javaSource)
			methodText := methodNode.Utf8Text(ctx.javaSource)

			// Check if this is an array constructor: Type[]::new
			if methodText == "new" && strings.HasSuffix(objectText, "[]") {
				// Extract the element type
				elementType := strings.TrimSuffix(objectText, "[]")
				// Convert to Go: make([]Type, 0)
				return &GoExpression{
					source: fmt.Sprintf("make([]%s, 0)", elementType),
				}, nil
			}
		}

		// Fallback: return as-is (may need more sophisticated handling)
		return &GoExpression{
			source: expression.Utf8Text(ctx.javaSource),
		}, nil
	default:
		fmt.Println(expression.Utf8Text(ctx.javaSource))
		expression.Parent()
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

func isArrayOrSliceType(ty Type) bool {
	return strings.HasPrefix(string(ty), "[]")
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
			// Convert array types to pointer-to-array for parameters
			if isArrayOrSliceType(ty) {
				ty = Type("*" + ty)
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
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "formal_parameters")
		}
	})
	return params
}

func convertFieldDeclaration(ctx *MigrationContext, fieldNode *tree_sitter.Node) (StructField, Expression, modifiers) {
	var mods modifiers
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
			mods = parseModifiers(child.Utf8Text(ctx.javaSource))
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
				initExpr = &ArrayLiteral{elementType: ty, elements: elements}
			}
		// ignored
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			unhandledChild(ctx, child, "field_declaration")
		}
	})
	return StructField{
		name:     name,
		ty:       ty,
		public:   mods&PUBLIC != 0,
		comments: comments,
	}, initExpr, mods
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
		// Skip array_initializer - parent will handle with type context
		if valueNode.Kind() == "array_initializer" {
			return variableDeclResult{
				name:  name,
				value: nil, // Signal to parent to handle
			}
		}

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
	case "scoped_type_identifier":
		// For scoped types like Atom.Kind, we only use the second part (Kind)
		// since Go doesn't have nested types
		var typeName string
		// The last type_identifier child is the actual type we want
		iterateChilden(node, func(child *tree_sitter.Node) {
			if child.Kind() == "type_identifier" {
				typeName = child.Utf8Text(ctx.javaSource)
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
				return Type(goType), true
			}
		}
		if strings.HasPrefix(typeName, "ST") {
			goType = "internal." + typeName
			return Type(goType), true
		}
		switch typeName {
		case "Object":
			goType = "interface{}"
		case "String":
			goType = "string"
		case "DiagnosticCode":
			goType = "diagnostics.DiagnosticCode"
		case "SyntaxKind":
			goType = "common.SyntaxKind"
		default:
			goType = typeName
		}
		return Type(goType), true
	case "type_identifier":
		var goType string
		typeName := node.Utf8Text(ctx.javaSource)
		unwantedPrefixes := []string{"Abstract", "LexerTerminals", "ST"}
		for _, prefix := range unwantedPrefixes {
			if strings.HasPrefix(typeName, prefix) {
				goType = typeName[len(prefix):]
				return Type(goType), true
			}
		}
		if strings.HasPrefix(typeName, "ST") {
			goType = "internal." + typeName
			return Type(goType), true
		}
		switch typeName {
		case "Object":
			goType = "interface{}"
		case "String":
			goType = "string"
		case "DiagnosticCode":
			goType = "diagnostics.DiagnosticCode"
		case "SyntaxKind":
			goType = "common.SyntaxKind"
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
