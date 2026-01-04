package main

import (
	"fmt"
	"os"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/java"
)

func main() {
	config := java.LoadConfig()
	args := os.Args[1:]
	sourcePath := args[0]
	var destPath *string
	if len(args) > 1 {
		destPath = &args[1]
	}
	javaSource, err := os.ReadFile(sourcePath)
	diagnostics.Fatal("reading source file failed due to: ", err)

	tree := java.ParseJava(javaSource)
	defer tree.Close()

	ctx := java.NewMigrationContext(javaSource)
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
