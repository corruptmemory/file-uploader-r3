package main

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/mitchellh/go-homedir"
)

// Config represents the TOML configuration file schema.
type Config struct {
	APIEndpoint        string `toml:"api-endpoint"`
	Address            string `toml:"address"`
	Port               int    `toml:"port"`
	Prefix             string `toml:"prefix"`
	SigningKey         string `toml:"signing-key"`
	ServiceCredentials string `toml:"service-credentials"`
	UsePlayersDB       bool   `toml:"use-players-db"`

	Org            OrgConfig            `toml:"org"`
	PlayersDB      PlayersDBConfig      `toml:"players-db"`
	DataProcessing DataProcessingConfig `toml:"data-processing"`
}

// OrgConfig holds organization-specific hashing configuration.
type OrgConfig struct {
	OrgPlayerIDPepper string `toml:"org-playerID-pepper"`
	OrgPlayerIDHash   string `toml:"org-playerID-hash"`
}

// PlayersDBConfig holds players database directory configuration.
type PlayersDBConfig struct {
	WorkDir string `toml:"work-dir"`
}

// DataProcessingConfig holds CSV processing directory configuration.
type DataProcessingConfig struct {
	UploadDir     string `toml:"upload-dir"`
	ProcessingDir string `toml:"processing-dir"`
	UploadingDir  string `toml:"uploading-dir"`
	ArchiveDir    string `toml:"archive-dir"`
}

// DefaultConfig returns a Config with all hardcoded defaults.
func DefaultConfig() Config {
	return Config{
		APIEndpoint:        "",
		Address:            "127.0.0.1",
		Port:               8080,
		Prefix:             "",
		SigningKey:         "",
		ServiceCredentials: "",
		UsePlayersDB:       false,
		Org: OrgConfig{
			OrgPlayerIDPepper: "",
			OrgPlayerIDHash:   "argon2",
		},
		PlayersDB: PlayersDBConfig{
			WorkDir: "./players-db/work",
		},
		DataProcessing: DataProcessingConfig{
			UploadDir:     "./data-processing/upload",
			ProcessingDir: "./data-processing/processing",
			UploadingDir:  "./data-processing/uploading",
			ArchiveDir:    "./data-processing/archive",
		},
	}
}

// LoadConfig loads configuration from a TOML file. Missing file is NOT an error;
// defaults are returned instead.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("stat config file: %w", err)
	}

	_, err = toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("decode config file: %w", err)
	}

	return cfg, nil
}

// expandPath expands ~ in paths using go-homedir and cleans the result
// to canonicalize any path traversal sequences.
func expandPath(path string) string {
	if path == "" {
		return path
	}
	expanded, err := homedir.Expand(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(expanded)
}

// ToApplicationConfig converts a Config to an ApplicationConfig.
// It parses the api-endpoint field and falls back to build-time defaults.
func (c *Config) ToApplicationConfig() app.ApplicationConfig {
	var environment, endpoint string

	if c.APIEndpoint != "" {
		// Split on first comma
		idx := strings.Index(c.APIEndpoint, ",")
		if idx >= 0 {
			environment = c.APIEndpoint[:idx]
			endpoint = c.APIEndpoint[idx+1:]
		} else {
			endpoint = c.APIEndpoint
		}
	}

	// Fall back to build-time defaults if empty
	if environment == "" {
		environment = PublicAPIEnvironment
	}
	if endpoint == "" {
		endpoint = PublicAPIEndpoint
	}

	usePlayersDB := "false"
	if c.UsePlayersDB {
		usePlayersDB = "true"
	}

	return app.ApplicationConfig{
		OrgPlayerIDPepper:  c.Org.OrgPlayerIDPepper,
		OrgPlayerIDHash:    c.Org.OrgPlayerIDHash,
		Endpoint:           endpoint,
		Environment:        environment,
		ServiceCredentials: c.ServiceCredentials,
		UsePlayersDB:       usePlayersDB,
		PlayersDBWorkDir:   expandPath(c.PlayersDB.WorkDir),
		CSVUploadDir:       expandPath(c.DataProcessing.UploadDir),
		CSVProcessingDir:   expandPath(c.DataProcessing.ProcessingDir),
		CSVUploadingDir:    expandPath(c.DataProcessing.UploadingDir),
		CSVArchiveDir:      expandPath(c.DataProcessing.ArchiveDir),
	}
}

// WriteFile returns a closure that reverse-maps an ApplicationConfig back to
// a Config and writes it as TOML to the specified path.
func (c *Config) WriteFile(path string) func(app.ApplicationConfig) error {
	return func(ac app.ApplicationConfig) error {
		// Reconstruct APIEndpoint
		apiEndpoint := ""
		if ac.Environment != "" && ac.Endpoint != "" {
			apiEndpoint = ac.Environment + "," + ac.Endpoint
		}

		// Convert UsePlayersDB string to bool
		usePlayersDB := ac.UsePlayersDB == "true"

		out := Config{
			APIEndpoint:        apiEndpoint,
			Address:            c.Address,
			Port:               c.Port,
			Prefix:             c.Prefix,
			SigningKey:         c.SigningKey,
			ServiceCredentials: ac.ServiceCredentials,
			UsePlayersDB:       usePlayersDB,
			Org: OrgConfig{
				OrgPlayerIDPepper: ac.OrgPlayerIDPepper,
				OrgPlayerIDHash:   ac.OrgPlayerIDHash,
			},
			PlayersDB: PlayersDBConfig{
				WorkDir: ac.PlayersDBWorkDir,
			},
			DataProcessing: DataProcessingConfig{
				UploadDir:     ac.CSVUploadDir,
				ProcessingDir: ac.CSVProcessingDir,
				UploadingDir:  ac.CSVUploadingDir,
				ArchiveDir:    ac.CSVArchiveDir,
			},
		}

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("create config file: %w", err)
		}
		defer f.Close()

		encoder := toml.NewEncoder(f)
		if err := encoder.Encode(out); err != nil {
			return fmt.Errorf("encode config: %w", err)
		}

		return nil
	}
}

// ValidateServerFields validates address and port fields.
func (c *Config) ValidateServerFields() error {
	if _, err := netip.ParseAddr(c.Address); err != nil {
		return fmt.Errorf("invalid address %q: %w", c.Address, err)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be 1-65535, got %d", c.Port)
	}
	return nil
}
