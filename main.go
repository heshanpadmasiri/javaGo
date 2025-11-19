package main

import (
	"fmt"
	"os"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

type MigrationContext struct {
	source GoSource
}

type GoSource struct {
	structs   []GoStruct
	functions []GoFunction
	methods   []GoMethod
}

func (s *GoSource) ToSource() string {
	sb := strings.Builder{}
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
	name   string
	ty     GoType
	public bool
}

func (f *GoStructField) ToSource() string {
	return fmt.Sprintf("%s %s", toIdentifier(f.name, f.public), f.ty.ToSource())
}

func toIdentifier(name string, public bool) string {
	first := name[0]
	if public {
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
	Expressions() []GoExpression
}

type GoExpression interface {
	SourceElement
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

	ctx := &MigrationContext{}
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
	fmt.Printf("Root node type: %s\n", root.Kind())
	fmt.Printf("Root node sexpr: %s\n", root.ToSexp())
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
