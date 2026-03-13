package app

import (
	"testing"
)

func TestNeedsSetup(t *testing.T) {
	tests := []struct {
		name string
		cfg  ApplicationConfig
		want bool
	}{
		{
			name: "true when pepper empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "https://api.example.com",
				Environment:        "prod",
				ServiceCredentials: "creds",
				UsePlayersDB:       "false",
			},
			want: true,
		},
		{
			name: "true when hash empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "",
				Endpoint:           "https://api.example.com",
				Environment:        "prod",
				ServiceCredentials: "creds",
				UsePlayersDB:       "false",
			},
			want: true,
		},
		{
			name: "true when endpoint empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "",
				Environment:        "prod",
				ServiceCredentials: "creds",
				UsePlayersDB:       "false",
			},
			want: true,
		},
		{
			name: "true when environment empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "https://api.example.com",
				Environment:        "",
				ServiceCredentials: "creds",
				UsePlayersDB:       "false",
			},
			want: true,
		},
		{
			name: "true when service credentials empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "https://api.example.com",
				Environment:        "prod",
				ServiceCredentials: "",
				UsePlayersDB:       "false",
			},
			want: true,
		},
		{
			name: "true when use-players-db empty",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "https://api.example.com",
				Environment:        "prod",
				ServiceCredentials: "creds",
				UsePlayersDB:       "",
			},
			want: true,
		},
		{
			name: "false when all set",
			cfg: ApplicationConfig{
				OrgPlayerIDPepper:  "pepper123",
				OrgPlayerIDHash:    "argon2",
				Endpoint:           "https://api.example.com",
				Environment:        "prod",
				ServiceCredentials: "creds",
				UsePlayersDB:       "false",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.NeedsSetup()
			if got != tt.want {
				t.Errorf("NeedsSetup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	cfg := ApplicationConfig{
		OrgPlayerIDPepper:  "  pepper  ",
		OrgPlayerIDHash:    " ARGON2 ",
		Endpoint:           "  https://api.example.com  ",
		Environment:        "  prod  ",
		ServiceCredentials: "  creds  ",
		UsePlayersDB:       "  TRUE  ",
		PlayersDBWorkDir:   "  /tmp/work  ",
		CSVUploadDir:       "  /tmp/upload  ",
		CSVProcessingDir:   "  /tmp/proc  ",
		CSVUploadingDir:    "  /tmp/uploading  ",
		CSVArchiveDir:      "  /tmp/archive  ",
	}

	cleaned := cfg.Cleanup()

	if cleaned.OrgPlayerIDPepper != "pepper" {
		t.Errorf("OrgPlayerIDPepper = %q, want %q", cleaned.OrgPlayerIDPepper, "pepper")
	}
	if cleaned.OrgPlayerIDHash != "argon2" {
		t.Errorf("OrgPlayerIDHash = %q, want %q", cleaned.OrgPlayerIDHash, "argon2")
	}
	if cleaned.Endpoint != "https://api.example.com" {
		t.Errorf("Endpoint = %q, want %q", cleaned.Endpoint, "https://api.example.com")
	}
	if cleaned.UsePlayersDB != "true" {
		t.Errorf("UsePlayersDB = %q, want %q", cleaned.UsePlayersDB, "true")
	}
	if cleaned.PlayersDBWorkDir != "/tmp/work" {
		t.Errorf("PlayersDBWorkDir = %q, want %q", cleaned.PlayersDBWorkDir, "/tmp/work")
	}
}

func TestWithMethodsReturnCopies(t *testing.T) {
	original := ApplicationConfig{
		OrgPlayerIDPepper:  "original-pepper",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://original.example.com",
		Environment:        "prod",
		ServiceCredentials: "original-creds",
		UsePlayersDB:       "false",
		PlayersDBWorkDir:   "/original/work",
		CSVUploadDir:       "/original/upload",
		CSVProcessingDir:   "/original/proc",
		CSVUploadingDir:    "/original/uploading",
		CSVArchiveDir:      "/original/archive",
	}

	tests := []struct {
		name     string
		fn       func(ApplicationConfig) ApplicationConfig
		checkOld func(ApplicationConfig) bool
		checkNew func(ApplicationConfig) bool
	}{
		{
			name:     "WithOrgPlayerIDPepper",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithOrgPlayerIDPepper("new-pepper") },
			checkOld: func(c ApplicationConfig) bool { return c.OrgPlayerIDPepper == "original-pepper" },
			checkNew: func(c ApplicationConfig) bool { return c.OrgPlayerIDPepper == "new-pepper" },
		},
		{
			name:     "WithOrgPlayerIDHash",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithOrgPlayerIDHash("ARGON2") },
			checkOld: func(c ApplicationConfig) bool { return c.OrgPlayerIDHash == "argon2" },
			checkNew: func(c ApplicationConfig) bool { return c.OrgPlayerIDHash == "argon2" }, // lowercased
		},
		{
			name:     "WithEndpoint",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithEndpoint("https://new.example.com") },
			checkOld: func(c ApplicationConfig) bool { return c.Endpoint == "https://original.example.com" },
			checkNew: func(c ApplicationConfig) bool { return c.Endpoint == "https://new.example.com" },
		},
		{
			name:     "WithEnvironment",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithEnvironment("staging") },
			checkOld: func(c ApplicationConfig) bool { return c.Environment == "prod" },
			checkNew: func(c ApplicationConfig) bool { return c.Environment == "staging" },
		},
		{
			name:     "WithServiceCredentials",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithServiceCredentials("new-creds") },
			checkOld: func(c ApplicationConfig) bool { return c.ServiceCredentials == "original-creds" },
			checkNew: func(c ApplicationConfig) bool { return c.ServiceCredentials == "new-creds" },
		},
		{
			name:     "WithUsePlayersDB",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithUsePlayersDB("TRUE") },
			checkOld: func(c ApplicationConfig) bool { return c.UsePlayersDB == "false" },
			checkNew: func(c ApplicationConfig) bool { return c.UsePlayersDB == "true" }, // lowercased
		},
		{
			name:     "WithPlayersDBWorkDir",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithPlayersDBWorkDir("/new/work") },
			checkOld: func(c ApplicationConfig) bool { return c.PlayersDBWorkDir == "/original/work" },
			checkNew: func(c ApplicationConfig) bool { return c.PlayersDBWorkDir == "/new/work" },
		},
		{
			name:     "WithCSVUploadDir",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithCSVUploadDir("/new/upload") },
			checkOld: func(c ApplicationConfig) bool { return c.CSVUploadDir == "/original/upload" },
			checkNew: func(c ApplicationConfig) bool { return c.CSVUploadDir == "/new/upload" },
		},
		{
			name:     "WithCSVProcessingDir",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithCSVProcessingDir("/new/proc") },
			checkOld: func(c ApplicationConfig) bool { return c.CSVProcessingDir == "/original/proc" },
			checkNew: func(c ApplicationConfig) bool { return c.CSVProcessingDir == "/new/proc" },
		},
		{
			name:     "WithCSVUploadingDir",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithCSVUploadingDir("/new/uploading") },
			checkOld: func(c ApplicationConfig) bool { return c.CSVUploadingDir == "/original/uploading" },
			checkNew: func(c ApplicationConfig) bool { return c.CSVUploadingDir == "/new/uploading" },
		},
		{
			name:     "WithCSVArchiveDir",
			fn:       func(c ApplicationConfig) ApplicationConfig { return c.WithCSVArchiveDir("/new/archive") },
			checkOld: func(c ApplicationConfig) bool { return c.CSVArchiveDir == "/original/archive" },
			checkNew: func(c ApplicationConfig) bool { return c.CSVArchiveDir == "/new/archive" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newCfg := tt.fn(original)

			if !tt.checkOld(original) {
				t.Error("original was mutated")
			}
			if !tt.checkNew(newCfg) {
				t.Error("new config does not have expected value")
			}
		})
	}
}

