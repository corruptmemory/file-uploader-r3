package playerdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
)

const testPepper = "test-pepper-value"

// Compile-time interface checks.
var _ hashers.PlayerDB = (*memDB)(nil)
var _ hashers.PlayerDB = (*ConcurrentPlayerDB)(nil)
var _ hashers.PlayerDB = (*DevNullDB)(nil)

func TestAddEntryAndGetPlayer(t *testing.T) {
	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")

	metaID, found := db.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if !found {
		t.Fatal("expected to find player")
	}
	if metaID != "meta1" {
		t.Fatalf("expected meta1, got %s", metaID)
	}
}

func TestFirstWriteWins(t *testing.T) {
	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")
	db.AddEntry("meta2", "player001", "us", "ma")

	metaID, found := db.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if !found {
		t.Fatal("expected to find player")
	}
	if metaID != "meta1" {
		t.Fatalf("expected meta1 (first write wins), got %s", metaID)
	}
}

func TestMetaIDBackfill(t *testing.T) {
	db := NewMemDB(testPepper)
	db.AddEntry("", "player001", "us", "ma")

	metaID, _ := db.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if metaID != "" {
		t.Fatalf("expected empty metaID, got %s", metaID)
	}

	// Backfill: existing empty MetaID gets updated.
	db.AddEntry("meta-backfilled", "player001", "us", "ma")
	metaID, found := db.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if !found {
		t.Fatal("expected to find player after backfill")
	}
	if metaID != "meta-backfilled" {
		t.Fatalf("expected meta-backfilled, got %s", metaID)
	}
}

func TestMergeTwoDBs(t *testing.T) {
	db1 := NewMemDB(testPepper)
	db1.AddEntry("meta1", "player001", "us", "ma")
	db1.AddEntry("meta2", "player002", "us", "ny")

	db2 := NewMemDB(testPepper)
	db2.AddEntry("meta3", "player003", "ca", "on")
	db2.AddEntry("meta4", "player001", "us", "ma") // duplicate key, should be ignored

	db1.Merge(db2)

	// All three unique players present.
	if _, found := db1.GetPlayerByOrgPlayerID("player001", "us", "ma"); !found {
		t.Fatal("missing player001")
	}
	if _, found := db1.GetPlayerByOrgPlayerID("player002", "us", "ny"); !found {
		t.Fatal("missing player002")
	}
	if _, found := db1.GetPlayerByOrgPlayerID("player003", "ca", "on"); !found {
		t.Fatal("missing player003")
	}

	// First-write-wins on merge.
	metaID, _ := db1.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if metaID != "meta1" {
		t.Fatalf("expected meta1 (first write wins on merge), got %s", metaID)
	}

	entries := db1.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after merge, got %d", len(entries))
	}
}

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")

	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")
	db.AddEntry("meta2", "player002", "us", "ny")
	db.AddEntry("meta3", "player003", "ca", "on")

	if err := SaveDB(db, dbPath); err != nil {
		t.Fatalf("SaveDB: %v", err)
	}

	loaded, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB: %v", err)
	}

	// Check header.
	if loaded.Header().OrganizationPlayerHash != "argon2" {
		t.Fatal("header mismatch: organization-player-hash")
	}

	// Check all entries present.
	for _, e := range db.Entries() {
		metaID, found := loaded.GetPlayerByOrgPlayerID(e.OrganizationPlayerID, e.Country, e.State)
		if !found {
			t.Fatalf("missing entry for %s after reload", e.OrganizationPlayerID)
		}
		if metaID != e.MetaID {
			t.Fatalf("metaID mismatch for %s: expected %s, got %s",
				e.OrganizationPlayerID, e.MetaID, metaID)
		}
	}

	if len(loaded.Entries()) != len(db.Entries()) {
		t.Fatalf("entry count mismatch: expected %d, got %d",
			len(db.Entries()), len(loaded.Entries()))
	}
}

func TestPepperMismatch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")

	db := NewMemDB("pepper-a")
	if err := SaveDB(db, dbPath); err != nil {
		t.Fatalf("SaveDB: %v", err)
	}

	_, err := LoadDB(dbPath, "pepper-b")
	if err == nil {
		t.Fatal("expected error for pepper mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "pepper mismatch") {
		t.Fatalf("expected pepper mismatch error, got: %v", err)
	}
}

func TestCaseInsensitiveKey(t *testing.T) {
	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "PLAYER1", "US", "MA")

	metaID, found := db.GetPlayerByOrgPlayerID("player1", "us", "ma")
	if !found {
		t.Fatal("expected case-insensitive lookup to find player")
	}
	if metaID != "meta1" {
		t.Fatalf("expected meta1, got %s", metaID)
	}
}

