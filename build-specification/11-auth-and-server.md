# 11 — Authentication, Server, and HTTP Wiring

**Dependencies:** 10-application-state-machine.md (Application, state interfaces), 03-configuration.md (ApplicationConfig).

**Produces:** JWT auth system, Server struct, WebApp struct, route registration, chi router setup, mock implementations.

---

## 1. AuthProvider Interface

```go
type AuthProvider interface {
    Login(username, password, mfaToken string) (SessionToken, error)
    ServiceAccountLogin(serviceCode string) (SessionToken, error)
    ConsumeRegistrationCode(endpoint, code string) (APIClient, error)
    MFARequired() bool
}

type SessionToken struct {
    Username string
    OrgID    string
}
```

---

## 2. APIClient Interface

```go
type APIClient interface {
    TestEndpoint() error
    GetConfig(logFunc func(string, ...any)) (RemoteConfig, error)
    UploadFile(csvType CSVType, fileSize int64, reader io.ReadCloser, logFunc func(string, ...any)) error
}
```

---

## 3. Uploader Interface

```go
type Uploader interface {
    UploadFile(inFile FileMetadata, filePath string, csvType CSVType, sink EventSink) error
    Stop()
    Wait()
}
```

---

## 4. JWT Authentication

### Configuration
- Algorithm: HS256 (HMAC-SHA256)
- Signing key: from config (`signing-key` field)
- Expiry: **5 minutes** from creation

### Claims

```go
type JWTClaims struct {
    Username string `json:"username"`
    OrgID    string `json:"orgID"`
    Exp      int64  `json:"exp"` // Unix timestamp
}
```

### Cookie Management

On login, set two cookies:

1. **`session`** — signed JWT string. `HttpOnly: true`, `Path: /`, `SameSite: Strict`.
2. **`session-expires`** — RFC 1123 expiry timestamp. `HttpOnly: false` (JS-readable), `Path: /`, `SameSite: Strict`.

### Session Extension (`GET /api/extend`)

- Validate current session.
- Check if JWT expires within **40 seconds**.
- If outside window: return 200, no action.
- If within window: issue new JWT with fresh 5-minute expiry, set updated cookies.

---

## 5. Session Middleware

**`withStateAndSession(handler)`:**
- Read `session` cookie, parse JWT, validate signature + expiry.
- Invalid/expired → clear cookies, redirect to `/login`.
- Valid → pass decoded `JWTClaims` to handler.

**`withStateOptionalSession(handler)`:**
- Same parsing. Invalid → pass nil claims (no redirect).

---

## 6. WebApp Struct

```go
type WebApp struct {
    app        *Application
    signingKey []byte
    uploadDir  string
    version    string
}
```

Registers all routes on a `chi.Router` during construction. Handlers access app via `app.GetState()` + type assertion.

---

## 7. Server Struct

```go
type Server struct {
    // unexported: *http.Server, address, tls config, sync.WaitGroup
}

func NewServer(addr string, handler http.Handler, tlsConfig *tls.Config) *Server
func (s *Server) Start() error   // begins listening, non-blocking
func (s *Server) Stop()          // http.Server.Shutdown with timeout
func (s *Server) Wait()          // blocks until stopped
```

---

## 8. Route Table

| Method | Path | Auth | State | Description |
|--------|------|------|-------|-------------|
| GET | `/` | Required | RunningApp | Dashboard |
| GET,POST | `/login` | None | RunningApp | Login form / submit |
| GET | `/logout` | None | RunningApp | Clear session, redirect |
| POST | `/upload` | Required | RunningApp | CSV upload (multipart) |
| GET | `/events` | Required | RunningApp | SSE stream |
| GET | `/api/extend` | Required | RunningApp | Extend JWT |
| GET | `/failure-details/{record-id}` | Required | RunningApp | Failure modal content |
| GET | `/settings` | Required | RunningApp | Settings page |
| POST | `/settings` | Required | RunningApp | Update settings |
| GET | `/players-db` | Required | RunningApp | Players DB page |
| GET | `/download-players-db` | Required | RunningApp | Download DB file |
| GET | `/archived` | Required | RunningApp | Archive page |
| POST | `/search-archived` | Required | RunningApp | Search archived files |
| GET | `/health` | None | Any | Returns 200 "ok" |
| GET | `/setup` | None | SetupApp | Wizard current step |
| POST | `/setup/{action}` | None | SetupApp | Wizard navigation |
| GET | `/js/*`, `/css/*`, `/img/*`, `/favicon.ico` | None | Any | Static assets |

