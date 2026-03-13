package main

import (
	"os"

	"github.com/BurntSushi/toml"
)

// GenConfigCommand implements the gen-config subcommand.
type GenConfigCommand struct{}

// Execute creates a Config with all defaults, encodes to TOML on stdout, and exits.
func (g *GenConfigCommand) Execute(args []string) error {
	cfg := DefaultConfig()
	encoder := toml.NewEncoder(os.Stdout)
	return encoder.Encode(cfg)
}
