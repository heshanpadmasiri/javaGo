// Package gosrc provide type safe way to represent go source code along with way
// to convert them to actual go source code
package gosrc

import (
	"fmt"
	"strings"
)

const (
	SelfRef     = "this"
	PackageName = "converted"
)

// Interfaces for source elements

type (
	// SourceElement represents any element that can be converted to Go source
	SourceElement interface {
		ToSource() string
	}

	// Statement represents a Go statement
	Statement interface {
		SourceElement
	}

	// Expression represents a Go expression
	Expression interface {
		SourceElement
	}
)

// Core Go source structures

type (
	// GoSource represents a complete Go source file
	GoSource struct {
		Imports          []Import
		Interfaces       []Interface
		Structs          []Struct
		Constants        []ModuleConst
		ConstBlocks      []ConstBlock
		Vars             []ModuleVar
		Functions        []Function
		Methods          []Method
		FailedMigrations []FailedMigration
	}

	// Import represents a package import
	Import struct {
		PackagePath string
		Alias       *string
	}

	// Interface represents a Go interface definition
	Interface struct {
		Name     string
		Embeds   []Type
		Methods  []InterfaceMethod
		Public   bool
		Comments []string
	}

	// InterfaceMethod represents a method signature in an interface
	InterfaceMethod struct {
		Name       string
		Params     []Param
		ReturnType *Type
		Public     bool
	}

	// Struct represents a Go struct definition
	Struct struct {
		Name     string
		Includes []Type
		Fields   []StructField
		Public   bool
		Comments []string
	}

	// StructField represents a field in a struct
	StructField struct {
		Name     string
		Ty       Type
		Public   bool
		Comments []string
	}

	// Function represents a Go function
	Function struct {
		Name       string
		Params     []Param
		ReturnType *Type
		Body       []Statement
		Comments   []string
		Public     bool
	}

	// Method represents a Go method with a receiver
	Method struct {
		Function
		Receiver Param
	}

	// Param represents a function or method parameter
	Param struct {
		Name string
		Ty   Type
	}

	// ModuleConst represents a module-level constant
	ModuleConst struct {
		Name  string
		Ty    Type
		Value Expression
	}

	// ConstBlock represents a const block with iota
	ConstBlock struct {
		TypeName  string
		Constants []string
	}

	// ModuleVar represents a module-level variable
	ModuleVar struct {
		Name  string
		Ty    Type
		Value Expression
	}

	// FailedMigration represents a migration that failed
	FailedMigration struct {
		ErrorMessage string
		JavaSource   string
		SExpr        string
		Location     string
	}
)

// Statement implementations

type (
	// GoStatement represents a raw Go statement string
	GoStatement struct {
		Source string
	}

	// IfStatement represents an if-else statement
	IfStatement struct {
		Condition Expression
		Body      []Statement
		ElseIf    []IfStatement
		ElseStmts []Statement
	}

	// SwitchStatement represents a switch statement
	SwitchStatement struct {
		Condition   Expression
		Cases       []SwitchCase
		DefaultBody []Statement
	}

	// SwitchCase represents a case in a switch statement
	SwitchCase struct {
		Condition Expression
		Body      []Statement
	}

	// ForStatement represents a traditional for loop
	ForStatement struct {
		Init      Statement
		Condition Expression
		Post      Statement
		Body      []Statement
	}

	// RangeForStatement represents a range-based for loop
	RangeForStatement struct {
		IndexVar       string
		ValueVar       string
		CollectionExpr Expression
		Body           []Statement
	}

	// ReturnStatement represents a return statement
	ReturnStatement struct {
		Value Expression
	}

	// VarDeclaration represents a variable declaration
	VarDeclaration struct {
		Name  string
		Ty    Type
		Value Expression
	}

	// AssignStatement represents an assignment
	AssignStatement struct {
		Ref   VarRef
		Value Expression
	}

	// CallStatement represents a function call statement
	CallStatement struct {
		Exp Expression
	}

	// TryStatement represents a try-catch-finally block
	TryStatement struct {
		TryBody      []Statement
		CatchClauses []CatchClause
		FinallyBody  []Statement
	}

	// CatchClause represents a catch clause in a try statement
	CatchClause struct {
		ExceptionType string
		ExceptionVar  string
		Body          []Statement
	}

	// CommentStmt represents comment statements
	CommentStmt struct {
		Comments []string
	}
)

