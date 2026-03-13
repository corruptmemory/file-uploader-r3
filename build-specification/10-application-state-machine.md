# 10 — Application State Machine

**Dependencies:** 02-chanutil-and-interfaces.md (chanutil, Stoppable), 03-configuration.md (ApplicationConfig).

**Produces:** `internal/app/` — Application actor, SetupApp/RunningApp/ErrorApp interfaces, RecoverableError, EventSink interfaces, file lifecycle types.

---

## 1. Application Actor

The top-level orchestrator. Owns exactly one active state at a time.

```go
type Application struct {
    // unexported: commands channel, current Stoppable state, sync.WaitGroup
}

func (a *Application) GetState() (any, error)
func (a *Application) SetState(stateBuilder func(app *Application) (Stoppable, error)) error
```

### Behavior

**Construction:** Created with an `initialStateBuilder` function. Internal `run()` goroutine calls this builder.

**`run()` goroutine:**
1. Call `initialStateBuilder(app)`.
2. If `RecoverableError` → extract `nextBuilder`, retry.
3. If retry also fails → install `ErrorApp`.
4. If success → install state, begin processing commands in `for/select` loop.

**`SetState` handling inside `run()`:**
1. `Stop()` on current state.
2. `Wait()` on current state.
3. Call `stateBuilder(app)`.
4. If `RecoverableError` → extract `nextBuilder`, retry.
5. If retry also fails → install `ErrorApp`.
6. Otherwise install new state.

### State Transitions

| From | To | Trigger |
|---|---|---|
| Setup | Running | Wizard completes, config written |
| Running | Setup | RecoverableError (e.g., credentials expired) |
| Any | Error | Unrecoverable error during state creation |

---

## 2. RecoverableError

```go
type RecoverableError struct {
    Err         error
    nextBuilder func(app *Application) (Stoppable, error)
}

func (e *RecoverableError) Error() string
func (e *RecoverableError) NextBuilder() func(app *Application) (Stoppable, error)
```

Satisfies `error` interface. Detected via `errors.As`. Only one level of recovery attempted.

---

## 3. State Interfaces

### SetupApp

```go
type SetupApp interface {
    Stoppable
    GoBackFrom(step SetupStepNumber) (SetupStepInfo, error)
    GetCurrentState() (SetupStepInfo, error)
    GetServiceEndpoint() (SetupStepInfo, error)
    SetServiceEndpoint(endpoint, env string) (SetupStepInfo, error)
    UseRegistrationCode(code string) (SetupStepInfo, error)
    SetPlayerIDHasher(pepper, hash string) (SetupStepInfo, error)
    SetUsePlayerDB(usePlayersDB bool) (SetupStepInfo, error)
}
```

**SetupStepNumber:** Integer enum (0=Welcome, 1=Endpoint, 2=ServiceCredentials, 3=PlayerIDHasher, 4=UsePlayersDB, 5=Done, 6=Error).

**SetupStepInfo:** Interface with `CurrentStep()`, `Next()`, `Prev()` methods.

SetupApp is an actor. Wizard state owned by single goroutine. Dispatch via map keyed by `(currentStep, targetStep)` pairs — no nested switch blocks.

**Completion flow:** Assemble config → validate → write to disk → `Application.SetState()` with RunningApp builder → return StepDone.

---

### RunningApp

```go
type RunningApp interface {
    Stoppable
    Subscribe() (*EventSubscription, error)
    Unsubscribe(id string) error
    ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error
    GetFinishedDetails(recordID string) (*CSVFinishedFile, error)
    GetState() (*RunningState, error)
    SearchFinished(status FinishedStatus, csvTypes []CSVType, search string) ([]CSVFinishedFile, error)
    GetConfig() (ApplicationConfig, error)
    MFARequired() (bool, error)
    UpdateConfig(config ApplicationConfig) error
    DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error
}
```

RunningApp is an actor. Single `run()` goroutine owns ALL mutable state:
- Queued files (FIFO), currently processing file, uploading files, finished files
- SSE subscriber map, player DB, CSV processor, uploader references

---

### ErrorApp

```go
type ErrorApp interface {
    Stoppable
    GetError() error
}
```

Minimal actor. Terminal state — no recovery.

---

## 4. File Lifecycle Types

