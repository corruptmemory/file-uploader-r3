# 03 — Configuration

**Dependencies:** 01-project-scaffold.md (project compiles, build.sh works), 02-chanutil-and-interfaces.md (Stoppable interface).

**Produces:** TOML config schema, CLI argument parsing, `ApplicationConfig` value type, config validation, config writer, `gen-config` subcommand, directory initialization.

---

## 1. TOML Config File Schema

File path: specified via CLI flag `-c/--config-file`, default `./file-uploader.toml`. Missing file is NOT an error — proceed with defaults and enter setup wizard.

### Complete Schema

```toml
api-endpoint = "production,https://api.example.com"
address = "0.0.0.0"
port = 8080
prefix = ""
signing-key = "your-jwt-signing-key-here"
service-credentials = ""
use-players-db = false

[org]
org-playerID-pepper = ""
org-playerID-hash = "argon2"

[players-db]
work-dir = "./players-db/work"

[data-processing]
upload-dir = "./data-processing/upload"
processing-dir = "./data-processing/processing"
uploading-dir = "./data-processing/uploading"
archive-dir = "./data-processing/archive"
```

### Go Struct (`cmd/` package scope — place in the main package)

```go
type Config struct {
    APIEndpoint        string `toml:"api-endpoint"`
    Address            string `toml:"address"`
    Port               int    `toml:"port"`
    Prefix             string `toml:"prefix"`
    SigningKey          string `toml:"signing-key"`
    ServiceCredentials string `toml:"service-credentials"`
    UsePlayersDB       bool   `toml:"use-players-db"`

    Org            OrgConfig            `toml:"org"`
    PlayersDB      PlayersDBConfig      `toml:"players-db"`
    DataProcessing DataProcessingConfig `toml:"data-processing"`
}

type OrgConfig struct {
    OrgPlayerIDPepper string `toml:"org-playerID-pepper"`
    OrgPlayerIDHash   string `toml:"org-playerID-hash"`
}

type PlayersDBConfig struct {
    WorkDir string `toml:"work-dir"`
}

type DataProcessingConfig struct {
    UploadDir     string `toml:"upload-dir"`
    ProcessingDir string `toml:"processing-dir"`
    UploadingDir  string `toml:"uploading-dir"`
    ArchiveDir    string `toml:"archive-dir"`
}
```

### Field Validation

| Field | Type | Default | Validation |
|---|---|---|---|
| `api-endpoint` | string | `""` | Comma-separated `"environment,url"`. Empty → use build-time defaults. |
| `address` | string | `"0.0.0.0"` | Must parse via `net/netip.ParseAddr` |
| `port` | int | `8080` | 1–65535 |
| `signing-key` | string | `""` | Must not be empty at runtime |
| `service-credentials` | string | `""` | Validated by remote API auth |
| `use-players-db` | bool | `false` | N/A |
| `org-playerID-pepper` | string | `""` | Non-empty, min 5 chars |
| `org-playerID-hash` | string | `"argon2"` | Must be known hasher name (only `"argon2"`) |
| All dir paths | string | (see schema) | Must be writable |

**`api-endpoint` parsing:** Split on first comma → left = environment (3–15 chars), right = URL.

---

## 2. CLI Arguments

Update the `Args` struct from spec 01:

```go
type Args struct {
    Version    bool   `short:"v" long:"version" description:"Show version and exit"`
    ConfigFile string `short:"c" long:"config-file" description:"Config file path" default:"./file-uploader.toml"`
    Endpoint   string `short:"E" long:"endpoint" description:"API endpoint (format: environment,url)"`
    Address    string `short:"a" long:"address" description:"Listen address" default:"0.0.0.0"`
    Port       int    `short:"p" long:"port" description:"Listen port"`
    Prefix     string `short:"P" long:"prefix" description:"URL prefix"`
    SigningKey string `short:"s" long:"signing-key" description:"JWT signing key"`
    Mock       bool   `long:"mock" description:"Use mock implementations"`
    MockOutDir string `long:"mock-output-dir" description:"Mock upload output directory" default:"./mock-output"`

    config *Config // internal, not a CLI flag
}
```

**Precedence:** CLI flag > TOML file > hardcoded default.

---

## 3. ApplicationConfig (Internal Config)

Package: `internal/app/`

`ApplicationConfig` is a **value type** — all methods use value receivers and return copies.

```go
type ApplicationConfig struct {
    OrgPlayerIDPepper  string
    OrgPlayerIDHash    string
    Endpoint           string
    Environment        string
    ServiceCredentials string
    UsePlayersDB       string   // "true" or "false" as string
    PlayersDBWorkDir   string
    CSVUploadDir       string
    CSVProcessingDir   string
    CSVUploadingDir    string
    CSVArchiveDir      string
}
```

### Methods

**`Config.ToApplicationConfig() ApplicationConfig`:**
1. Parse `APIEndpoint` on first comma → environment, endpoint.
2. If empty, use `PublicAPIEnvironment` and `PublicAPIEndpoint` build-time defaults.
3. Convert `UsePlayersDB` bool → `"true"`/`"false"` string.
4. Expand `~` in all dir paths via `go-homedir`.

**`Cleanup() ApplicationConfig`:** Trim whitespace, lowercase `OrgPlayerIDHash` and `UsePlayersDB`.

**`NeedsSetup() bool`:** Returns `true` if ANY of these are empty: `OrgPlayerIDPepper`, `OrgPlayerIDHash`, `Endpoint`, `Environment`, `ServiceCredentials`, `UsePlayersDB`.

**`UsePlayersDBValue() bool`:** Returns `UsePlayersDB == "true"`.

