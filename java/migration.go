package java

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	"github.com/pelletier/go-toml/v2"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// MigrationContext holds state during Java to Go migration
type MigrationContext struct {
	Source                   gosrc.GoSource
	JavaSource               []byte
	SourceFilePath           string // Path to the source Java file
	InReturn                 bool
	AbstractClasses          map[string]bool
	InDefaultMethod          bool
	DefaultMethodSelf        string
	EnumConstants            map[string]string // Maps enum constant name to prefixed name (e.g., "ACTIVE" -> "Status_ACTIVE")
	Constructors             map[gosrc.Type][]FunctionData
	Methods                  map[string][]FunctionData       // Maps method name to method signatures
	MethodMetadataCache      map[uintptr]methodMetadata      // Cache of parsed method signatures by node ID
	ConstructorMetadataCache map[uintptr]constructorMetadata // Cache of parsed constructor signatures by node ID
	StrictMode               bool                            // If true, treat migration errors as fatal
	Errors                   []MigrationError                // Collected migration errors
	TypeMappings             map[string]string
}

// MigrationError represents an error that occurred during migration
type MigrationError struct {
	Location   string // e.g., "class Foo.method bar"
	JavaSource string // The Java code that failed
	SExpr      string // The S-expression
	Message    string // Error message
	NodeKind   string // Type of node (for debugging)
}

type FunctionData struct {
	Name          string
	ArgumentTypes []gosrc.Type
}

func (this FunctionData) sameArgs(other FunctionData) bool {
	return slices.Equal(this.ArgumentTypes, other.ArgumentTypes)
}

// NewMigrationContext creates and initializes a new MigrationContext
func NewMigrationContext(javaSource []byte, sourceFilePath string, strictMode bool) *MigrationContext {
	// TODO: get these from config file
	typeMappings := make(map[string]string)
	typeMappings["DiagnosticCode"] = "diagnostics.DiagnosticCode"
	typeMappings["SyntaxKind"] = "diagnostics.DiagnosticCode"
	return &MigrationContext{
		JavaSource:               javaSource,
		SourceFilePath:           sourceFilePath,
		AbstractClasses:          make(map[string]bool),
		EnumConstants:            make(map[string]string),
		Constructors:             make(map[gosrc.Type][]FunctionData),
		Methods:                  make(map[string][]FunctionData),
		MethodMetadataCache:      make(map[uintptr]methodMetadata),
		ConstructorMetadataCache: make(map[uintptr]constructorMetadata),
		StrictMode:               strictMode,
		Errors:                   []MigrationError{},
		TypeMappings:             typeMappings,
	}
}

// LoadConfig loads migration configuration from Config.toml
func LoadConfig() gosrc.Config {
	config := gosrc.Config{
		PackageName:   gosrc.PackageName,
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

	var fileConfig gosrc.Config
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

// MigrateTree migrates a Java tree-sitter tree to Go source
func MigrateTree(ctx *MigrationContext, tree *tree_sitter.Tree) {
	// Analyze tree first to collect method metadata
	analyzeNode(ctx, tree)

	// Then perform migration
	root := tree.RootNode()
	migrateNode(ctx, root)
}

// analyzeNode performs pre-migration analysis to collect method signatures
func analyzeNode(ctx *MigrationContext, tree *tree_sitter.Tree) {
	analyzeMethodDeclartions(ctx, tree)
	analyzeConstructorDeclarations(ctx, tree)
}

func analyzeMethodDeclartions(ctx *MigrationContext, tree *tree_sitter.Tree) {
	// Create query to find all method declarations
	language := tree_sitter.NewLanguage(tree_sitter_java.Language())
	query, err := tree_sitter.NewQuery(language, "(method_declaration) @method")
	if err != nil {
		// This is a programming error - the query syntax is invalid
		panic(fmt.Sprintf("Invalid tree-sitter query: %v", err))
	}
	defer query.Close()

	// Execute query
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	root := tree.RootNode()
	matches := cursor.Matches(query, root, ctx.JavaSource)

	// Process each match
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			methodNode := &capture.Node

			// Parse method signature with error recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						// In strict mode, let panic propagate
						if ctx.StrictMode {
							panic(r)
						}
						// In non-strict mode, skip this method and continue
						// We don't add it to the context, but log the error
						if panicErr, ok := r.(MigrationPanic); ok {
							fmt.Fprintf(os.Stderr, "Warning: Failed to analyze method signature: %s\n", panicErr.Message)
						} else {
							fmt.Fprintf(os.Stderr, "Warning: Failed to analyze method signature: %v\n", r)
						}
					}
				}()

				methodMetadata := parseMethodSignature(ctx, methodNode)
				funcData := methodMetadata.toFunctionData()
				addMethodToCtx(ctx, funcData, methodMetadata, methodNode.Id())
			}()
		}
	}
}

