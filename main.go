package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/java"
)

func main() {
	// Parse command-line flags
	strictMode := flag.Bool("Werror", false, "treat migration errors as fatal (exit on first error)")
	flag.Parse()

	config := java.LoadConfig()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: javaGo [-Werror] <source.java> [dest.go]\n")
		os.Exit(1)
	}
	sourcePath := args[0]
	var destPath *string
	if len(args) > 1 {
		destPath = &args[1]
	}
	javaSource, err := os.ReadFile(sourcePath)
	diagnostics.Fatal("reading source file failed due to: ", err)

	tree := java.ParseJava(javaSource)
	defer tree.Close()

	sourceFileName := filepath.Base(sourcePath)
	ctx := java.NewMigrationContext(javaSource, sourceFileName, *strictMode, config.TypeMappings)
	java.MigrateTree(ctx, tree)
	goSource := ctx.Source.ToSource(config)
	if destPath != nil {
		// TODO: use a proper mode
		err = os.WriteFile(*destPath, []byte(goSource), 0o644)
		if err != nil {
			diagnostics.Fatal("Failed to write to file", err)
		}
	} else {
		fmt.Println(goSource)
	}
}