**`With*` methods:** Immutable setters for each field. Trim whitespace. Lowercase `OrgPlayerIDHash` and `UsePlayersDB`.

```go
func (c ApplicationConfig) WithOrgPlayerIDPepper(v string) ApplicationConfig
func (c ApplicationConfig) WithOrgPlayerIDHash(v string) ApplicationConfig
func (c ApplicationConfig) WithEndpoint(v string) ApplicationConfig
func (c ApplicationConfig) WithEnvironment(v string) ApplicationConfig
func (c ApplicationConfig) WithServiceCredentials(v string) ApplicationConfig
func (c ApplicationConfig) WithUsePlayersDB(v string) ApplicationConfig
// ... and for each dir path
```

**`MergeSettableValues(other ApplicationConfig) (ApplicationConfig, []ApplicationConfigErrorTag)`:**

Merges changed settable fields from `other` into receiver. Returns merged copy + list of changed field tags.

Settable fields: `OrgPlayerIDPepper`, `OrgPlayerIDHash`, `Endpoint`, `Environment`, `ServiceCredentials`, `UsePlayersDB`.

**CRITICAL:** The tag for `UsePlayersDB` changes MUST be `ACEUsePlayersDB`, NOT `ACEServiceCredentials`.

---

## 4. Config Validation Errors

```go
type ApplicationConfigErrorTag int

const (
    ACEOrgPlayerIDPepper    ApplicationConfigErrorTag = iota
    ACEOrgPlayerIDHash
    ACEEndpoint
    ACEEnvironment
    ACEUsePlayersDB
    ACEServiceCredentials
)
```

```go
type ApplicationConfigError struct {
    Tag ApplicationConfigErrorTag
    Err error
}

func (e *ApplicationConfigError) Error() string      // "field-name: message"
func (e *ApplicationConfigError) ErrorPrefix() string // human-readable field name
func (e *ApplicationConfigError) Unwrap() error

type ApplicationConfigErrors struct {
    Errors []*ApplicationConfigError
}

func (e *ApplicationConfigErrors) HasErrors() bool
func (e *ApplicationConfigErrors) Add(err error)
func (e *ApplicationConfigErrors) Error() string

// Filter methods — return errors matching the given tag:
func (e *ApplicationConfigErrors) GetPlayerIDPepperError() []*ApplicationConfigError
func (e *ApplicationConfigErrors) GetPlayerIDHashError() []*ApplicationConfigError
func (e *ApplicationConfigErrors) GetEndpointError() []*ApplicationConfigError
func (e *ApplicationConfigErrors) GetEnvironmentError() []*ApplicationConfigError
func (e *ApplicationConfigErrors) GetUsePlayersDBError() []*ApplicationConfigError
func (e *ApplicationConfigErrors) GetServiceCredentialsError() []*ApplicationConfigError
```

**`ValidateSettableValues(logFunc func(string, ...any)) error`:** Validates all settable fields independently, collecting ALL errors (fail-soft). Returns `*ApplicationConfigErrors`.

---

## 5. Config Writer

```go
func (c *Config) WriteFile(path string) func(ApplicationConfig) error
```

Returns a closure that reverse-maps `ApplicationConfig` → `Config` and writes TOML to `path`.

Reverse mapping:
- Reconstruct `APIEndpoint` as `"environment,url"`
- Convert `UsePlayersDB` string → bool
- Map dir paths back to sub-structs

---

## 6. Directory Initialization

```go
package util // internal/util/

func EnsureDirs(path string) error  // os.MkdirAll(path, 0755)
```

After config load, call `EnsureDirs` on all 5 directory paths. Fatal error if any fails. Empty paths are skipped.

---

## 7. gen-config Subcommand

```go
type GenConfigCommand struct{}

func (g *GenConfigCommand) Execute(args []string) error
```

Creates a `Config` with all defaults, encodes to TOML on stdout, exits.

---

## Tests

### Config Tests

| Test | Description |
|------|-------------|
| ToApplicationConfig parses api-endpoint | `"prod,https://api.example.com"` → Environment=`"prod"`, Endpoint=`"https://api.example.com"` |
| ToApplicationConfig empty endpoint uses defaults | Empty `api-endpoint` → uses build-time defaults |
| NeedsSetup true when fields empty | Missing pepper → returns true |
| NeedsSetup false when all set | All fields populated → returns false |
| Cleanup trims and lowercases | `" ARGON2 "` → `"argon2"` |
| With* returns new copy | Original unchanged after With call |
| MergeSettableValues correct tags | UsePlayersDB change → `ACEUsePlayersDB` tag (NOT `ACEServiceCredentials`) |
| Validation collects all errors | Multiple invalid fields → all reported |
| Config writer round-trip | Write → read → compare |
| gen-config outputs valid TOML | Capture stdout, parse as TOML, verify all fields |

### Directory Init Tests

| Test | Description |
|------|-------------|
| EnsureDirs creates nested dirs | `t.TempDir()/a/b/c` → created |
| EnsureDirs idempotent | Call twice, no error |
| EnsureDirs skips empty path | `""` → no error, nothing created |

## Acceptance Criteria

- [ ] TOML config loads with all fields correctly mapped
- [ ] CLI flags override TOML values
- [ ] Missing config file → setup wizard, not crash
- [ ] `NeedsSetup()` checks all 6 required fields
- [ ] `MergeSettableValues` tags `UsePlayersDB` with `ACEUsePlayersDB`
- [ ] All `With*` methods return copies without mutation
- [ ] `gen-config` prints valid TOML
- [ ] Directories created on startup
- [ ] All tests pass
