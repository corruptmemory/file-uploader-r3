package app

import (
	"fmt"
	"net/url"
	"strings"
)

// ApplicationConfig is a value type representing the resolved application configuration.
// All methods use value receivers and return copies.
type ApplicationConfig struct {
	OrgPlayerIDPepper  string
	OrgPlayerIDHash    string
	Endpoint           string
	Environment        string
	ServiceCredentials string
	UsePlayersDB       string // "true" or "false" as string
	PlayersDBWorkDir   string
	CSVUploadDir       string
	CSVProcessingDir   string
	CSVUploadingDir    string
	CSVArchiveDir      string
}

// Cleanup trims whitespace from all fields and lowercases OrgPlayerIDHash and UsePlayersDB.
func (c ApplicationConfig) Cleanup() ApplicationConfig {
	c.OrgPlayerIDPepper = strings.TrimSpace(c.OrgPlayerIDPepper)
	c.OrgPlayerIDHash = strings.ToLower(strings.TrimSpace(c.OrgPlayerIDHash))
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.Environment = strings.TrimSpace(c.Environment)
	c.ServiceCredentials = strings.TrimSpace(c.ServiceCredentials)
	c.UsePlayersDB = strings.ToLower(strings.TrimSpace(c.UsePlayersDB))
	c.PlayersDBWorkDir = strings.TrimSpace(c.PlayersDBWorkDir)
	c.CSVUploadDir = strings.TrimSpace(c.CSVUploadDir)
	c.CSVProcessingDir = strings.TrimSpace(c.CSVProcessingDir)
	c.CSVUploadingDir = strings.TrimSpace(c.CSVUploadingDir)
	c.CSVArchiveDir = strings.TrimSpace(c.CSVArchiveDir)
	return c
}

// NeedsSetup returns true if any required settable field is empty.
func (c ApplicationConfig) NeedsSetup() bool {
	return c.OrgPlayerIDPepper == "" ||
		c.OrgPlayerIDHash == "" ||
		c.Endpoint == "" ||
		c.Environment == "" ||
		c.ServiceCredentials == "" ||
		c.UsePlayersDB == ""
}

// UsePlayersDBValue returns true if UsePlayersDB is "true".
func (c ApplicationConfig) UsePlayersDBValue() bool {
	return c.UsePlayersDB == "true"
}

// With* methods — immutable setters that return copies.