func TestDevNullDB(t *testing.T) {
	db := GetDevNullDB()

	db.AddEntry("meta1", "player001", "us", "ma")
	metaID, found := db.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if found {
		t.Fatal("DevNullDB should never find a player")
	}
	if metaID != "" {
		t.Fatalf("DevNullDB should return empty metaID, got %s", metaID)
	}

	// Merge should be a no-op.
	other := NewMemDB(testPepper)
	other.AddEntry("meta2", "player002", "us", "ny")
	db.Merge(other)

	_, found = db.GetPlayerByOrgPlayerID("player002", "us", "ny")
	if found {
		t.Fatal("DevNullDB should not retain merged entries")
	}

	// Save should be a no-op.
	if err := db.Save("/nonexistent/path"); err != nil {
		t.Fatalf("DevNullDB.Save should return nil, got: %v", err)
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")

	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")

	if err := SaveDB(db, dbPath); err != nil {
		t.Fatalf("SaveDB: %v", err)
	}

	// Verify the file exists and is valid.
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("file does not exist after atomic write: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("file is empty after atomic write")
	}

	// Verify it can be loaded.
	loaded, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB after atomic write: %v", err)
	}
	if len(loaded.Entries()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries()))
	}

	// Verify no temp files remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".playerdb-") {
			t.Fatalf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestSnapshotCreatesCopy(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")
	snapshotDir := filepath.Join(dir, "snapshots")

	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")
	db.AddEntry("meta2", "player002", "us", "ny")

	if err := SaveDB(db, dbPath); err != nil {
		t.Fatalf("SaveDB: %v", err)
	}

	snapshotPath, err := CreateSnapshot(dbPath, snapshotDir)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	// Verify snapshot exists.
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("snapshot file does not exist: %v", err)
	}

	// Verify contents match.
	original, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("reading original: %v", err)
	}
	snapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if string(original) != string(snapshot) {
		t.Fatal("snapshot contents do not match original")
	}

	// Verify filename format.
	name := filepath.Base(snapshotPath)
	if !strings.HasPrefix(name, "players-") || !strings.HasSuffix(name, ".db") {
		t.Fatalf("unexpected snapshot filename: %s", name)
	}
}

func TestConcurrentPlayerDB(t *testing.T) {
	inner := NewMemDB(testPepper)
	cdb := NewConcurrentPlayerDB(inner)
	defer cdb.Close()

	cdb.AddEntry("meta1", "player001", "us", "ma")

	// Give the fire-and-forget add time to process by doing a synchronous lookup.
	// The lookup goes through the actor, so the add must have been processed first.
	metaID, found := cdb.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if !found {
		t.Fatal("expected to find player via ConcurrentPlayerDB")
	}
	if metaID != "meta1" {
		t.Fatalf("expected meta1, got %s", metaID)
	}

	// Not found case.
	_, found = cdb.GetPlayerByOrgPlayerID("nonexistent", "us", "ma")
	if found {
		t.Fatal("expected not found for nonexistent player")
	}
}

func TestConcurrentPlayerDBSaveDrainsQueue(t *testing.T) {
	inner := NewMemDB(testPepper)
	cdb := NewConcurrentPlayerDB(inner)
	defer cdb.Close()

	// Fire off many AddEntry commands (fire-and-forget).
	for i := 0; i < 100; i++ {
		cdb.AddEntry(fmt.Sprintf("meta%d", i), fmt.Sprintf("player%03d", i), "us", "ma")
	}

	// Save goes through the actor channel, so it must drain all preceding adds.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")
	if err := cdb.Save(dbPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify all entries were persisted.
	loaded, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB: %v", err)
	}
	if len(loaded.Entries()) != 100 {
		t.Fatalf("expected 100 entries, got %d", len(loaded.Entries()))
	}

	// Entries() also goes through the actor.
	entries := cdb.Entries()
	if len(entries) != 100 {
		t.Fatalf("expected 100 entries from Entries(), got %d", len(entries))
	}
}

func TestConcurrentPlayerDBMergeThroughActor(t *testing.T) {
	inner := NewMemDB(testPepper)
	cdb := NewConcurrentPlayerDB(inner)
	defer cdb.Close()

	cdb.AddEntry("meta1", "player001", "us", "ma")

	other := NewMemDB(testPepper)
	other.AddEntry("meta2", "player002", "us", "ny")

	// Merge goes through the actor.
	cdb.Merge(other)

	// Verify both entries are present.
	_, found := cdb.GetPlayerByOrgPlayerID("player001", "us", "ma")
	if !found {
		t.Fatal("expected player001 after merge")
	}
	_, found = cdb.GetPlayerByOrgPlayerID("player002", "us", "ny")
	if !found {
		t.Fatal("expected player002 after merge")
	}
}

func TestSaveMethodOnMemDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")

	db := NewMemDB(testPepper)
	db.AddEntry("meta1", "player001", "us", "ma")

	if err := db.Save(dbPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB: %v", err)
	}
	if len(loaded.Entries()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries()))
	}
}

func TestDBDirName(t *testing.T) {
	name := DBDirName("argon2", "somepepper")
	if !strings.HasPrefix(name, "playersdb-argon2-") {
		t.Fatalf("unexpected dir name: %s", name)
	}
	// Verify the suffix is a hex hash (not the raw pepper).
	suffix := strings.TrimPrefix(name, "playersdb-argon2-")
	if len(suffix) != 16 {
		t.Fatalf("expected 16-char hex hash suffix, got %q (len %d)", suffix, len(suffix))
	}
	// Same pepper must produce the same directory name.
	if name2 := DBDirName("argon2", "somepepper"); name2 != name {
		t.Fatalf("DBDirName not deterministic: %q != %q", name, name2)
	}
	// Different pepper must produce a different directory name.
	if name3 := DBDirName("argon2", "otherpepper"); name3 == name {
		t.Fatal("different peppers should produce different dir names")
	}
}

func TestLoadDBFileNotExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nonexistent.db")

	db, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB should create empty DB for nonexistent file, got: %v", err)
	}
	if len(db.Entries()) != 0 {
		t.Fatalf("expected 0 entries for new DB, got %d", len(db.Entries()))
	}
	if db.Header().OrganizationPlayerHash != "argon2" {
		t.Fatal("new DB should have argon2 hash algorithm")
	}
}

func TestEmptyDBRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "players.db")

	db := NewMemDB(testPepper)
	if err := SaveDB(db, dbPath); err != nil {
		t.Fatalf("SaveDB: %v", err)
	}

	loaded, err := LoadDB(dbPath, testPepper)
	if err != nil {
		t.Fatalf("LoadDB: %v", err)
	}
	if len(loaded.Entries()) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(loaded.Entries()))
	}
}
