package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestToApplicationConfigParsesAPIEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIEndpoint = "prod,https://api.example.com"

	ac := cfg.ToApplicationConfig()

	if ac.Environment != "prod" {
		t.Errorf("Environment = %q, want %q", ac.Environment, "prod")
	}
	if ac.Endpoint != "https://api.example.com" {
		t.Errorf("Endpoint = %q, want %q", ac.Endpoint, "https://api.example.com")
	}
}

func TestToApplicationConfigEmptyEndpointUsesDefaults(t *testing.T) {
	// Save and restore build-time defaults
	origEnv := PublicAPIEnvironment
	origEndpoint := PublicAPIEndpoint
	defer func() {
		PublicAPIEnvironment = origEnv
		PublicAPIEndpoint = origEndpoint
	}()

	PublicAPIEnvironment = "default-env"
	PublicAPIEndpoint = "https://default.example.com"

	cfg := DefaultConfig()
	cfg.APIEndpoint = ""

	ac := cfg.ToApplicationConfig()

	if ac.Environment != "default-env" {
		t.Errorf("Environment = %q, want %q", ac.Environment, "default-env")
	}
	if ac.Endpoint != "https://default.example.com" {
		t.Errorf("Endpoint = %q, want %q", ac.Endpoint, "https://default.example.com")
	}
}

func TestToApplicationConfigUsePlayersDB(t *testing.T) {
	cfg := DefaultConfig()

	cfg.UsePlayersDB = false
	ac := cfg.ToApplicationConfig()
	if ac.UsePlayersDB != "false" {
		t.Errorf("UsePlayersDB = %q, want %q", ac.UsePlayersDB, "false")
	}

	cfg.UsePlayersDB = true
	ac = cfg.ToApplicationConfig()
	if ac.UsePlayersDB != "true" {
		t.Errorf("UsePlayersDB = %q, want %q", ac.UsePlayersDB, "true")
	}
}

func TestLoadConfigMissingFileNotError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.toml")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig with missing file returned error: %v", err)
	}

	// Should get defaults
	if cfg.Address != "0.0.0.0" {
		t.Errorf("Address = %q, want %q", cfg.Address, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
}

func TestLoadConfigValidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.toml")

	content := `
api-endpoint = "staging,https://staging.example.com"
address = "127.0.0.1"
port = 9090
signing-key = "test-key"
service-credentials = "test-creds"
use-players-db = true

[org]
org-playerID-pepper = "my-pepper"
org-playerID-hash = "argon2"

[players-db]
work-dir = "/tmp/players-db"

[data-processing]
upload-dir = "/tmp/upload"
processing-dir = "/tmp/processing"
uploading-dir = "/tmp/uploading"
archive-dir = "/tmp/archive"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.APIEndpoint != "staging,https://staging.example.com" {
		t.Errorf("APIEndpoint = %q", cfg.APIEndpoint)
	}
	if cfg.Address != "127.0.0.1" {
		t.Errorf("Address = %q", cfg.Address)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.SigningKey != "test-key" {
		t.Errorf("SigningKey = %q", cfg.SigningKey)
	}
	if cfg.ServiceCredentials != "test-creds" {
		t.Errorf("ServiceCredentials = %q", cfg.ServiceCredentials)
	}
	if !cfg.UsePlayersDB {
		t.Error("UsePlayersDB should be true")
	}
	if cfg.Org.OrgPlayerIDPepper != "my-pepper" {
		t.Errorf("OrgPlayerIDPepper = %q", cfg.Org.OrgPlayerIDPepper)
	}
	if cfg.DataProcessing.UploadDir != "/tmp/upload" {
		t.Errorf("UploadDir = %q", cfg.DataProcessing.UploadDir)
	}
}

func TestConfigWriteFileRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "roundtrip.toml")

	original := DefaultConfig()
	original.APIEndpoint = "prod,https://api.example.com"
	original.SigningKey = "my-signing-key"
	original.ServiceCredentials = "my-creds"
	original.Org.OrgPlayerIDPepper = "pepper12345"
	original.UsePlayersDB = true

	// Convert to ApplicationConfig
	ac := original.ToApplicationConfig()

	// Write via closure
	writer := original.WriteFile(path)
	if err := writer(ac); err != nil {
		t.Fatalf("WriteFile closure returned error: %v", err)
	}

	// Read back
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	// Compare key fields
	loadedAC := loaded.ToApplicationConfig()

	if loadedAC.Endpoint != ac.Endpoint {
		t.Errorf("Endpoint = %q, want %q", loadedAC.Endpoint, ac.Endpoint)
	}
	if loadedAC.Environment != ac.Environment {
		t.Errorf("Environment = %q, want %q", loadedAC.Environment, ac.Environment)
	}
	if loadedAC.OrgPlayerIDPepper != ac.OrgPlayerIDPepper {
		t.Errorf("OrgPlayerIDPepper = %q, want %q", loadedAC.OrgPlayerIDPepper, ac.OrgPlayerIDPepper)
	}
	if loadedAC.UsePlayersDB != ac.UsePlayersDB {
		t.Errorf("UsePlayersDB = %q, want %q", loadedAC.UsePlayersDB, ac.UsePlayersDB)
	}
	if loadedAC.CSVUploadDir != ac.CSVUploadDir {
		t.Errorf("CSVUploadDir = %q, want %q", loadedAC.CSVUploadDir, ac.CSVUploadDir)
	}
}

func TestGenConfigOutputsValidTOML(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	cmd := &GenConfigCommand{}
	if err := cmd.Execute(nil); err != nil {
		w.Close()
		os.Stdout = old
		t.Fatalf("GenConfigCommand.Execute returned error: %v", err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("gen-config produced empty output")
	}

	// Parse as TOML
	var parsed Config
	if _, err := toml.Decode(output, &parsed); err != nil {
		t.Fatalf("gen-config output is not valid TOML: %v\nOutput:\n%s", err, output)
	}

	// Verify defaults
	if parsed.Address != "0.0.0.0" {
		t.Errorf("Address = %q, want %q", parsed.Address, "0.0.0.0")
	}
	if parsed.Port != 8080 {
		t.Errorf("Port = %d, want %d", parsed.Port, 8080)
	}
	if parsed.Org.OrgPlayerIDHash != "argon2" {
		t.Errorf("OrgPlayerIDHash = %q, want %q", parsed.Org.OrgPlayerIDHash, "argon2")
	}
	if parsed.DataProcessing.UploadDir != "./data-processing/upload" {
		t.Errorf("UploadDir = %q, want %q", parsed.DataProcessing.UploadDir, "./data-processing/upload")
	}
}

func TestToApplicationConfigEndpointWithCommaInURL(t *testing.T) {
	cfg := DefaultConfig()
	// URL might contain commas after the first one
	cfg.APIEndpoint = "prod,https://api.example.com/path,extra"

	ac := cfg.ToApplicationConfig()

	if ac.Environment != "prod" {
		t.Errorf("Environment = %q, want %q", ac.Environment, "prod")
	}
	if ac.Endpoint != "https://api.example.com/path,extra" {
		t.Errorf("Endpoint = %q, want %q", ac.Endpoint, "https://api.example.com/path,extra")
	}
}
