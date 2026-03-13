package playerdb

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/corruptmemory/file-uploader-r3/internal/chanutil"
	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
	"github.com/gocarina/gocsv"
)

// headerDelimiter separates the TOML header from the CSV body in the DB file.
const headerDelimiter = "--- END OF HEADER ---\n"

// Entry represents a single player record in the database.
type Entry struct {
	MetaID               string `csv:"meta_id"`
	OrganizationPlayerID string `csv:"organization_player_id"`
	Country              string `csv:"country"`
	State                string `csv:"state"`
}

// DBHeader is the TOML header portion of the DB file.
type DBHeader struct {
	OrganizationPlayerHash         string `toml:"organization-player-hash"`
	OrganizationPlayerIDPepperHash string `toml:"organization-player-id-pepper-hash"`
}

// PlayerDB defines the full interface for the player deduplication database.
// GetPlayerByOrgPlayerID returns (metaID, found) to satisfy hashers.PlayerDB.
type PlayerDB interface {
	GetPlayerByOrgPlayerID(orgPlayerID, country, state string) (metaID string, found bool)
	AddEntry(metaID, orgPlayerID, country, state string)
	Merge(other PlayerDB)
	Header() DBHeader
	Entries() []Entry
}

// compositeKey builds the lowercase lookup key.
func compositeKey(orgPlayerID, country, state string) string {
	return strings.ToLower(orgPlayerID + ":" + country + ":" + state)
}

// pepperHash computes "sha256:{hex}" of a pepper string.
func pepperHash(pepper string) string {
	h := sha256.Sum256([]byte(pepper))
	return fmt.Sprintf("sha256:%x", h[:])
}

// --- memDB: in-memory implementation ---

type memDB struct {
	header  DBHeader
	entries []Entry
	index   map[string]int // compositeKey → index in entries
}

// NewMemDB creates a new empty in-memory player database.
func NewMemDB(pepper string) *memDB {
	return &memDB{
		header: DBHeader{
			OrganizationPlayerHash:         "argon2",
			OrganizationPlayerIDPepperHash: pepperHash(pepper),
		},
		entries: nil,
		index:   make(map[string]int),
	}
}

// newMemDBWithHeader creates a memDB with a pre-built header (used by LoadDB).
func newMemDBWithHeader(header DBHeader) *memDB {
	return &memDB{
		header:  header,
		entries: nil,
		index:   make(map[string]int),
	}
}

func (db *memDB) GetPlayerByOrgPlayerID(playerID, country, state string) (string, bool) {
	key := compositeKey(playerID, country, state)
	idx, ok := db.index[key]
	if !ok {
		return "", false
	}
	return db.entries[idx].MetaID, true
}

// GetEntry returns the full Entry (used internally and by tests).
func (db *memDB) GetEntry(playerID, country, state string) (Entry, bool) {
	key := compositeKey(playerID, country, state)
	idx, ok := db.index[key]
	if !ok {
		return Entry{}, false
	}
	return db.entries[idx], true
}

func (db *memDB) AddEntry(metaID, playerID, country, state string) {
	key := compositeKey(playerID, country, state)
	if idx, exists := db.index[key]; exists {
		// Backfill: update if existing MetaID is empty and new is non-empty.
		if db.entries[idx].MetaID == "" && metaID != "" {
			db.entries[idx].MetaID = metaID
		}
		return
	}
	db.index[key] = len(db.entries)
	db.entries = append(db.entries, Entry{
		MetaID:               metaID,
		OrganizationPlayerID: playerID,
		Country:              country,
		State:                state,
	})
}

func (db *memDB) Merge(other PlayerDB) {
	for _, e := range other.Entries() {
		db.AddEntry(e.MetaID, e.OrganizationPlayerID, e.Country, e.State)
	}
}

func (db *memDB) Header() DBHeader {
	return db.header
}

func (db *memDB) Entries() []Entry {
	return db.entries
}

// Save persists the database to disk using atomic write.
func (db *memDB) Save(path string) error {
	return SaveDB(db, path)
}

// --- SaveDB: atomic write ---

