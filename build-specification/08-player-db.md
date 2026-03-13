# 08 — Player Deduplication Database

**Dependencies:** 02-chanutil-and-interfaces.md (actor pattern helpers), 04-hashing-and-normalization.md (PlayerDB interface contract).

**Produces:** `internal/playerdb/` package — file format, PlayerDB implementation, ConcurrentPlayerDB actor wrapper, DevNullDB, persistence, snapshot support.

---

## 1. Data Model

### Entry Struct

```go
type Entry struct {
    MetaID               string `csv:"meta_id"`
    OrganizationPlayerID string `csv:"organization_player_id"`
    Country              string `csv:"country"`
    State                string `csv:"state"`
}
```

### Composite Lookup Key

Format: `"orgPlayerID:country:state"` — all components **lowercased** before joining.

In-memory index: `map[string]int` mapping lookup key → index in entry slice.

### DBHeader Struct

```go
type DBHeader struct {
    OrganizationPlayerHash         string `toml:"organization-player-hash"`
    OrganizationPlayerIDPepperHash string `toml:"organization-player-id-pepper-hash"`
}
```

- `OrganizationPlayerHash`: always `"argon2"`.
- `OrganizationPlayerIDPepperHash`: **SHA-256 hash of the pepper** (NOT the raw pepper). Format: `"sha256:{hex-encoded-hash}"`.

---

## 2. File Format — TOML Header + CSV Body

```
organization-player-hash = "argon2"
organization-player-id-pepper-hash = "sha256:e3b0c44..."

--- END OF HEADER ---
meta_id,organization_player_id,country,state
base64hash1,player001,us,ma
base64hash2,player002,us,ny
```

### Reading

1. Read entire file as bytes.
2. Split on `"--- END OF HEADER ---\n"`.
3. Parse before delimiter as TOML → `DBHeader`.
4. Parse after delimiter as CSV (with headers) → `[]Entry` using `gocsv`.
5. Build in-memory index.
6. **Pepper validation:** SHA-256 of current pepper vs `OrganizationPlayerIDPepperHash`. Mismatch → error.
7. File doesn't exist → create new empty DB with correct header.

### Writing (Atomic)

1. Marshal `DBHeader` as TOML.
2. Write header bytes.
3. Write `"--- END OF HEADER ---\n"`.
4. Marshal entries as CSV using `gocsv`.
5. Write CSV bytes.
6. Use temp-file-and-rename pattern: write to temp file, `Sync()`, rename over target (atomic on POSIX).

---

## 3. PlayerDB Interface Implementation

```go
type PlayerDB interface {
    GetPlayerByOrgPlayerID(orgPlayerID, country, state string) (Entry, bool)
    AddEntry(metaID, orgPlayerID, country, state string)
    Merge(other PlayerDB)
    Header() DBHeader
}
```

### Method Behaviors

**GetPlayerByOrgPlayerID:** Construct composite key (all lowercased), look up in index. Return Entry+true if found, zero Entry+false otherwise.

**AddEntry:** Construct composite key. If key exists:
- Existing entry has empty MetaID AND new MetaID is non-empty → update (backfill case).
- Otherwise → skip (first-write-wins).

If key doesn't exist: append new Entry, update index.

**Merge:** Iterate all entries in `other`, call `AddEntry` for each. Used when merging per-file temp DB into main DB.

**Header:** Return the DBHeader.

---

## 4. ConcurrentPlayerDB — Actor Wrapper

```go
type ConcurrentPlayerDB struct {
    db       PlayerDB
    commands chan playerDBCommand
    wg       sync.WaitGroup
}
```

Command channel buffer size: **1000**.

### Command Tags

```go
type playerDBCommandTag int

const (
    playerDBLookup playerDBCommandTag = iota
    playerDBAdd
)
```

### Public Methods

```go
func NewConcurrentPlayerDB(db PlayerDB) *ConcurrentPlayerDB
func (c *ConcurrentPlayerDB) GetPlayerByOrgPlayerID(orgPlayerID, country, state string) (Entry, bool)
func (c *ConcurrentPlayerDB) AddEntry(metaID, orgPlayerID, country, state string)
func (c *ConcurrentPlayerDB) Merge(other PlayerDB)
func (c *ConcurrentPlayerDB) Header() DBHeader
func (c *ConcurrentPlayerDB) InnerDB() PlayerDB  // for persistence
func (c *ConcurrentPlayerDB) Close()              // close channel, wait for goroutine
```

---

## 5. DevNullDB — Disabled Implementation

```go
type DevNullDB struct{}

func (d *DevNullDB) GetPlayerByOrgPlayerID(orgPlayerID, country, state string) (Entry, bool) {
    return Entry{}, false
}
func (d *DevNullDB) AddEntry(metaID, orgPlayerID, country, state string) {}
func (d *DevNullDB) Merge(other PlayerDB) {}
func (d *DevNullDB) Header() DBHeader { return DBHeader{} }
```

Singleton — same instance shared everywhere.

---

## 6. Persistence

```go
func SaveDB(db PlayerDB, filePath string) error
```

Called after each CSV file finishes processing. Uses atomic write from Section 2.

---

## 7. Directory Naming

Database directory: `playersdb-{algorithm}-{base64url(pepper)}`

Example: `playersdb-argon2-c29tZXBlcHBlcg`

Uses `base64.RawURLEncoding` (no padding). Created under configured work directory. Database file: `players.db`.

---

## 8. Snapshot / Download

```go
func CreateSnapshot(dbPath string, snapshotDir string) (string, error)
```

1. Timestamp: `time.Now().Format("20060102_150405.000000000")`.
2. Filename: `players-{timestamp}.db`.
3. Byte-for-byte copy, `Sync()`.
4. Return snapshot path.

Snapshot files are temporary — delete after serving.

---

## 9. Initialization Flow

1. Check if player DB enabled in config.
2. Disabled → use `DevNullDB`, done.
3. Enabled:
   a. Compute directory path (Section 7).
   b. Ensure directory exists.
   c. If `players.db` exists: read, parse, validate pepper hash. Error on mismatch.
   d. If not exists: create empty DB with correct header.
   e. Wrap in `ConcurrentPlayerDB`.
4. Pass to CSV processing pipeline.

---

## Tests

| Test | Description |
|------|-------------|
| AddEntry + GetPlayer round-trip | Add entry, retrieve by key, verify match |
| First-write-wins dedup | Add same key twice, second ignored |
| MetaID backfill | Add with empty MetaID, then with non-empty → updated |
| Merge two DBs | Merge, verify all entries present |
| File round-trip | Save → reload → compare all entries |
| Pepper mismatch error | Create with pepper A, load with pepper B → error |
| Case-insensitive key | `("PLAYER1", "US", "MA")` matches `("player1", "us", "ma")` |
| DevNullDB returns false | All lookups return not-found |
| Atomic write | Write completes, verify file exists and is valid |
| Snapshot creates copy | Create snapshot, verify contents match original |

## Acceptance Criteria

- [ ] TOML+CSV hybrid format reads and writes correctly
- [ ] Pepper validation via SHA-256 comparison
- [ ] First-write-wins with MetaID backfill
- [ ] ConcurrentPlayerDB wraps with actor pattern
- [ ] DevNullDB silently discards all operations
- [ ] Atomic writes via temp-file-and-rename
- [ ] Directory naming from algorithm + base64url pepper
- [ ] Snapshot creates point-in-time copy
- [ ] All tests pass
