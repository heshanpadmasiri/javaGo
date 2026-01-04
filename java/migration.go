package java

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	"github.com/pelletier/go-toml/v2"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// MigrationContext holds state during Java to Go migration
type MigrationContext struct {
	Source              gosrc.GoSource
	JavaSource          []byte
	InReturn            bool
	AbstractClasses     map[string]bool
	InDefaultMethod     bool
	DefaultMethodSelf   string
	EnumConstants       map[string]string // Maps enum constant name to prefixed name (e.g., "ACTIVE" -> "Status_ACTIVE")
	Constructors        map[gosrc.Type][]FunctionData
	Methods             map[string][]FunctionData  // Maps method name to method signatures
	MethodMetadataCache map[uintptr]methodMetadata // Cache of parsed method signatures by node ID
}

type FunctionData struct {
	Name          string
	ArgumentTypes []gosrc.Type
}

// NewMigrationContext creates and initializes a new MigrationContext
func NewMigrationContext(javaSource []byte) *MigrationContext {
	return &MigrationContext{
		JavaSource:          javaSource,
		AbstractClasses:     make(map[string]bool),
		EnumConstants:       make(map[string]string),
		Constructors:        make(map[gosrc.Type][]FunctionData),
		Methods:             make(map[string][]FunctionData),
		MethodMetadataCache: make(map[uintptr]methodMetadata),
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

			// Parse method signature using existing function
			methodMetadata := parseMethodSignature(ctx, methodNode)

			// Store in cache by node ID
			nodeId := methodNode.Id()
			ctx.MethodMetadataCache[nodeId] = methodMetadata

			// Convert to FunctionData
			var argTypes []gosrc.Type
			for _, param := range methodMetadata.params {
				argTypes = append(argTypes, param.Ty)
			}

			funcData := FunctionData{
				Name:          methodMetadata.name,
				ArgumentTypes: argTypes,
			}

			// Store in Methods map
			methodName := methodMetadata.name
			ctx.Methods[methodName] = append(ctx.Methods[methodName], funcData)
		}
	}
}

// migrateNode dispatches node migration based on node kind
func migrateNode(ctx *MigrationContext, node *tree_sitter.Node) {
	switch node.Kind() {
	case "program":
		IterateChilden(node, func(child *tree_sitter.Node) {
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