// SaveDB writes the database to filePath atomically (temp file + sync + rename).
func SaveDB(db PlayerDB, filePath string) error {
	dir := filepath.Dir(filePath)

	// Marshal TOML header.
	var headerBuf bytes.Buffer
	enc := toml.NewEncoder(&headerBuf)
	if err := enc.Encode(db.Header()); err != nil {
		return fmt.Errorf("playerdb: encoding header: %w", err)
	}

	// Marshal CSV body.
	entries := db.Entries()
	var csvBuf bytes.Buffer
	if len(entries) == 0 {
		// gocsv won't write headers for empty slice; write header manually.
		csvBuf.WriteString("meta_id,organization_player_id,country,state\n")
	} else {
		if err := gocsv.Marshal(entries, &csvBuf); err != nil {
			return fmt.Errorf("playerdb: encoding csv: %w", err)
		}
	}

	// Assemble full content.
	var buf bytes.Buffer
	buf.Write(headerBuf.Bytes())
	buf.WriteString(headerDelimiter)
	buf.Write(csvBuf.Bytes())

	// Atomic write: temp file → sync → rename.
	tmp, err := os.CreateTemp(dir, ".playerdb-*.tmp")
	if err != nil {
		return fmt.Errorf("playerdb: creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("playerdb: writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("playerdb: syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("playerdb: closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("playerdb: renaming temp file: %w", err)
	}
	return nil
}

// --- LoadDB ---

// LoadDB reads a player database from filePath. If the file does not exist,
// it creates a new empty DB. The pepper is validated against the header hash.
func LoadDB(filePath string, pepper string) (*memDB, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			db := NewMemDB(pepper)
			return db, nil
		}
		return nil, fmt.Errorf("playerdb: reading file: %w", err)
	}

	parts := bytes.SplitN(data, []byte(headerDelimiter), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("playerdb: missing header delimiter")
	}

	var header DBHeader
	if err := toml.Unmarshal(parts[0], &header); err != nil {
		return nil, fmt.Errorf("playerdb: parsing header: %w", err)
	}

	// Validate pepper.
	expected := pepperHash(pepper)
	if header.OrganizationPlayerIDPepperHash != expected {
		return nil, fmt.Errorf("playerdb: pepper mismatch: expected %s, got %s",
			expected, header.OrganizationPlayerIDPepperHash)
	}

	db := newMemDBWithHeader(header)

	var entries []Entry
	csvReader := bytes.NewReader(parts[1])
	if err := gocsv.Unmarshal(csvReader, &entries); err != nil {
		return nil, fmt.Errorf("playerdb: parsing csv: %w", err)
	}

	for _, e := range entries {
		db.AddEntry(e.MetaID, e.OrganizationPlayerID, e.Country, e.State)
	}

	return db, nil
}

// --- Directory Naming ---

// DBDirName returns the directory name for a player database.
func DBDirName(algorithm, pepper string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(pepper))
	return fmt.Sprintf("playersdb-%s-%s", algorithm, encoded)
}

// --- Snapshot ---