func TestMergeSettableValuesCorrectTags(t *testing.T) {
	base := ApplicationConfig{
		OrgPlayerIDPepper:  "pepper123",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://api.example.com",
		Environment:        "prod",
		ServiceCredentials: "creds",
		UsePlayersDB:       "false",
	}

	other := ApplicationConfig{
		OrgPlayerIDPepper:  "pepper123",               // unchanged
		OrgPlayerIDHash:    "argon2",                  // unchanged
		Endpoint:           "https://api.example.com", // unchanged
		Environment:        "prod",                    // unchanged
		ServiceCredentials: "creds",                   // unchanged
		UsePlayersDB:       "true",                    // CHANGED
	}

	merged, tags := base.MergeSettableValues(other)

	if merged.UsePlayersDB != "true" {
		t.Errorf("merged.UsePlayersDB = %q, want %q", merged.UsePlayersDB, "true")
	}

	if len(tags) != 1 {
		t.Fatalf("expected 1 changed tag, got %d: %v", len(tags), tags)
	}

	if tags[0] != ACEUsePlayersDB {
		t.Errorf("expected ACEUsePlayersDB tag, got %v", tags[0])
	}
}

func TestMergeSettableValuesMultipleChanges(t *testing.T) {
	base := ApplicationConfig{
		OrgPlayerIDPepper:  "pepper123",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://api.example.com",
		Environment:        "prod",
		ServiceCredentials: "creds",
		UsePlayersDB:       "false",
	}

	other := ApplicationConfig{
		OrgPlayerIDPepper:  "new-pepper",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://new-api.example.com",
		Environment:        "staging",
		ServiceCredentials: "new-creds",
		UsePlayersDB:       "true",
	}

	merged, tags := base.MergeSettableValues(other)

	if merged.OrgPlayerIDPepper != "new-pepper" {
		t.Errorf("merged.OrgPlayerIDPepper = %q, want %q", merged.OrgPlayerIDPepper, "new-pepper")
	}

	// Should have 5 changes (all except OrgPlayerIDHash)
	if len(tags) != 5 {
		t.Fatalf("expected 5 changed tags, got %d: %v", len(tags), tags)
	}

	// Verify expected tags are present
	tagSet := make(map[ApplicationConfigErrorTag]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	expected := []ApplicationConfigErrorTag{
		ACEOrgPlayerIDPepper,
		ACEEndpoint,
		ACEEnvironment,
		ACEServiceCredentials,
		ACEUsePlayersDB,
	}
	for _, e := range expected {
		if !tagSet[e] {
			t.Errorf("missing expected tag %v", e)
		}
	}
}

func TestValidateSettableValuesCollectsAllErrors(t *testing.T) {
	cfg := ApplicationConfig{
		OrgPlayerIDPepper:  "",     // empty
		OrgPlayerIDHash:    "sha1", // unknown
		Endpoint:           "",     // empty
		Environment:        "",     // empty
		ServiceCredentials: "",     // empty
		UsePlayersDB:       "yes",  // invalid
	}

	err := cfg.ValidateSettableValues(nil)
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}

	errs, ok := err.(*ApplicationConfigErrors)
	if !ok {
		t.Fatalf("expected *ApplicationConfigErrors, got %T", err)
	}

	// Should have errors for all 6 fields
	if len(errs.Errors) != 6 {
		t.Errorf("expected 6 errors, got %d", len(errs.Errors))
		for _, e := range errs.Errors {
			t.Logf("  %s", e.Error())
		}
	}

	// Check filter methods return correct errors
	if len(errs.GetPlayerIDPepperError()) != 1 {
		t.Errorf("expected 1 pepper error, got %d", len(errs.GetPlayerIDPepperError()))
	}
	if len(errs.GetPlayerIDHashError()) != 1 {
		t.Errorf("expected 1 hash error, got %d", len(errs.GetPlayerIDHashError()))
	}
	if len(errs.GetEndpointError()) != 1 {
		t.Errorf("expected 1 endpoint error, got %d", len(errs.GetEndpointError()))
	}
	if len(errs.GetEnvironmentError()) != 1 {
		t.Errorf("expected 1 environment error, got %d", len(errs.GetEnvironmentError()))
	}
	if len(errs.GetServiceCredentialsError()) != 1 {
		t.Errorf("expected 1 service-credentials error, got %d", len(errs.GetServiceCredentialsError()))
	}
	if len(errs.GetUsePlayersDBError()) != 1 {
		t.Errorf("expected 1 use-players-db error, got %d", len(errs.GetUsePlayersDBError()))
	}
}

