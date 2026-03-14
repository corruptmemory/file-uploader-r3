#!/usr/bin/env bash
# Spec 08: Player Deduplication Database — Dev Test Harness
# Tests the playerdb package for spec compliance.

set -euo pipefail
cd "$(dirname "$0")/.."

PASS=0
FAIL=0

pass() {
  echo "  PASS: $1"
  ((PASS++)) || true
}

fail() {
  echo "  FAIL: $1"
  ((FAIL++)) || true
}

check() {
  if eval "$2"; then
    pass "$1"
  else
    fail "$1"
  fi
}

echo "=== Spec 08: Player Deduplication Database ==="
echo

# --- Section 1: Source code structure checks ---
echo "--- Source Structure ---"

SRC="internal/playerdb/playerdb.go"
TEST="internal/playerdb/playerdb_test.go"

check "playerdb.go exists" "[ -f '$SRC' ]"
check "playerdb_test.go exists" "[ -f '$TEST' ]"

# --- Section 2: Data Model ---
echo
echo "--- Data Model ---"

# Entry struct with correct CSV tags
check "Entry struct exists" "grep -q 'type Entry struct' '$SRC'"
check "Entry has MetaID csv tag" "grep -q 'MetaID.*csv:\"meta_id\"' '$SRC'"
check "Entry has OrganizationPlayerID csv tag" "grep -q 'OrganizationPlayerID.*csv:\"organization_player_id\"' '$SRC'"
check "Entry has Country csv tag" "grep -q 'Country.*csv:\"country\"' '$SRC'"
check "Entry has State csv tag" "grep -q 'State.*csv:\"state\"' '$SRC'"

# DBHeader struct with correct TOML tags
check "DBHeader struct exists" "grep -q 'type DBHeader struct' '$SRC'"
check "DBHeader has OrganizationPlayerHash toml tag" "grep -q 'toml:\"organization-player-hash\"' '$SRC'"
check "DBHeader has OrganizationPlayerIDPepperHash toml tag" "grep -q 'toml:\"organization-player-id-pepper-hash\"' '$SRC'"

# Composite key
check "compositeKey function exists" "grep -q 'func compositeKey' '$SRC'"
check "compositeKey lowercases components" "grep -q 'strings.ToLower' '$SRC'"
check "compositeKey uses colon separator" "grep -q 'orgPlayerID.*:.*country.*:.*state' '$SRC'"

# --- Section 3: File Format ---
echo
echo "--- File Format ---"

check "header delimiter constant defined" "grep -qF -- '--- END OF HEADER ---' '$SRC'"
check "uses BurntSushi/toml for header" "grep -q '\"github.com/BurntSushi/toml\"' '$SRC'"
check "uses gocsv for CSV body" "grep -q '\"github.com/gocarina/gocsv\"' '$SRC'"

# Pepper hash format
check "pepperHash uses SHA-256" "grep -q 'sha256.Sum256' '$SRC'"
check "pepperHash format is sha256:hex" "grep -q 'sha256:%x' '$SRC'"

# --- Section 4: PlayerDB Interface ---
echo
echo "--- PlayerDB Interface ---"

check "PlayerDB interface defined in playerdb package" "grep -q 'type PlayerDB interface' '$SRC'"
check "PlayerDB has GetPlayerByOrgPlayerID" "grep -q 'GetPlayerByOrgPlayerID' '$SRC'"
check "PlayerDB has AddEntry" "grep -q 'AddEntry(metaID' '$SRC'"
check "PlayerDB has Merge" "grep -q 'Merge(other PlayerDB)' '$SRC'"
check "PlayerDB has Header" "grep -q 'Header() DBHeader' '$SRC'"

# memDB implementation
check "memDB struct exists" "grep -q 'type memDB struct' '$SRC'"
check "memDB has entries slice" "grep -q 'entries.*\[\]Entry' '$SRC'"
check "memDB has index map" "grep -q 'index.*map\[string\]int' '$SRC'"
check "NewMemDB constructor exists" "grep -q 'func NewMemDB' '$SRC'"
check "NewMemDB sets algorithm to argon2" "grep -q '\"argon2\"' '$SRC'"

