package java

import (
	"os"
	"path/filepath"

	"github.com/heshanpadmasiri/javaGo/gosrc"

	"github.com/pelletier/go-toml/v2"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const (
	SELF_REF     = "this"
	PACKAGE_NAME = "converted"
)

// MigrationContext holds state during Java to Go migration
type MigrationContext struct {
	Source            gosrc.GoSource
	JavaSource        []byte
	InReturn          bool
	AbstractClasses   map[string]bool
	InDefaultMethod   bool
	DefaultMethodSelf string
	EnumConstants     map[string]string // Maps enum constant name to prefixed name (e.g., "ACTIVE" -> "Status_ACTIVE")
}

// LoadConfig loads migration configuration from Config.toml
func LoadConfig() gosrc.Config {
	config := gosrc.Config{
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
	root := tree.RootNode()
	migrateNode(ctx, root)
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