// CreateSnapshot creates a point-in-time copy of the database file.
// Returns the path to the snapshot file.
func CreateSnapshot(dbPath string, snapshotDir string) (string, error) {
	if err := os.MkdirAll(snapshotDir, 0o750); err != nil {
		return "", fmt.Errorf("playerdb: creating snapshot dir: %w", err)
	}

	ts := time.Now().Format("20060102_150405.000000000")
	snapshotName := fmt.Sprintf("players-%s.db", ts)
	snapshotPath := filepath.Join(snapshotDir, snapshotName)

	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("playerdb: opening source: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(snapshotPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("playerdb: creating snapshot: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(snapshotPath)
		return "", fmt.Errorf("playerdb: copying to snapshot: %w", err)
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		os.Remove(snapshotPath)
		return "", fmt.Errorf("playerdb: syncing snapshot: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(snapshotPath)
		return "", fmt.Errorf("playerdb: closing snapshot: %w", err)
	}

	return snapshotPath, nil
}

// --- ConcurrentPlayerDB: actor wrapper ---

type playerDBCommandTag int

const (
	playerDBLookup playerDBCommandTag = iota
	playerDBAdd
	playerDBSave
	playerDBEntries
	playerDBMerge
)

type playerDBCommand struct {
	tag      playerDBCommandTag
	metaID   string
	playerID string
	country  string
	state    string
	savePath string
	mergeDB  PlayerDB
	result   chan any
}

func (c playerDBCommand) WithResult(ch chan any) playerDBCommand {
	c.result = ch
	return c
}

// lookupResult is used to send lookup results through the result channel.
type lookupResult struct {
	metaID string
	found  bool
}

// ConcurrentPlayerDB wraps a PlayerDB with a channel-based actor for safe
// concurrent access.
type ConcurrentPlayerDB struct {
	db       PlayerDB
	commands chan playerDBCommand
	wg       sync.WaitGroup
}

// NewConcurrentPlayerDB creates a new actor-wrapped player database.
func NewConcurrentPlayerDB(db PlayerDB) *ConcurrentPlayerDB {
	c := &ConcurrentPlayerDB{
		db:       db,
		commands: make(chan playerDBCommand, 1000),
	}
	c.wg.Add(1)
	go c.run()
	return c
}

func (c *ConcurrentPlayerDB) run() {
	defer c.wg.Done()
	for cmd := range c.commands {
		switch cmd.tag {
		case playerDBLookup:
			metaID, found := c.db.GetPlayerByOrgPlayerID(cmd.playerID, cmd.country, cmd.state)
			entry := lookupResult{metaID: metaID, found: found}
			if cmd.result != nil {
				cmd.result <- entry
			}
		case playerDBAdd:
			c.db.AddEntry(cmd.metaID, cmd.playerID, cmd.country, cmd.state)
			// Fire-and-forget: no response needed.
		case playerDBSave:
			err := SaveDB(c.db, cmd.savePath)
			if cmd.result != nil {
				cmd.result <- err
			}
		case playerDBEntries:
			// Return a copy of the entries slice to avoid aliasing.
			src := c.db.Entries()
			cp := make([]Entry, len(src))
			copy(cp, src)
			if cmd.result != nil {
				cmd.result <- cp
			}
		case playerDBMerge:
			c.db.Merge(cmd.mergeDB)
			if cmd.result != nil {
				cmd.result <- nil
			}
		}
	}
}

// GetPlayerByOrgPlayerID satisfies hashers.PlayerDB interface.
func (c *ConcurrentPlayerDB) GetPlayerByOrgPlayerID(playerID, country, state string) (string, bool) {
	result, err := chanutil.SendReceiveMessage[playerDBCommand, lookupResult](
		c.commands,
		playerDBCommand{
			tag:      playerDBLookup,
			playerID: playerID,
			country:  country,
			state:    state,
		},
	)
	if err != nil {
		return "", false
	}
	return result.metaID, result.found
}

// AddEntry satisfies hashers.PlayerDB interface. Fire-and-forget.
func (c *ConcurrentPlayerDB) AddEntry(metaID, playerID, country, state string) {
	// Fire-and-forget: send without waiting for response.
	cmd := playerDBCommand{
		tag:      playerDBAdd,
		metaID:   metaID,
		playerID: playerID,
		country:  country,
		state:    state,
	}
	// Use trySend-style: if channel is closed, silently drop.
	defer func() { recover() }()
	c.commands <- cmd
}

// Merge delegates to the inner DB through the actor channel.
func (c *ConcurrentPlayerDB) Merge(other PlayerDB) {
	_ = chanutil.SendReceiveError[playerDBCommand](
		c.commands,
		playerDBCommand{
			tag:     playerDBMerge,
			mergeDB: other,
		},
	)
}

// Header returns the header from the inner DB. Header is immutable after
// construction, so it is safe to read directly.
func (c *ConcurrentPlayerDB) Header() DBHeader {
	return c.db.Header()
}

// Entries returns a copy of the entries through the actor channel.
func (c *ConcurrentPlayerDB) Entries() []Entry {
	result, err := chanutil.SendReceiveMessage[playerDBCommand, []Entry](
		c.commands,
		playerDBCommand{
			tag: playerDBEntries,
		},
	)
	if err != nil {
		return nil
	}
	return result
}

// Save persists the inner database to disk through the actor channel,
// ensuring all queued AddEntry commands are processed first.
func (c *ConcurrentPlayerDB) Save(path string) error {
	return chanutil.SendReceiveError[playerDBCommand](
		c.commands,
		playerDBCommand{
			tag:      playerDBSave,
			savePath: path,
		},
	)
}

// Close shuts down the actor goroutine and waits for it to finish.
func (c *ConcurrentPlayerDB) Close() {
	close(c.commands)
	c.wg.Wait()
}

// --- DevNullDB: no-op implementation ---

// DevNullDB is a no-op PlayerDB that silently discards all operations.
type DevNullDB struct{}

var devNullSingleton = &DevNullDB{}

// GetDevNullDB returns the singleton DevNullDB instance.
func GetDevNullDB() *DevNullDB {
	return devNullSingleton
}

func (d *DevNullDB) GetPlayerByOrgPlayerID(playerID, country, state string) (string, bool) {
	return "", false
}

func (d *DevNullDB) AddEntry(metaID, playerID, country, state string) {}

func (d *DevNullDB) Merge(other PlayerDB) {}

func (d *DevNullDB) Header() DBHeader { return DBHeader{} }

func (d *DevNullDB) Entries() []Entry { return nil }

func (d *DevNullDB) Save(path string) error { return nil }

// Compile-time checks that our types satisfy both PlayerDB and hashers.PlayerDB.
var (
	_ PlayerDB         = (*memDB)(nil)
	_ PlayerDB         = (*DevNullDB)(nil)
	_ hashers.PlayerDB = (*memDB)(nil)
	_ hashers.PlayerDB = (*ConcurrentPlayerDB)(nil)
	_ hashers.PlayerDB = (*DevNullDB)(nil)
)