func analyzeConstructorDeclarations(ctx *MigrationContext, tree *tree_sitter.Tree) {
	// Create query to find all constructor declarations
	language := tree_sitter.NewLanguage(tree_sitter_java.Language())
	query, err := tree_sitter.NewQuery(language, "(constructor_declaration) @constructor")
	if err != nil {
		// This is a programming error - the query syntax is invalid
		panic(fmt.Sprintf("Invalid tree-sitter query: %v", err))
	}
	defer query.Close()

	// Execute query
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	root := tree.RootNode()
	matches := cursor.Matches(query, root, ctx.JavaSource)

	// Process each match
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			constructorNode := &capture.Node

			// Parse constructor signature
			constructorMetadata := parseConstructorSignature(ctx, constructorNode)
			funcData := constructorMetadata.toFunctionData()

			addConstructorToCtx(ctx, funcData, constructorMetadata, constructorNode.Id())
		}
	}
}

func addMethodToCtx(ctx *MigrationContext, fn FunctionData, metadata methodMetadata, nodeID uintptr) {
	name, shouldChangeName := addMethodToCtxInner(ctx, fn)
	if shouldChangeName {
		metadata.name = name
	}
	ctx.MethodMetadataCache[nodeID] = metadata
}

func addMethodToCtxInner(ctx *MigrationContext, fn FunctionData) (string, bool) {
	currentMethods := ctx.Methods[fn.Name]
	if len(currentMethods) == 0 {
		ctx.Methods[fn.Name] = append(currentMethods, fn)
		return fn.Name, false
	}
	// Check if we already have a matching method
	for _, each := range currentMethods {
		if each.sameArgs(fn) {
			// No need to add we already have a mathching method
			return each.Name, true
		}
	}
	baseName := fn.Name
	overloadedName := overloadedName(baseName, fn.ArgumentTypes)
	fn.Name = overloadedName
	ctx.Methods[baseName] = append(currentMethods, fn)
	return overloadedName, true
}

func addConstructorToCtx(ctx *MigrationContext, fn FunctionData, metadata constructorMetadata, nodeID uintptr) {
	ty := gosrc.Type(metadata.structName)
	currentConstructors := ctx.Constructors[ty]
	if len(currentConstructors) == 0 {
		ctx.Constructors[ty] = append(currentConstructors, fn)
		ctx.ConstructorMetadataCache[nodeID] = metadata
		return
	}
	// Check if we already have a matching constructor
	for _, each := range currentConstructors {
		if each.sameArgs(fn) {
			// No need to add we already have a matching constructor
			ctx.ConstructorMetadataCache[nodeID] = metadata
			return
		}
	}
	// Constructor names already include parameter types (e.g., "newTypeFromString"),
	// so they should be unique. Just add it with the original name.
	ctx.Constructors[ty] = append(currentConstructors, fn)
	ctx.ConstructorMetadataCache[nodeID] = metadata
}

// getMigrationComment creates a comment indicating the source location in the Java file
func getMigrationComment(ctx *MigrationContext, node *tree_sitter.Node) string {
	pos := node.StartPosition()
	// Convert from 0-based to 1-based
	row := pos.Row + 1
	col := pos.Column + 1
	return fmt.Sprintf("migrated from %s:%d:%d", ctx.SourceFilePath, row, col)
}

// migrateNode dispatches node migration based on node kind
func migrateNode(ctx *MigrationContext, node *tree_sitter.Node) {
	switch node.Kind() {
	case "program":
		IterateChildren(node, func(child *tree_sitter.Node) {
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
		UnhandledChild(ctx, node, "<root>")
	}
}