# First-write-wins logic
check "AddEntry has backfill logic (empty MetaID check)" "grep -q 'MetaID == \"\"' '$SRC'"

# --- Section 5: ConcurrentPlayerDB ---
echo
echo "--- ConcurrentPlayerDB ---"

check "ConcurrentPlayerDB struct exists" "grep -q 'type ConcurrentPlayerDB struct' '$SRC'"
check "ConcurrentPlayerDB has commands channel" "grep -q 'commands chan playerDBCommand' '$SRC'"
check "ConcurrentPlayerDB has sync.WaitGroup" "grep -q 'wg.*sync.WaitGroup' '$SRC'"
check "command buffer size is 1000" "grep -q 'make(chan playerDBCommand, 1000)' '$SRC'"

# Command tags
check "playerDBLookup command tag" "grep -q 'playerDBLookup' '$SRC'"
check "playerDBAdd command tag" "grep -q 'playerDBAdd' '$SRC'"

# Actor methods
check "NewConcurrentPlayerDB constructor" "grep -q 'func NewConcurrentPlayerDB' '$SRC'"
check "ConcurrentPlayerDB.GetPlayerByOrgPlayerID uses chanutil" "grep -q 'chanutil.SendReceiveMessage.*playerDBCommand.*lookupResult' '$SRC'"
check "ConcurrentPlayerDB.AddEntry is fire-and-forget" "grep -q 'Fire-and-forget' '$SRC'"
check "ConcurrentPlayerDB.Close closes channel and waits" "grep -A2 'func.*ConcurrentPlayerDB.*Close' '$SRC' | grep -q 'close(c.commands)'"
check "ConcurrentPlayerDB.Close calls wg.Wait" "grep -A3 'func.*ConcurrentPlayerDB.*Close' '$SRC' | grep -q 'c.wg.Wait()'"

# Actor-routed Save, Entries, Merge (code review fix)
check "ConcurrentPlayerDB.Save routes through actor (SendReceiveError)" "grep -A5 'func.*ConcurrentPlayerDB.*Save' '$SRC' | grep -q 'chanutil.SendReceiveError'"
check "ConcurrentPlayerDB.Entries routes through actor" "grep -A5 'func.*ConcurrentPlayerDB.*Entries' '$SRC' | grep -q 'chanutil.SendReceiveMessage'"
check "ConcurrentPlayerDB.Merge routes through actor" "grep -A5 'func.*ConcurrentPlayerDB.*Merge' '$SRC' | grep -q 'chanutil.SendReceiveError'"

# Spec calls for InnerDB() method
if grep -q 'func.*ConcurrentPlayerDB.*InnerDB' "$SRC"; then
  pass "ConcurrentPlayerDB.InnerDB() exists (per spec)"
else
  fail "ConcurrentPlayerDB.InnerDB() missing — spec section 4 requires it"
fi

# --- Section 6: DevNullDB ---
echo
echo "--- DevNullDB ---"

check "DevNullDB struct exists" "grep -q 'type DevNullDB struct{}' '$SRC'"
check "DevNullDB is singleton" "grep -q 'devNullSingleton' '$SRC'"
check "GetDevNullDB returns singleton" "grep -q 'func GetDevNullDB' '$SRC'"
check "DevNullDB.GetPlayerByOrgPlayerID returns empty/false" "grep -A2 'func.*DevNullDB.*GetPlayerByOrgPlayerID' '$SRC' | grep -q 'return \"\", false'"
check "DevNullDB.AddEntry is no-op" "grep -q 'func.*DevNullDB.*AddEntry.*{}' '$SRC'"
check "DevNullDB.Merge is no-op" "grep -q 'func.*DevNullDB.*Merge.*{}' '$SRC'"
check "DevNullDB.Header returns empty DBHeader" "grep -q 'func.*DevNullDB.*Header.*{ return DBHeader{} }' '$SRC'"

# --- Section 7: Persistence ---
echo
echo "--- Persistence ---"

