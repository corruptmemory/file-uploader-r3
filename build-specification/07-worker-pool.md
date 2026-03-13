# 07 — Worker Pool and File Processing

**Dependencies:** 05-csv-framework.md (CSVMetadata, RowData, CSVOutputRow), 04-hashing-and-normalization.md (Hashers interface).

**Produces:** `internal/csv/` additions — Processor struct, worker pool, file processing flow, CSV reading utility, error types.

---

## 1. Processor Struct

```go
type Processor struct {
    queue            chan workUnit
    workerCount      int
    workingDirectory string
    wg               sync.WaitGroup
    log              Logger
}

type workUnit struct {
    file       FileMetadata
    hashers    Hashers
    eventSink  EventSink
    operatorID CSVOutputString
}

func NewProcessor(ctx context.Context, log Logger, queueSize, workerCount int, workingDirectory string) *Processor
func (p *Processor) AddWork(file FileMetadata, hashers Hashers, eventSink EventSink, operatorID CSVOutputString)
func (p *Processor) Stop()
func (p *Processor) Wait()
```

- `NewProcessor` creates the processor, starts a single goroutine that reads from `queue` and processes files serially.
- `AddWork` sends a work unit to the queue. Blocks if queue is full.
- `Stop` closes the queue channel.
- `Wait` blocks until all queued files have been processed.

The `FileMetadata` struct is defined in spec 10. For this spec, use:

```go
type FileMetadata struct {
    ID               string
    UploadedBy       string
    OriginalFilename string
    LocalFilePath    string
    UploadedAt       time.Time
}
```

The `EventSink` interface is defined in spec 10. For this spec, use:

```go
type EventSink interface {
    Starting(file OutFileMetadata)
    Identified(file OutFileMetadata)
    Progress(file OutFileMetadata, record ProgressRecord)
    Success(file OutFileMetadata)
    Failure(file OutFileMetadata, record ProgressRecord, err error)
}

type OutFileMetadata struct {
    InFile   FileMetadata
    CSVType  CSVType
    OutPath  string
}

type ProgressRecord struct {
    RowsProcessed int
    TotalRows     int
    Percent       float64
}
```

---

## 2. Processing a Single File

1. **Emit Starting event** via `eventSink`.
2. Open the file. Wrap reader with BOM-stripping (`github.com/dimchansky/utfbom`).
3. Read first line, parse as CSV to get headers.
4. Run auto-detection against all 10 registered `CSVMetadata` handlers:
   - Zero matches → **emit Failure**, return.
   - Multiple matches → **emit Failure**, return.
5. Count total lines (second pass or pre-counting).
6. Create output file in `workingDirectory`.
7. Write quoted output headers as first line.
8. **Emit Identified event** (with CSVType and total row count).
9. Create two channels:
   - `inputCh` (buffered, capacity = `workerCount * 3`) for `RowData`.
   - `outputCh` (buffered, capacity = `workerCount * 3`) for results.
10. Launch `workerCount` worker goroutines. Each reads `RowData` from `inputCh`, calls `handler.ProcessRow(rowData)`, sends result to `outputCh`.
11. Launch a feeder goroutine that re-reads the file (after headers), builds `RowData` structs, sends to `inputCh`, closes `inputCh` when done.
12. A collector goroutine waits for all workers to finish, then closes `outputCh`.
13. Main processing loop reads from `outputCh`:
    - **On success:** write `row.RowString()` + newline to output. **Emit Progress** (row count / total).
    - **On error:** cancel context (cancels workers + feeder). **Emit Failure**. Drain output channel. Return.
14. When output fully drained with no errors: **emit Success** with output file path.
15. **After each file:** call `hashers.SaveDB()` to persist PlayerDB.

---

## 3. CSV Type Auto-Detection

```go
func DetectCSVType(headers []string, allHandlers []CSVMetadata) (CSVMetadata, error)
```

1. Iterate all handlers, call `MatchHeaders(headers)`.
2. Count matches.
3. Exactly one → return it.
4. Zero → error: `"no CSV type matched the headers in this file"`.
5. Multiple → error: `"multiple CSV types matched: [list]"`.

---

## 4. CSV Reading Utility

```go
func CSVToChanMaps(ctx context.Context, reader io.Reader, headers []string, out chan<- RowData) error
```

1. Create `encoding/csv` Reader from `reader` (already BOM-stripped, positioned after header).
2. Row counter starts at 1.
3. For each row: build `map[string]string` from headers + cell values, construct `RowData`, send on `out`.
4. If context canceled, stop and return context error.
5. Close `out` when done.

---

## 5. Error Types

```go
type RowErrors struct {
    RowIndex int
    Errors   []error
}

func (e RowErrors) Error() string  // "Row[N]: [err1, err2, ...]"
```

First row error aborts the entire file. All workers canceled via context.

---

## 6. Output Format

### Header Row
- Output headers from matched handler, each quoted via `encoding/csv`.
- Joined by commas.

### Data Rows
- `Quoted` values: CSV-escaped (handles commas, quotes, newlines).
- `Raw` values: as-is (hashes, integers, floats).
- `EmptyString`: empty field.
- Joined by commas via `CSVOutputRow.RowString()`.

### Row Ordering
Output row order does NOT need to match input. Consequence of parallel processing.

---

## Tests

### Auto-Detection Tests

| Test | Description |
|------|-------------|
| Valid Players headers → CSVPlayers | Exact required columns match |
| Extra columns still match | Extra columns ignored |
| Missing column → no match | One required column removed |
| No match → clear error | Completely wrong headers |

### Worker Pool Tests

| Test | Description |
|------|-------------|
| Single file processes completely | Generate CSV, process, verify output has correct headers and row count |
| Row error aborts file | Inject bad row, verify Failure event and early termination |
| Output has hashed identifiers | Process Players CSV, verify no raw player IDs in output |
| SaveDB called after file | Mock PlayerDB, verify SaveDB called |
| Multiple files queued | Add 3 files, verify serial processing (one at a time) |

### CSVToChanMaps Tests

| Test | Description |
|------|-------------|
| Correct row count | 10 rows → 10 RowData on channel |
| Header mapping | Column values mapped to correct header keys |
| Context cancellation | Cancel context mid-stream, verify early return |

## Acceptance Criteria

- [ ] Files processed serially from queue
- [ ] Rows processed in parallel by configurable worker pool
- [ ] First row error cancels all workers and aborts file
- [ ] Auto-detection returns exactly one handler or a clear error
- [ ] Output has quoted headers as first line
- [ ] UTF-8 BOM stripped transparently
- [ ] `SaveDB()` called after each file (success or failure)
- [ ] All tests pass
