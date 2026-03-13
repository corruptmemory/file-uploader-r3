package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/corruptmemory/file-uploader-r3/internal/util"
	flags "github.com/jessevdk/go-flags"
)

type Args struct {
	Version        bool   `short:"v" long:"version" description:"Show version and exit"`
	ConfigFile     string `short:"c" long:"config-file" description:"Config file path" default:"./file-uploader.toml"`
	Endpoint       string `short:"E" long:"endpoint" description:"API endpoint (format: environment,url)"`
	Address        string `short:"a" long:"address" description:"Listen address"`
	Port           int    `short:"p" long:"port" description:"Listen port"`
	Prefix         string `short:"P" long:"prefix" description:"URL prefix"`
	SigningKeyFile string `short:"s" long:"signing-key-file" description:"Path to file containing JWT signing key"`
	Mock           bool   `long:"mock" description:"Use mock implementations"`
	MockOutDir     string `long:"mock-output-dir" description:"Mock upload output directory" default:"./mock-output"`

	config *Config // internal, not a CLI flag
}

func main() {
	var args Args
	parser := flags.NewParser(&args, flags.Default)

	// Register subcommands
	parser.AddCommand("gen-config", "Generate default config", "Generate a default TOML configuration file to stdout", &GenConfigCommand{})
	// parser.AddCommand("gen-csv", ...)

	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			switch flagsErr.Type {
			case flags.ErrHelp:
				os.Exit(0)
			case flags.ErrCommandRequired:
				// No subcommand given — fall through to server startup
				if args.Version {
					PrintVersion()
					os.Exit(0)
				}
			default:
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	} else if parser.Active != nil {
		// A subcommand was executed successfully — exit
		os.Exit(0)
	}

	if args.Version {
		PrintVersion()
		os.Exit(0)
	}

	// Load TOML config (missing file is not an error)
	cfg, err := LoadConfig(args.ConfigFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	args.config = &cfg

	// Apply CLI overrides: CLI flag > TOML file > hardcoded default
	if args.Endpoint != "" {
		cfg.APIEndpoint = args.Endpoint
	}
	if args.Port != 0 {
		cfg.Port = args.Port
	}
	if args.SigningKeyFile != "" {
		data, err := os.ReadFile(args.SigningKeyFile)
		if err != nil {
			log.Fatalf("Failed to read signing key file %q: %v", args.SigningKeyFile, err)
		}
		cfg.SigningKey = strings.TrimSpace(string(data))
	}
	// Address and Prefix: only override TOML when the user explicitly passed the flag.
	// Since there's no default: tag, empty string means "not set on CLI".
	if args.Address != "" {
		cfg.Address = args.Address
	}
	if args.Prefix != "" {
		cfg.Prefix = args.Prefix
	}

	// Validate server fields (address and port)
	if err := cfg.ValidateServerFields(); err != nil {
		log.Fatalf("Invalid server config: %v", err)
	}

	// Convert to ApplicationConfig
	appCfg := cfg.ToApplicationConfig()

	// Ensure all directories exist
	dirs := []string{
		appCfg.PlayersDBWorkDir,
		appCfg.CSVUploadDir,
		appCfg.CSVProcessingDir,
		appCfg.CSVUploadingDir,
		appCfg.CSVArchiveDir,
	}
	for _, dir := range dirs {
		if err := util.EnsureDirs(dir); err != nil {
			log.Fatalf("Failed to create directory %q: %v", dir, err)
		}
	}

	// Server startup (implemented in spec 11)
	_ = appCfg
	fmt.Println("Server startup not yet implemented")
}