check "SaveDB function exists" "grep -q 'func SaveDB(db PlayerDB, filePath string) error' '$SRC'"
check "SaveDB uses temp file" "grep -q 'CreateTemp' '$SRC'"
check "SaveDB calls Sync" "grep -q 'tmp.Sync()' '$SRC'"
check "SaveDB calls Rename (atomic)" "grep -q 'os.Rename' '$SRC'"
check "SaveDB cleans up temp on error" "grep -c 'os.Remove(tmpName)' '$SRC' | grep -q '[3-9]'"
check "LoadDB function exists" "grep -q 'func LoadDB(filePath string, pepper string)' '$SRC'"
check "LoadDB creates empty DB for nonexistent file" "grep -q 'os.IsNotExist' '$SRC'"
check "LoadDB validates pepper hash" "grep -q 'pepper mismatch' '$SRC'"

# Empty slice header handling
check "SaveDB writes manual CSV header for empty entries" "grep -q 'meta_id,organization_player_id,country,state' '$SRC'"

# --- Section 8: Directory Naming ---
echo
echo "--- Directory Naming ---"

check "DBDirName function exists" "grep -q 'func DBDirName' '$SRC'"
check "DBDirName uses RawURLEncoding" "grep -q 'base64.RawURLEncoding' '$SRC'"
check "DBDirName format is playersdb-algo-pepper" "grep -q 'playersdb-%s-%s' '$SRC'"

# --- Section 9: Snapshot ---
echo
echo "--- Snapshot ---"

check "CreateSnapshot function exists" "grep -q 'func CreateSnapshot(dbPath string, snapshotDir string)' '$SRC'"
check "Snapshot uses correct timestamp format" "grep -q '20060102_150405.000000000' '$SRC'"
check "Snapshot filename format is players-timestamp.db" "grep -q 'players-%s.db' '$SRC'"
check "Snapshot copies file via io.Copy" "grep -q 'io.Copy' '$SRC'"
check "Snapshot calls Sync" "grep -q 'dst.Sync()' '$SRC'"
check "Snapshot creates snapshot dir via MkdirAll" "grep -q 'os.MkdirAll(snapshotDir' '$SRC'"
check "Snapshot file permissions are 0600" "grep -q '0o600' '$SRC'"

# --- Section 10: Interface Compliance ---
echo
echo "--- Interface Compliance ---"

check "memDB satisfies PlayerDB" "grep -q '_ PlayerDB.*=.*\*memDB' '$SRC'"
check "DevNullDB satisfies PlayerDB" "grep -q '_ PlayerDB.*=.*\*DevNullDB' '$SRC'"
check "memDB satisfies hashers.PlayerDB" "grep -q '_ hashers.PlayerDB.*=.*\*memDB' '$SRC'"
check "ConcurrentPlayerDB satisfies hashers.PlayerDB" "grep -q '_ hashers.PlayerDB.*=.*\*ConcurrentPlayerDB' '$SRC'"
check "DevNullDB satisfies hashers.PlayerDB" "grep -q '_ hashers.PlayerDB.*=.*\*DevNullDB' '$SRC'"

# --- Section 11: Test Coverage ---
echo
echo "--- Test Coverage ---"

check "Test: AddEntry + GetPlayer round-trip" "grep -q 'TestAddEntryAndGetPlayer' '$TEST'"
check "Test: First-write-wins" "grep -q 'TestFirstWriteWins' '$TEST'"
check "Test: MetaID backfill" "grep -q 'TestMetaIDBackfill' '$TEST'"
check "Test: Merge two DBs" "grep -q 'TestMergeTwoDBs' '$TEST'"
check "Test: File round-trip" "grep -q 'TestFileRoundTrip' '$TEST'"
check "Test: Pepper mismatch" "grep -q 'TestPepperMismatch' '$TEST'"
check "Test: Case-insensitive key" "grep -q 'TestCaseInsensitiveKey' '$TEST'"
check "Test: DevNullDB" "grep -q 'TestDevNullDB' '$TEST'"
check "Test: Atomic write" "grep -q 'TestAtomicWrite' '$TEST'"
check "Test: Snapshot creates copy" "grep -q 'TestSnapshotCreatesCopy' '$TEST'"
check "Test: ConcurrentPlayerDB basic" "grep -q 'TestConcurrentPlayerDB' '$TEST'"
check "Test: Save drains queue" "grep -q 'TestConcurrentPlayerDBSaveDrainsQueue' '$TEST'"
check "Test: Merge through actor" "grep -q 'TestConcurrentPlayerDBMergeThroughActor' '$TEST'"
check "Test: DB dir naming" "grep -q 'TestDBDirName' '$TEST'"
check "Test: Load nonexistent file" "grep -q 'TestLoadDBFileNotExist' '$TEST'"
check "Test: Empty DB round-trip" "grep -q 'TestEmptyDBRoundTrip' '$TEST'"

