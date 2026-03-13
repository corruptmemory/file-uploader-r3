package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
)

type Args struct {
	Version    bool   `short:"v" long:"version" description:"Show version and exit"`
	ConfigFile string `short:"c" long:"config-file" description:"Config file path" default:"./file-uploader.toml"`
}

func main() {
	var args Args
	parser := flags.NewParser(&args, flags.Default)

	// Register subcommands (implemented in later specs)
	// parser.AddCommand("gen-config", ...)
	// parser.AddCommand("gen-csv", ...)

	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if args.Version {
		PrintVersion()
		os.Exit(0)
	}

	// Server startup (implemented in spec 11)
	fmt.Println("Server startup not yet implemented")
}