func TestValidateSettableValuesValid(t *testing.T) {
	cfg := ApplicationConfig{
		OrgPlayerIDPepper:  "pepper123",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://api.example.com",
		Environment:        "prod",
		ServiceCredentials: "creds",
		UsePlayersDB:       "false",
	}

	err := cfg.ValidateSettableValues(nil)
	if err != nil {
		t.Errorf("expected no errors, got: %v", err)
	}
}

func TestValidateSettableValuesPepperTooShort(t *testing.T) {
	cfg := ApplicationConfig{
		OrgPlayerIDPepper:  "abc",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "https://api.example.com",
		Environment:        "prod",
		ServiceCredentials: "creds",
		UsePlayersDB:       "false",
	}

	err := cfg.ValidateSettableValues(nil)
	if err == nil {
		t.Fatal("expected validation error for short pepper")
	}

	errs := err.(*ApplicationConfigErrors)
	if len(errs.GetPlayerIDPepperError()) != 1 {
		t.Errorf("expected 1 pepper error, got %d", len(errs.GetPlayerIDPepperError()))
	}
}

func TestValidateSettableValuesInvalidEndpointURL(t *testing.T) {
	cfg := ApplicationConfig{
		OrgPlayerIDPepper:  "pepper123",
		OrgPlayerIDHash:    "argon2",
		Endpoint:           "not-a-url",
		Environment:        "prod",
		ServiceCredentials: "creds",
		UsePlayersDB:       "false",
	}

	err := cfg.ValidateSettableValues(nil)
	if err == nil {
		t.Fatal("expected validation error for invalid URL endpoint")
	}

	errs := err.(*ApplicationConfigErrors)
	if len(errs.GetEndpointError()) != 1 {
		t.Errorf("expected 1 endpoint error, got %d", len(errs.GetEndpointError()))
	}
}

func TestUsePlayersDBValue(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"", false},
		{"yes", false},
	}

	for _, tt := range tests {
		cfg := ApplicationConfig{UsePlayersDB: tt.value}
		if got := cfg.UsePlayersDBValue(); got != tt.want {
			t.Errorf("UsePlayersDBValue() with %q = %v, want %v", tt.value, got, tt.want)
		}
	}
}
