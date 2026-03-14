package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/mock"
	"github.com/corruptmemory/file-uploader-r3/internal/server"
	"github.com/corruptmemory/file-uploader-r3/internal/setup"
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
	SecureCookies  bool   `long:"secure-cookies" description:"Set Secure flag on cookies (enable behind TLS-terminating proxy)"`
	Mock           bool   `long:"mock" description:"Use mock implementations"`
	MockOutDir     string `long:"mock-output-dir" description:"Mock upload output directory" default:"./mock-output"`

	config *Config // internal, not a CLI flag
}

func main() {
	var args Args
	parser := flags.NewParser(&args, flags.Default)

	// Register subcommands
	parser.AddCommand("gen-config", "Generate default config", "Generate a default TOML configuration file to stdout", &GenConfigCommand{})
	parser.AddCommand("gen-csv", "Generate synthetic CSV", "Generate synthetic CSV test data for a given type", &GenCSVCommand{})

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
		fi, err := os.Stat(args.SigningKeyFile)
		if err != nil {
			log.Fatalf("Failed to stat signing key file %q: %v", args.SigningKeyFile, err)
		}
		if perm := fi.Mode().Perm(); perm&0077 != 0 {
			log.Fatalf("Signing key file %q has overly permissive mode %04o; must not be group/world accessible (e.g. 0600)", args.SigningKeyFile, perm)
		}
		data, err := os.ReadFile(args.SigningKeyFile)
		if err != nil {
			log.Fatalf("Failed to read signing key file %q: %v", args.SigningKeyFile, err)
		}
		cfg.SigningKey = strings.TrimSpace(string(data))
	}
	// Address and Prefix: only override TOML when the user explicitly passed the flag.
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

	// Determine signing key
	signingKey := []byte(cfg.SigningKey)
	if len(signingKey) == 0 {
		// Use a default key in mock mode; require it otherwise
		if args.Mock {
			signingKey = []byte("mock-development-signing-key")
			log.Println("WARNING: Using hardcoded mock signing key — DO NOT use in production")
		} else {
			log.Fatal("Signing key required: set 'signing-key' in config or use --signing-key-file")
		}
	}

	// Create auth provider (mock or placeholder for real — real impl comes in later specs)
	var authProvider app.AuthProvider
	if args.Mock {
		authProvider = &mock.MockAuthProvider{}
	} else if appCfg.NeedsSetup() {
		// During setup, use the mock auth provider as a placeholder. The setup wizard
		// needs an AuthProvider for the registration code step; the real provider will
		// be configured after setup completes.
		authProvider = &mock.MockAuthProvider{}
	} else {
		log.Fatal("Non-mock mode requires a real auth provider. Use --mock for development or implement a real AuthProvider.")
	}

	// Determine initial state
	var initialStateBuilder app.StateBuilder
	configWriter := cfg.WriteFile(args.ConfigFile)
	if args.Mock || !appCfg.NeedsSetup() {
		// Mock mode: fill in default values for settable fields so the
		// settings page isn't blank (setup wizard is skipped in mock mode).
		if args.Mock && appCfg.NeedsSetup() {
			if appCfg.Endpoint == "" {
				appCfg = appCfg.WithEndpoint("http://localhost:8080/api")
			}
			if appCfg.Environment == "" {
				appCfg = appCfg.WithEnvironment("mock")
			}
			if appCfg.ServiceCredentials == "" {
				appCfg = appCfg.WithServiceCredentials("mock-credentials")
			}
			if appCfg.OrgPlayerIDPepper == "" {
				appCfg = appCfg.WithOrgPlayerIDPepper("mock-pepper-value")
			}
			if appCfg.OrgPlayerIDHash == "" {
				appCfg = appCfg.WithOrgPlayerIDHash("argon2")
			}
			if appCfg.UsePlayersDB == "" {
				appCfg = appCfg.WithUsePlayersDB("false")
			}
		}
		capturedCfg := appCfg
		initialStateBuilder = func(a *app.Application) (app.Stoppable, error) {
			uploader := mock.NewMockUploader(args.MockOutDir, mock.WithDelay(100*time.Millisecond))
			return app.NewRunningApp(capturedCfg, uploader, authProvider, configWriter)
		}
	} else {
		// Needs setup: start in SetupApp
		runningAppBuilder := func(a *app.Application) (app.Stoppable, error) {
			updatedCfg := cfg.ToApplicationConfig()
			uploader := mock.NewMockUploader(args.MockOutDir, mock.WithDelay(100*time.Millisecond))
			return app.NewRunningApp(updatedCfg, uploader, authProvider, configWriter)
		}
		capturedCfg := appCfg
		initialStateBuilder = func(a *app.Application) (app.Stoppable, error) {
			return setup.NewSetupApp(a, capturedCfg, authProvider, configWriter, runningAppBuilder, ""), nil
		}
	}

	// Create Application
	application := app.NewApplication(initialStateBuilder)

	// Create static FS sub-tree
	staticSub, err := fs.Sub(StaticFS, "static")
	if err != nil {
		log.Fatalf("Failed to create static filesystem: %v", err)
	}

	// Create WebApp and chi router
	listenAddr := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	_, router := server.NewWebApp(application, authProvider, signingKey, appCfg.CSVUploadDir, GitVersion, cfg.Prefix, staticSub, args.SecureCookies)

	// Create and start Server
	srv := server.NewServer(listenAddr, router, nil)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Server listening on %s", listenAddr)
	if cfg.Prefix != "" {
		log.Printf("URL prefix: %s", cfg.Prefix)
	}
	if args.Mock {
		log.Printf("Running in MOCK mode")
		if cfg.Address != "127.0.0.1" && cfg.Address != "localhost" && cfg.Address != "::1" && cfg.Address != "" {
			log.Printf("WARNING: Mock mode is listening on non-localhost address %q — this is not recommended", cfg.Address)
		}
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	srv.Stop()
	srv.Wait()
	application.Stop()
	application.Wait()
	log.Println("Shutdown complete")
}