All routes mounted under configurable prefix.

---

## 9. State-Aware Routing Helpers

**`withRunningState(handler)`:** GetState → RunningApp passes through, SetupApp → redirect `/setup`, ErrorApp → render error page.

**`withSetupApp(handler)`:** GetState → SetupApp passes through, RunningApp → redirect `/`, ErrorApp → render error page.

---

## 10. Mock Implementations

### MockRemoteAPIClient

- `TestEndpoint()`: always nil.
- `GetConfig(logFunc)`: returns static `RemoteConfig`.
- `UploadFile(...)`: reads body, writes to mock output dir.
- `Login(...)`: accepts any creds, returns fixed SessionToken.

### MockAuthProvider

- `Login(...)`: accepts anything, returns valid token (24h expiry).
- `MFARequired()`: always false.
- `ConsumeRegistrationCode(...)`: returns MockRemoteAPIClient.

### MockUploader

- `UploadFile(...)`: copies file to output dir, fires simulated progress events (25%, 50%, 75%, 100%).
- Options: `WithFailure(afterPercent)`, `WithDelay(duration)`.

### CRITICAL: Mock Scope

The `--mock` flag replaces ONLY external dependencies:
- `MockRemoteAPIClient` replaces the real API client
- `MockAuthProvider` replaces the real auth provider
- `MockUploader` replaces the real uploader

Everything else — the CSV worker pool, SSE broadcaster, player DB, archive, file lifecycle — uses REAL implementations regardless of `--mock`. In mock mode, the app starts directly in RunningApp with real internal pipeline but mocked external services.

Do NOT create a `MockRunningApp` or `NewMockRunningApp` that stubs out the internal pipeline. There is no such thing as a mock running app.

### Activating Mocks

`--mock` CLI flag:
- Mocks replace all real implementations.
- Setup wizard skipped.
- App starts directly in RunningApp.

---

## 11. Startup Sequence

```
main()
  → Parse CLI args
  → If -v: PrintVersion(), exit
  → If gen-config/gen-csv subcommand: execute, exit
  → Load TOML config
  → Merge CLI overrides
  → Validate config
  → Create temp working directory
  → Determine initial state (NeedsSetup → SetupApp, else RunningApp)
  → Create Application with initialStateBuilder
  → Create chi router + Logger middleware
  → Create WebApp, register routes
  → Create Server
  → server.Start()
  → server.Wait() (blocks until SIGINT/SIGTERM)
```

---

## Tests

| Test | Description |
|------|-------------|
| JWT creation and parsing | Create token, parse back, verify claims |
| Expired JWT rejected | Token with past exp → error |
| Session extension within window | Token expiring in 30s → new token issued |
| Session extension outside window | Token expiring in 60s → no action |
| Mock auth accepts any creds | Login with random creds → success |
| Health endpoint works in all states | Returns 200 regardless |
| Upload enforces 50MB limit | Request exceeding limit → 413 |
| Static assets served correctly | GET /js/app.js → 200 with correct MIME |

## Acceptance Criteria

- [ ] JWT HS256 with 5-minute expiry
- [ ] HttpOnly session cookie, JS-readable session-expires cookie
- [ ] Extension only within 40-second window
- [ ] All routes registered with correct methods and middleware
- [ ] State-aware routing redirects correctly
- [ ] Mock implementations functional with --mock flag
- [ ] Server supports both HTTP and HTTPS
- [ ] Startup sequence handles missing config gracefully
- [ ] All tests pass