func (c ApplicationConfig) WithOrgPlayerIDPepper(v string) ApplicationConfig {
	c.OrgPlayerIDPepper = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithOrgPlayerIDHash(v string) ApplicationConfig {
	c.OrgPlayerIDHash = strings.ToLower(strings.TrimSpace(v))
	return c
}

func (c ApplicationConfig) WithEndpoint(v string) ApplicationConfig {
	c.Endpoint = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithEnvironment(v string) ApplicationConfig {
	c.Environment = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithServiceCredentials(v string) ApplicationConfig {
	c.ServiceCredentials = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithUsePlayersDB(v string) ApplicationConfig {
	c.UsePlayersDB = strings.ToLower(strings.TrimSpace(v))
	return c
}

func (c ApplicationConfig) WithPlayersDBWorkDir(v string) ApplicationConfig {
	c.PlayersDBWorkDir = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithCSVUploadDir(v string) ApplicationConfig {
	c.CSVUploadDir = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithCSVProcessingDir(v string) ApplicationConfig {
	c.CSVProcessingDir = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithCSVUploadingDir(v string) ApplicationConfig {
	c.CSVUploadingDir = strings.TrimSpace(v)
	return c
}

func (c ApplicationConfig) WithCSVArchiveDir(v string) ApplicationConfig {
	c.CSVArchiveDir = strings.TrimSpace(v)
	return c
}

// MergeSettableValues merges changed settable fields from other into c.
// Returns the merged copy and a list of tags for fields that changed.
func (c ApplicationConfig) MergeSettableValues(other ApplicationConfig) (ApplicationConfig, []ApplicationConfigErrorTag) {
	var changed []ApplicationConfigErrorTag

	if other.OrgPlayerIDPepper != c.OrgPlayerIDPepper {
		c.OrgPlayerIDPepper = other.OrgPlayerIDPepper
		changed = append(changed, ACEOrgPlayerIDPepper)
	}
	if other.OrgPlayerIDHash != c.OrgPlayerIDHash {
		c.OrgPlayerIDHash = other.OrgPlayerIDHash
		changed = append(changed, ACEOrgPlayerIDHash)
	}
	if other.Endpoint != c.Endpoint {
		c.Endpoint = other.Endpoint
		changed = append(changed, ACEEndpoint)
	}
	if other.Environment != c.Environment {
		c.Environment = other.Environment
		changed = append(changed, ACEEnvironment)
	}
	if other.ServiceCredentials != c.ServiceCredentials {
		c.ServiceCredentials = other.ServiceCredentials
		changed = append(changed, ACEServiceCredentials)
	}
	if other.UsePlayersDB != c.UsePlayersDB {
		c.UsePlayersDB = other.UsePlayersDB
		changed = append(changed, ACEUsePlayersDB)
	}

	return c, changed
}

// ValidateSettableValues validates all settable fields, collecting ALL errors (fail-soft).
// logFunc is called for each validation error found.
func (c ApplicationConfig) ValidateSettableValues(logFunc func(string, ...any)) error {
	errs := &ApplicationConfigErrors{}

	// OrgPlayerIDPepper: non-empty, min 5 chars
	if c.OrgPlayerIDPepper == "" {
		e := &ApplicationConfigError{Tag: ACEOrgPlayerIDPepper, Err: fmt.Errorf("must not be empty")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	} else if len(c.OrgPlayerIDPepper) < 5 {
		e := &ApplicationConfigError{Tag: ACEOrgPlayerIDPepper, Err: fmt.Errorf("must be at least 5 characters")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	// OrgPlayerIDHash: must be known hasher name (only "argon2")
	if c.OrgPlayerIDHash == "" {
		e := &ApplicationConfigError{Tag: ACEOrgPlayerIDHash, Err: fmt.Errorf("must not be empty")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	} else if c.OrgPlayerIDHash != "argon2" {
		e := &ApplicationConfigError{Tag: ACEOrgPlayerIDHash, Err: fmt.Errorf("unknown hash algorithm %q, must be \"argon2\"", c.OrgPlayerIDHash)}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	// Endpoint: must not be empty, must be a valid URL
	if c.Endpoint == "" {
		e := &ApplicationConfigError{Tag: ACEEndpoint, Err: fmt.Errorf("must not be empty")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	} else if u, err := url.Parse(c.Endpoint); err != nil || u.Scheme == "" || u.Host == "" {
		e := &ApplicationConfigError{Tag: ACEEndpoint, Err: fmt.Errorf("must be a valid URL with scheme and host")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	// Environment: must not be empty, 3-15 chars
	if c.Environment == "" {
		e := &ApplicationConfigError{Tag: ACEEnvironment, Err: fmt.Errorf("must not be empty")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	} else if len(c.Environment) < 3 || len(c.Environment) > 15 {
		e := &ApplicationConfigError{Tag: ACEEnvironment, Err: fmt.Errorf("must be 3-15 characters, got %d", len(c.Environment))}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	// ServiceCredentials: must not be empty
	if c.ServiceCredentials == "" {
		e := &ApplicationConfigError{Tag: ACEServiceCredentials, Err: fmt.Errorf("must not be empty")}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	// UsePlayersDB: must be "true" or "false"
	if c.UsePlayersDB != "true" && c.UsePlayersDB != "false" {
		e := &ApplicationConfigError{Tag: ACEUsePlayersDB, Err: fmt.Errorf("must be \"true\" or \"false\", got %q", c.UsePlayersDB)}
		errs.Add(e)
		if logFunc != nil {
			logFunc("validation error: %s", e.Error())
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}