```go
type FileMetadata struct {
    ID               string
    UploadedBy       string
    OriginalFilename string
    LocalFilePath    string
    UploadedAt       time.Time
}

type CSVProcessingFile struct {
    InFile               FileMetadata
    CSVType              CSVType
    StartedAt            time.Time
    FinishedAt           *time.Time
    ProcessingOutputPath string
    ProgressPercent      float64
}

type CSVUploadingFile struct {
    InFile               FileMetadata
    CSVType              CSVType
    ProcessingStartedAt  time.Time
    ProcessingFinishedAt time.Time
    UploadingStartedAt   time.Time
    UploadingFinishedAt  *time.Time
    ProgressPercent      float64
}

type CSVFinishedFile struct {
    InFile               FileMetadata
    CSVType              CSVType
    ProcessingStartedAt  time.Time
    ProcessingFinishedAt time.Time
    UploadingStartedAt   *time.Time
    UploadingFinishedAt  *time.Time
    Success              bool
    FailurePhase         FailurePhase
    FailureReason        string
}

type FailurePhase string
const (
    FailurePhaseProcessing FailurePhase = "processing"
    FailurePhaseUploading  FailurePhase = "uploading"
)

type FinishedStatus string
const (
    FinishedStatusAll     FinishedStatus = ""
    FinishedStatusSuccess FinishedStatus = "success"
    FinishedStatusFailure FinishedStatus = "failure"
)
```

---

## 5. SSE Subscription

```go
type EventSubscription struct {
    ID     string
    Events <-chan DataUpdateEvent
}

type DataUpdateEvent struct {
    State CSVProcessingState
}

type CSVProcessingState struct {
    QueuedFiles    []FileMetadata
    ProcessingFile *CSVProcessingFile
    UploadingFiles []CSVUploadingFile
    FinishedFiles  []CSVFinishedFile
}

type RunningState struct {
    Started        time.Time
    OrganizationID string
    AppConfig      ApplicationConfig
    DataProcessing CSVProcessingState
    PlayersDB      PlayersDBState
}
```

**Subscribe():** Generate unique ID, create buffered events channel, send current state as first event.

**Unsubscribe(id):** Remove from map, close channel.

**Broadcasting:** After every state change, iterate subscribers, non-blocking send. Track consecutive failures per subscriber. 10 consecutive failures → auto-remove.

**Throttle:** Progress broadcasts at most once per second.

---

## 6. Event Sink Interfaces

### CSV Processing EventSink

```go
type EventSink interface {
    Starting(file OutFileMetadata)
    Identified(file OutFileMetadata)
    Progress(file OutFileMetadata, record ProgressRecord)
    Success(file OutFileMetadata)
    Failure(file OutFileMetadata, record ProgressRecord, err error)
}
```

### Uploading EventSink

```go
type UploaderEventSink interface {
    Starting(file UploadingFileMetadata)
    Progress(file UploadingFileMetadata, record UploaderProgressRecord)
    Success(file UploadingFileMetadata)
    Failure(file UploadingFileMetadata, record UploaderProgressRecord, err error)
}
```

Both sinks: implementations hold reference to RunningApp's command channel. Progress throttled to 1/second in the sink.

---

## 7. Initial State Recovery

When RunningApp starts, scan processing directories:
1. Files in upload-dir → re-queue.
2. Files in processing-dir → archive as failed (processing interrupted).
3. Files in uploading-dir → archive as failed (upload interrupted).
4. Files in archive-dir → load into finished files list.

---

## CRITICAL: Real Implementation Wiring

In non-mock mode, `RunningApp` MUST use the real implementations:
- Real CSV worker pool (from spec 07) that actually processes files
- Real SSE broadcaster that sends real-time events to the dashboard
- Real player dedup database (from spec 08) that persists across uploads
- Real archive system that stores processed results
- Real file lifecycle management (temp → processing → archive → cleanup)

Do NOT use mock implementations for any of these in non-mock mode. The mock flag (`--mock`) replaces only EXTERNAL dependencies (remote API client, auth provider, uploader). The internal pipeline must always be real.

This is the #1 failure mode from previous rounds: wiring `NewMockRunningApp` for all code paths instead of only when `--mock` is set.

---

## Tests

| Test | Description |
|------|-------------|
| Application starts in SetupApp when config needs setup | `NeedsSetup()` true → SetupApp state |
| Application starts in RunningApp when config complete | `NeedsSetup()` false → RunningApp state |
| SetState stops old state first | Verify Stop() + Wait() called before new state |
| RecoverableError retries with nextBuilder | First builder returns RecoverableError → second builder called |
| Double recovery → ErrorApp | Both builders fail → ErrorApp installed |
| GetState returns current state | Type-assert to correct state interface |
| SSE subscriber receives updates | Subscribe, trigger state change, verify event received |
| Auto-remove after 10 failures | Fill subscriber's channel, verify removal after 10 drops |

## Acceptance Criteria

- [ ] Application actor manages state transitions atomically
- [ ] RecoverableError with one retry level
- [ ] SetupApp wizard navigates forward/backward preserving values
- [ ] RunningApp processes files FIFO, one at a time
- [ ] SSE subscribers auto-removed after 10 consecutive failures
- [ ] Progress broadcasts throttled to 1/second
- [ ] Initial state recovery scans processing directories
- [ ] All tests pass