// Expression implementations

type (
	// GoExpression represents a raw Go expression string
	GoExpression struct {
		Source string
	}

	// CastExpression represents a type cast
	CastExpression struct {
		Ty    Type
		Value Expression
	}

	// CallExpression represents a function call
	CallExpression struct {
		Function string
		Args     []Expression
	}

	// VarRef represents a variable reference
	VarRef struct {
		Ref string
	}

	// BooleanLiteral represents a boolean literal
	BooleanLiteral struct {
		Value bool
	}

	// IntLiteral represents an integer literal
	IntLiteral struct {
		Value int
	}

	// Int64Literal represents a 64-bit integer literal
	Int64Literal struct {
		Value int64
	}

	// CharLiteral represents a character literal
	CharLiteral struct {
		Value string
	}

	// ArrayLiteral represents an array/slice literal
	ArrayLiteral struct {
		ElementType Type
		Elements    []Expression
	}

	// BinaryExpression represents a binary operation
	BinaryExpression struct {
		Left     Expression
		Operator string
		Right    Expression
	}

	// UnaryExpression represents a unary operation
	UnaryExpression struct {
		Operator string
		Operand  Expression
	}

	// ReturnExpression represents a return expression
	ReturnExpression struct {
		Value Expression
	}

	// UnhandledExpression represents an unhandled expression (fallback)
	UnhandledExpression struct {
		Text string
	}
)

// Type alias

type (
	// Type represents a Go type
	Type string
)

// Type constants
const (
	TypeInt     Type = "int"
	TypeString  Type = "string"
	TypeBool    Type = "bool"
	TypeFloat64 Type = "float64"
)

// NIL is a predefined nil expression
var NIL = VarRef{Ref: "nil"}

// Config represents migration configuration
type Config struct {
	PackageName   string `toml:"package_name"`
	LicenseHeader string `toml:"license_header"`
}

