package main

import (
	"os"
	"path/filepath"

	"github.com/heshanpadmasiri/javaGo/gosrc"
	"github.com/pelletier/go-toml/v2"
)

// Config represents migration configuration
type config struct {
	PackageName   string            `toml:"package_name"`
	LicenseHeader string            `toml:"license_header"`
	TypeMappings  map[string]string `toml:"type_mappings"`
}

// loadConfig loads migration configuration from Config.toml
func loadConfig() config {
	c := config{
		PackageName:   gosrc.PackageName,
		LicenseHeader: "",
	}

	wd, err := os.Getwd()
	if err != nil {
		return c
	}

	configPath := filepath.Join(wd, "Config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file doesn't exist, return defaults
		return c
	}

	var fileConfig config
	if err := toml.Unmarshal(data, &fileConfig); err != nil {
		// Invalid TOML, return defaults
		return c
	}

	// Use values from file if provided, otherwise keep defaults
	if fileConfig.PackageName != "" {
		c.PackageName = fileConfig.PackageName
	}
	if fileConfig.LicenseHeader != "" {
		c.LicenseHeader = fileConfig.LicenseHeader
	}
	if fileConfig.TypeMappings != nil {
		c.TypeMappings = fileConfig.TypeMappings
	}

	return c
}