# --- Section 12: Run Tests with Race Detector ---
echo
echo "--- Running playerdb tests (race detector) ---"

TEST_OUTPUT=$(./build.sh -t 2>&1)
if echo "$TEST_OUTPUT" | grep -q 'FAIL'; then
  fail "playerdb tests have failures"
  echo "$TEST_OUTPUT" | grep -A5 'FAIL'
else
  pass "all playerdb tests pass with race detector"
fi

# Verify specific package test count
PLAYERDB_COUNT=$(echo "$TEST_OUTPUT" | grep -c 'ok.*playerdb' || true)
check "playerdb package appears in test output" "[ '$PLAYERDB_COUNT' -ge 1 ]"

# --- Section 13: gocsv dependency ---
echo
echo "--- Dependencies ---"

check "gocsv dependency in go.mod" "grep -q 'gocarina/gocsv' go.mod"
check "gocsv module is resolvable" "go list -m github.com/gocarina/gocsv > /dev/null 2>&1"

# --- Section 14: File format validation (via go test) ---
echo
echo "--- File Format Validation (via go test) ---"

# The built-in tests already cover file format, so we verify test output
TEST_VERBOSE=$(go test -v -race -run 'TestFileRoundTrip|TestEmptyDBRoundTrip|TestPepperMismatch|TestLoadDBFileNotExist|TestAtomicWrite|TestSnapshotCreatesCopy' ./internal/playerdb/ 2>&1)
if echo "$TEST_VERBOSE" | grep -q '^ok'; then
  pass "file format validation tests pass"
  # Count passing tests
  FORMAT_PASS=$(echo "$TEST_VERBOSE" | grep -cF -- '--- PASS:' || true)
  pass "file format tests: $FORMAT_PASS tests passed"
else
  fail "file format validation tests failed"
  echo "$TEST_VERBOSE" | tail -20
fi

# Verify concurrent actor tests pass
ACTOR_VERBOSE=$(go test -v -race -run 'TestConcurrentPlayerDB' ./internal/playerdb/ 2>&1)
if echo "$ACTOR_VERBOSE" | grep -q '^ok'; then
  ACTOR_PASS=$(echo "$ACTOR_VERBOSE" | grep -cF -- '--- PASS:' || true)
  pass "actor tests: $ACTOR_PASS tests passed"
else
  fail "actor tests failed"
  echo "$ACTOR_VERBOSE" | tail -20
fi

# --- Section 15: Spec Compliance Deviations ---
echo
echo "--- Spec Compliance Deviations ---"

# Spec says GetPlayerByOrgPlayerID returns (Entry, bool)
# Implementation returns (metaID string, found bool) to match hashers.PlayerDB
RETURN_TYPE=$(grep 'GetPlayerByOrgPlayerID.*string.*bool' internal/playerdb/playerdb.go | head -1)
if echo "$RETURN_TYPE" | grep -q 'metaID string, found bool'; then
  echo "  NOTE: GetPlayerByOrgPlayerID returns (string, bool) not (Entry, bool) — adapted for hashers.PlayerDB interface"
else
  pass "GetPlayerByOrgPlayerID return type matches spec"
fi

# Spec section 4 lists InnerDB() as a required method
if grep -q 'InnerDB' internal/playerdb/playerdb.go; then
  pass "InnerDB method present per spec section 4"
else
  fail "InnerDB() method missing from ConcurrentPlayerDB — spec section 4 requires it for persistence"
fi

# --- Results ---
echo
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