// ToSource methods for all types

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
	if len(s.Imports) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range s.Imports {
			sb.WriteString("    ")
			sb.WriteString(imp.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString(")\n\n")
	}
	for _, iface := range s.Interfaces {
		sb.WriteString(iface.ToSource())
		sb.WriteString("\n")
	}
	for _, strct := range s.Structs {
		sb.WriteString(strct.ToSource())
		sb.WriteString("\n")
	}
	for _, cb := range s.ConstBlocks {
		sb.WriteString(cb.ToSource())
		sb.WriteString("\n")
	}
	for _, c := range s.Constants {
		sb.WriteString(c.ToSource())
		sb.WriteString("\n")
	}
	for _, v := range s.Vars {
		sb.WriteString(v.ToSource())
		sb.WriteString("\n")
	}
	for _, fn := range s.Functions {
		sb.WriteString(fn.ToSource())
		sb.WriteString("\n")
	}
	for _, method := range s.Methods {
		sb.WriteString(method.ToSource())
		sb.WriteString("\n")
	}
	// Render failed migrations as comments
	for _, failed := range s.FailedMigrations {
		sb.WriteString("// FIXME: Failed to migrate\n")
		sb.WriteString(fmt.Sprintf("// Location: %s\n", failed.Location))
		sb.WriteString(fmt.Sprintf("// Error: %s\n", failed.ErrorMessage))
		if failed.JavaSource != "" {
			sb.WriteString("// Java source:\n")
			for _, line := range strings.Split(failed.JavaSource, "\n") {
				sb.WriteString("// " + line + "\n")
			}
		}
		if failed.SExpr != "" {
			sb.WriteString("// S-expression:\n")
			for _, line := range strings.Split(failed.SExpr, "\n") {
				sb.WriteString("// " + line + "\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (imp *Import) ToSource() string {
	if imp.Alias != nil {
		return fmt.Sprintf("%s \"%s\"", *imp.Alias, imp.PackagePath)
	}
	return fmt.Sprintf("\"%s\"", imp.PackagePath)
}

func (i *Interface) ToSource() string {
	sb := strings.Builder{}
	AddComments(&sb, i.Comments)
	sb.WriteString("type ")
	sb.WriteString(ToIdentifier(i.Name, i.Public))
	sb.WriteString(" interface {\n")
	for _, embed := range i.Embeds {
		sb.WriteString("    ")
		sb.WriteString(embed.ToSource())
		sb.WriteString("\n")
	}
	for _, method := range i.Methods {
		sb.WriteString("    ")
		sb.WriteString(ToIdentifier(method.Name, method.Public))
		sb.WriteString("(")
		for j, param := range method.Params {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(param.ToSource())
		}
		sb.WriteString(")")
		if method.ReturnType != nil {
			sb.WriteString(" ")
			sb.WriteString(method.ReturnType.ToSource())
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (s *Struct) ToSource() string {
	sb := strings.Builder{}
	// Check if this is a type alias (empty fields and comment starting with "type")
	if len(s.Fields) == 0 && len(s.Includes) == 0 && len(s.Comments) > 0 {
		// Check if first comment is a type alias declaration
		firstComment := strings.TrimSpace(s.Comments[0])
		if strings.HasPrefix(firstComment, "type ") {
			// Output as type alias (skip the comment, output directly)
			sb.WriteString("type ")
			sb.WriteString(ToIdentifier(s.Name, s.Public))
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
	AddComments(&sb, s.Comments)
	sb.WriteString("type ")
	sb.WriteString(ToIdentifier(s.Name, s.Public))
	sb.WriteString(" struct {\n")
	for _, include := range s.Includes {
		sb.WriteString("    ")
		sb.WriteString(include.ToSource())
		sb.WriteString("\n")
	}
	for _, field := range s.Fields {
		sb.WriteString("    ")
		sb.WriteString(field.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (f *StructField) ToSource() string {
	sb := strings.Builder{}
	AddComments(&sb, f.Comments)
	sb.WriteString(fmt.Sprintf("%s %s", ToIdentifier(f.Name, f.Public), f.Ty.ToSource()))
	return sb.String()
}

func (f *Function) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString(ToIdentifier(f.Name, f.Public))
	return finishGoFunctionToSource(&sb, f)
}

func (f *Method) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("func ")
	sb.WriteString("(")
	sb.WriteString(f.Receiver.ToSource())
	sb.WriteString(") ")
	sb.WriteString(ToIdentifier(f.Name, f.Public))
	return finishGoFunctionToSource(&sb, &f.Function)
}

func finishGoFunctionToSource(sb *strings.Builder, f *Function) string {
	sb.WriteString("(")
	for i, param := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(param.ToSource())
	}
	sb.WriteString(")")
	if f.ReturnType != nil {
		sb.WriteString(" ")
		sb.WriteString(f.ReturnType.ToSource())
	}
	sb.WriteString(" {\n")
	AddComments(sb, f.Comments)
	for _, stmt := range f.Body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (p *Param) ToSource() string {
	return fmt.Sprintf("%s %s", p.Name, p.Ty.ToSource())
}

func (c *ModuleConst) ToSource() string {
	if c.Value != nil {
		return fmt.Sprintf("const %s %s = %s", c.Name, c.Ty.ToSource(), c.Value.ToSource())
	}
	if c.Ty != "" {
		return fmt.Sprintf("const %s %s", c.Name, c.Ty.ToSource())
	}
	return fmt.Sprintf("const %s", c.Name)
}

func (cb *ConstBlock) ToSource() string {
	if len(cb.Constants) == 0 {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("const (\n")
	for i, constName := range cb.Constants {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    %s %s = iota\n", constName, cb.TypeName))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", constName))
		}
	}
	sb.WriteString(")\n")
	return sb.String()
}

func (v *ModuleVar) ToSource() string {
	if v.Value != nil {
		// If name is "_" and both type and value are present, include type annotation (needed for type assertions)
		// Otherwise, use type inference (existing behavior for regular vars)
		if v.Name == "_" && v.Ty != "" {
			return fmt.Sprintf("var %s %s = %s", v.Name, v.Ty.ToSource(), v.Value.ToSource())
		}
		return fmt.Sprintf("var %s = %s", v.Name, v.Value.ToSource())
	}
	return fmt.Sprintf("var %s %s", v.Name, v.Ty.ToSource())
}

func (t *Type) ToSource() string {
	return string(*t)
}

func (t *Type) IsArray() bool {
	return strings.HasPrefix(string(*t), "[]")
}

// Statement ToSource methods

func (s *GoStatement) ToSource() string {
	return s.Source
}

func (s *IfStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("if ")
	sb.WriteString(s.Condition.ToSource())
	sb.WriteString(" {\n")
	for _, stmt := range s.Body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	// Write all else-if chains recursively
	s.writeElseIfChain(&sb, s.ElseIf)
	// Handle the final else block at the top level
	if len(s.ElseStmts) > 0 {
		sb.WriteString("else {\n")
		for _, stmt := range s.ElseStmts {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString("}")
	}
	return sb.String()
}

func (s *IfStatement) writeElseIfChain(sb *strings.Builder, elseIfs []IfStatement) {
	for _, elseIf := range elseIfs {
		sb.WriteString("else if ")
		sb.WriteString(elseIf.Condition.ToSource())
		sb.WriteString(" {\n")
		for _, stmt := range elseIf.Body {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
		sb.WriteString("}")
		// Recursively handle nested else-if chains
		s.writeElseIfChain(sb, elseIf.ElseIf)
		// Handle the final else block at this level
		if len(elseIf.ElseStmts) > 0 {
			sb.WriteString("else {\n")
			for _, stmt := range elseIf.ElseStmts {
				sb.WriteString(stmt.ToSource())
				sb.WriteString("\n")
			}
			sb.WriteString("}")
		}
	}
}

func (s *SwitchStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("switch ")
	sb.WriteString(s.Condition.ToSource())
	sb.WriteString(" {\n")
	for _, cs := range s.Cases {
		conditionStr := cs.Condition.ToSource()
		if conditionStr == "default" {
			sb.WriteString("default:\n")
			for _, stmt := range cs.Body {
				sb.WriteString(stmt.ToSource())
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("case ")
			conditionStr = strings.TrimPrefix(conditionStr, "case ")
			sb.WriteString(conditionStr)
			if len(cs.Body) > 0 {
				sb.WriteString(":\n")
				for _, stmt := range cs.Body {
					sb.WriteString(stmt.ToSource())
					sb.WriteString("\n")
				}
			} else {
				sb.WriteString(",\n")
			}
		}
	}
	if len(s.DefaultBody) > 0 {
		sb.WriteString("default:\n")
		for _, stmt := range s.DefaultBody {
			sb.WriteString(stmt.ToSource())
			sb.WriteString("\n")
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func (s *ForStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("for ")
	if s.Init != nil {
		sb.WriteString(s.Init.ToSource())
	} else {
		sb.WriteString("; ")
	}
	if s.Condition != nil {
		sb.WriteString(s.Condition.ToSource())
		sb.WriteString("; ")
	} else {
		sb.WriteString("; ")
	}
	if s.Post != nil {
		sb.WriteString(s.Post.ToSource())
	} else {
		sb.WriteString(" ")
	}
	sb.WriteString(" {\n")
	for _, stmt := range s.Body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

func (s *RangeForStatement) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString("for ")
	if s.IndexVar != "" {
		sb.WriteString(s.IndexVar)
	} else {
		sb.WriteString("_")
	}
	sb.WriteString(", ")
	if s.ValueVar != "" {
		sb.WriteString(s.ValueVar)
	} else {
		sb.WriteString("_")
	}
	sb.WriteString(" := range ")
	sb.WriteString(s.CollectionExpr.ToSource())
	sb.WriteString(" {\n")
	for _, stmt := range s.Body {
		sb.WriteString(stmt.ToSource())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

func (s *ReturnStatement) ToSource() string {
	if s.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", s.Value.ToSource())
}

func (s *VarDeclaration) ToSource() string {
	if s.Value != nil {
		return fmt.Sprintf("%s := %s", s.Name, s.Value.ToSource())
	}
	return fmt.Sprintf("var %s %s", s.Name, s.Ty.ToSource())
}

func (s *AssignStatement) ToSource() string {
	return fmt.Sprintf("%s = %s", s.Ref.ToSource(), s.Value.ToSource())
}

func (s *CallStatement) ToSource() string {
	return s.Exp.ToSource()
}

func (s *TryStatement) ToSource() string {
	sb := strings.Builder{}
	// Wrap try body in an IIFE with defer/recover
	sb.WriteString("func() {\n")
	// Add defer with recover
	sb.WriteString("    defer func() {\n")
	sb.WriteString("        if r := recover(); r != nil {\n")
	// Handle catch clauses
	if len(s.CatchClauses) > 0 {
		for i, catch := range s.CatchClauses {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("            if _, ok := r.(%s); ok {\n", catch.ExceptionType))
			} else {
				sb.WriteString(fmt.Sprintf("            } else if _, ok := r.(%s); ok {\n", catch.ExceptionType))
			}
			// Write catch body
			for _, stmt := range catch.Body {
				stmtSource := stmt.ToSource()
				// Indent each line
				lines := strings.SplitSeq(stmtSource, "\n")
				for line := range lines {
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
	for _, stmt := range s.TryBody {
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
	if len(s.FinallyBody) > 0 {
		for _, stmt := range s.FinallyBody {
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

func (s *CommentStmt) ToSource() string {
	sb := strings.Builder{}
	AddComments(&sb, s.Comments)
	return sb.String()
}

// Expression ToSource methods

func (e *GoExpression) ToSource() string {
	return e.Source
}

func (e *CastExpression) ToSource() string {
	return fmt.Sprintf("%s(%s)", e.Ty.ToSource(), e.Value.ToSource())
}

func (e *CallExpression) ToSource() string {
	sb := strings.Builder{}
	sb.WriteString(e.Function)
	sb.WriteString("(")
	for i, arg := range e.Args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.ToSource())
	}
	sb.WriteString(")")
	return sb.String()
}

func (e *VarRef) ToSource() string {
	return e.Ref
}

func (e *BooleanLiteral) ToSource() string {
	return fmt.Sprintf("%t", e.Value)
}

func (e *IntLiteral) ToSource() string {
	return fmt.Sprintf("%d", e.Value)
}

func (e *Int64Literal) ToSource() string {
	return fmt.Sprintf("int64(%d)", e.Value)
}

func (e *CharLiteral) ToSource() string {
	return fmt.Sprintf("%s", e.Value)
}

func (e *ArrayLiteral) ToSource() string {
	sb := strings.Builder{}
	// Ensure elementType has [] prefix for slice literals
	elementTypeStr := e.ElementType.ToSource()
	if !strings.HasPrefix(elementTypeStr, "[]") {
		sb.WriteString("[]")
	}
	sb.WriteString(elementTypeStr)
	sb.WriteString("{")
	for i, elem := range e.Elements {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(elem.ToSource())
	}
	sb.WriteString("}")
	return sb.String()
}

func (e *BinaryExpression) ToSource() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.ToSource(), e.Operator, e.Right.ToSource())
}

func (e *UnaryExpression) ToSource() string {
	return fmt.Sprintf("(%s%s)", e.Operator, e.Operand.ToSource())
}

func (e *ReturnExpression) ToSource() string {
	if e.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", e.Value.ToSource())
}

func (e *UnhandledExpression) ToSource() string {
	return e.Text
}

// Helper functions

// ToIdentifier converts a name to a public or private identifier
func ToIdentifier(name string, public bool) string {
	first := name[0]
	if first >= 'a' && first <= 'z' && public {
		first = first - 'a' + 'A'
	} else if first >= 'A' && first <= 'Z' && !public {
		first = first - 'A' + 'a'
	}
	return string(first) + name[1:]
}

// TODO: move thse to a common string utils package
// CapitalizeFirstLetter capitalizes the first letter of a string
func CapitalizeFirstLetter(name string) string {
	first := name[0]
	if first >= 'a' && first <= 'z' {
		first = first - 'a' + 'A'
	}
	return string(first) + name[1:]
}

// LowercaseFirstLetter lowercases the first letter of a string
func LowercaseFirstLetter(name string) string {
	if len(name) == 0 {
		return name
	}
	first := name[0]
	if first >= 'A' && first <= 'Z' {
		first = first - 'A' + 'a'
	}
	return string(first) + name[1:]
}

// AddComments adds comment lines to a string builder
func AddComments(sb *strings.Builder, comments []string) {
	for _, comment := range comments {
		sb.WriteString("// ")
		sb.WriteString(comment)
		sb.WriteString("\n")
	}
}
