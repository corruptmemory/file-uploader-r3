#!/usr/bin/env bash
set -euo pipefail

PASS=0
FAIL=0
RESULTS=""

pass() { PASS=$((PASS+1)); RESULTS+="  PASS: $1"$'\n'; }
fail() { FAIL=$((FAIL+1)); RESULTS+="  FAIL: $1"$'\n'; }
check() { if eval "$2" >/dev/null 2>&1; then pass "$1"; else fail "$1 -- $3"; fi; }

echo "=== Spec 04: Hashing and Name Normalization Tests ==="

HASHERS_GO="internal/hashers/hashers.go"
SUBDIVISIONS_GO="internal/hashers/subdivisions.go"
HASHERS_TEST="internal/hashers/hashers_test.go"

echo ""
echo "--- Package Structure ---"
check "hashers.go exists" "test -f $HASHERS_GO" "file missing"
check "subdivisions.go exists" "test -f $SUBDIVISIONS_GO" "file missing"
check "hashers_test.go exists" "test -f $HASHERS_TEST" "file missing"

echo ""
echo "--- Hashers Interface ---"
check "Hashers interface defined" "grep -q 'type Hashers interface' $HASHERS_GO" "interface not defined"
check "PlayerUniqueHasher in interface" "grep -q 'PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string' $HASHERS_GO" "method not in interface"
check "OrganizationPlayerIDHasher in interface" "grep -q 'OrganizationPlayerIDHasher(playerID, country, state string) string' $HASHERS_GO" "method not in interface"
check "SaveDB in interface" "grep -q 'SaveDB() error' $HASHERS_GO" "method not in interface"

echo ""
echo "--- PlayerDB Interface ---"
check "PlayerDB interface defined" "grep -q 'type PlayerDB interface' $HASHERS_GO" "interface not defined"
check "GetPlayerByOrgPlayerID method" "grep -q 'GetPlayerByOrgPlayerID' $HASHERS_GO" "method missing"
check "AddEntry method" "grep -q 'AddEntry' $HASHERS_GO" "method missing"

echo ""
echo "--- PlayerDataHasher Struct ---"
check "PlayerDataHasher struct defined" "grep -q 'type PlayerDataHasher struct' $HASHERS_GO" "struct not defined"
check "useOnlyFirstLetterOfFirstName field" "grep -q 'useOnlyFirstLetterOfFirstName.*bool' $HASHERS_GO" "field missing"
check "dbPath field" "grep -q 'dbPath.*string' $HASHERS_GO" "field missing"
check "uniqueIDPepper field" "grep -q 'uniqueIDPepper.*string' $HASHERS_GO" "field missing"
check "orgPlayerIDPepper field" "grep -q 'orgPlayerIDPepper.*string' $HASHERS_GO" "field missing"
check "nameProcessor field" "grep -q 'nameProcessor.*func(string) string' $HASHERS_GO" "field missing"
check "playersdb field" "grep -q 'playersdb.*PlayerDB' $HASHERS_GO" "field missing"

echo ""
echo "--- NewPlayerDataHasher Constructor ---"
check "NewPlayerDataHasher function exists" "grep -q 'func NewPlayerDataHasher' $HASHERS_GO" "constructor missing"
# Verify it accepts all required params
check "Constructor takes useOnlyFirstLetterOfFirstName param" "grep -A8 'func NewPlayerDataHasher' $HASHERS_GO | grep -q 'useOnlyFirstLetterOfFirstName bool'" "param missing"
check "Constructor takes dbPath param" "grep -A8 'func NewPlayerDataHasher' $HASHERS_GO | grep -q 'dbPath string'" "param missing"
check "Constructor takes pepper params" "grep -A8 'func NewPlayerDataHasher' $HASHERS_GO | grep -q 'uniqueIDPepper'" "param missing"
check "Constructor takes nameProcessor param" "grep -A8 'func NewPlayerDataHasher' $HASHERS_GO | grep -q 'nameProcessor'" "param missing"
check "Constructor takes playersdb param" "grep -A8 'func NewPlayerDataHasher' $HASHERS_GO | grep -q 'playersdb PlayerDB'" "param missing"

echo ""
echo "--- Argon2 Parameters ---"
check "Uses argon2id (IDKey)" "grep -q 'argon2.IDKey' $HASHERS_GO" "must use argon2id not argon2i"
check "Time cost = 3" "grep 'argon2.IDKey' $HASHERS_GO | grep -q ', 3,'" "time cost must be 3"
check "Memory = 32*1024" "grep 'argon2.IDKey' $HASHERS_GO | grep -qE '32\*1024|32768'" "memory must be 32*1024"
check "Parallelism = 4" "grep 'argon2.IDKey' $HASHERS_GO | grep -q ', 4,'" "parallelism must be 4"
check "Key length = 32" "grep 'argon2.IDKey' $HASHERS_GO | grep -q ', 32)'" "key length must be 32"
check "Uses StdEncoding for base64" "grep -q 'base64.StdEncoding' $HASHERS_GO" "must use standard base64"

echo ""
echo "--- PlayerUniqueHasher Logic ---"
# Verify truncation before normalization when flag is true
check "First-letter flag truncates before normalization" \
    "grep -A5 'func.*PlayerUniqueHasher' $HASHERS_GO | grep -q 'useOnlyFirstLetterOfFirstName'" \
    "flag check missing"
# Verify nameProcessor is called on firstName and lastName
check "nameProcessor applied to firstName" "grep -q 'h.nameProcessor(firstName)' $HASHERS_GO" "missing nameProcessor on firstName"
check "nameProcessor applied to lastName" "grep -q 'h.nameProcessor(lastName)' $HASHERS_GO" "missing nameProcessor on lastName"
# Verify first char extraction after normalization
check "firstName truncated to first char after normalization" "grep -q 'firstName\[:1\]' $HASHERS_GO" "missing first char extraction"

echo ""
echo "--- PlayerUniqueHasher Cleartext Format ---"
# Spec: uniqueIDPepper + last4SSN + firstName[:1] + lastName + dob (no delimiters)
check "Cleartext uses uniqueIDPepper as salt" "grep 'argon2Hash(cleartext, h.uniqueIDPepper)' $HASHERS_GO" "wrong salt"
check "Cleartext concatenation (no colons)" \
    "grep -q 'h.uniqueIDPepper + last4SSN + firstName + lastName + dob' $HASHERS_GO" \
    "cleartext format wrong - spec says no delimiters"

echo ""
echo "--- OrganizationPlayerIDHasher Logic ---"
# Verify cache check comes first
check "Cache check before hash computation" \
    "grep -n 'GetPlayerByOrgPlayerID\|cleartext.*orgPlayerIDPepper' $HASHERS_GO | head -1 | grep -q 'GetPlayerByOrgPlayerID'" \
    "cache check must come before hash"
# Verify cleartext format with colon delimiters
check "Cleartext uses colon delimiters" \
    "grep -q 'orgPlayerIDPepper.*:.*playerID.*:.*country.*:.*state' $HASHERS_GO" \
    "cleartext must use colon delimiters per spec"
# Verify AddEntry called after hash
check "AddEntry called with metaID" "grep -q 'playersdb.AddEntry(metaID' $HASHERS_GO" "AddEntry not called"
check "Hash uses orgPlayerIDPepper as salt" "grep -q 'argon2Hash(cleartext, h.orgPlayerIDPepper)' $HASHERS_GO" "wrong salt"

echo ""
echo "--- SaveDB ---"
check "SaveDB method exists" "grep -q 'func.*PlayerDataHasher.*SaveDB' $HASHERS_GO" "SaveDB missing"

echo ""
echo "--- ProcessName Function ---"
check "ProcessName function exists" "grep -q 'func ProcessName' $HASHERS_GO" "function missing"
check "ProcessName iterates until stable" "grep -q 'if next == current' $HASHERS_GO" "iteration check missing"
check "Handles m-dash to hyphen" "grep -q '2014' $HASHERS_GO" "m-dash handling missing"
check "Handles n-dash to hyphen" "grep -q '2013' $HASHERS_GO" "n-dash handling missing"
check "Uses unicode.IsPunct for other punctuation" "grep -q 'unicode.IsPunct' $HASHERS_GO" "IsPunct check missing"
check "NFD decomposition for diacriticals" "grep -q 'norm.NFD' $HASHERS_GO" "NFD missing"
check "Removes combining marks (Mn)" "grep -q 'unicode.Mn' $HASHERS_GO" "Mn category missing"
check "NFC recomposition" "grep -q 'norm.NFC' $HASHERS_GO" "NFC missing"
check "Non-ASCII replacement (>127)" "grep -q 'r > 127' $HASHERS_GO" "non-ASCII check missing"
check "Whitespace collapse" "grep -qE 'multiSpace|\\\\s\{2' $HASHERS_GO" "whitespace collapse missing"
check "Leading/trailing hyphen removal" "grep -q 'TrimLeft.*-' $HASHERS_GO" "hyphen trim missing"
check "Multiple hyphen collapse" "grep -qE 'multiHyphen|-\{2' $HASHERS_GO" "hyphen collapse missing"
check "ToLower applied" "grep -q 'strings.ToLower' $HASHERS_GO" "lowercase missing"
check "TrimSpace applied" "grep -q 'strings.TrimSpace' $HASHERS_GO" "TrimSpace missing"

echo ""
echo "--- GetCountrySubdivisions ---"
check "GetCountrySubdivisions function exists" "grep -q 'func GetCountrySubdivisions' $SUBDIVISIONS_GO" "function missing"
check "Accepts US" "grep -q '\"US\"' $SUBDIVISIONS_GO" "US not handled"
check "Accepts USA" "grep -q '\"USA\"' $SUBDIVISIONS_GO" "USA not handled"
check "Returns error for unknown" "grep -q 'unrecognized country' $SUBDIVISIONS_GO" "error missing"
check "Input trimmed and uppercased" "grep -q 'strings.ToUpper(strings.TrimSpace' $SUBDIVISIONS_GO" "no trim/upper"
check "Returns copy of slice" "grep -q 'copy(result' $SUBDIVISIONS_GO" "should return copy"

# Count state codes
STATE_COUNT=$(grep -oE '"[A-Z][A-Z]"' $SUBDIVISIONS_GO | wc -l || true)
if [ "$STATE_COUNT" -ge 51 ]; then
    pass "51 state codes present (found $STATE_COUNT)"
else
    fail "Expected at least 51 state codes, found $STATE_COUNT"
fi

echo ""
echo "--- Compile-time Interface Check ---"
check "Compile-time Hashers interface check" \
    "grep -q 'var _ Hashers = (\*PlayerDataHasher)(nil)' $HASHERS_GO" \
    "compile-time check missing"

echo ""
echo "--- Test Coverage ---"
check "Test: known output regression" "grep -q 'TestArgon2KnownOutput' $HASHERS_TEST" "test missing"
check "Test: determinism" "grep -q 'TestArgon2Determinism' $HASHERS_TEST" "test missing"
check "Test: different inputs" "grep -q 'TestArgon2DifferentInputs' $HASHERS_TEST" "test missing"
check "Test: parameter check" "grep -q 'TestArgon2Parameters' $HASHERS_TEST" "test missing"
check "Test: PlayerUniqueHasher deterministic" "grep -q 'TestPlayerUniqueHasherDeterministic' $HASHERS_TEST" "test missing"
check "Test: OrgPlayerIDHasher cache" "grep -q 'TestOrgPlayerIDHasherUsesCache' $HASHERS_TEST" "test missing"
check "Test: OrgPlayerIDHasher country/state" "grep -q 'TestOrgPlayerIDHasherIncludesCountryState' $HASHERS_TEST" "test missing"
check "Test: first-letter flag" "grep -q 'TestPlayerUniqueHasherFirstLetterFlag' $HASHERS_TEST" "test missing"
check "Test: ProcessName table" "grep -q 'TestProcessName' $HASHERS_TEST" "test missing"
check "Test: GetCountrySubdivisions US" "grep -q 'TestGetCountrySubdivisionsUS' $HASHERS_TEST" "test missing"
check "Test: GetCountrySubdivisions USA" "grep -q 'TestGetCountrySubdivisionsUSA' $HASHERS_TEST" "test missing"
check "Test: case insensitive country" "grep -q 'TestGetCountrySubdivisionsCaseInsensitive' $HASHERS_TEST" "test missing"
check "Test: unknown country" "grep -q 'TestGetCountrySubdivisionsUnknown' $HASHERS_TEST" "test missing"
check "Test: all 51 codes" "grep -q 'TestGetCountrySubdivisionsAll51Codes' $HASHERS_TEST" "test missing"

echo ""
echo "--- Test Execution ---"
# Run actual tests with race detector
TEST_OUTPUT=$(cd /home/jim/projects/file-uploader-r3 && go test -race -v ./internal/hashers/ 2>&1)
if echo "$TEST_OUTPUT" | grep -q '^ok'; then
    pass "all hashers tests pass with race detector"
else
    fail "hashers tests failed: $(echo "$TEST_OUTPUT" | tail -5)"
fi

# Count test functions
TEST_COUNT=$(echo "$TEST_OUTPUT" | grep -cF 'PASS:' || true)
if [ "$TEST_COUNT" -ge 14 ]; then
    pass "at least 14 test cases pass ($TEST_COUNT found)"
else
    fail "expected at least 14 test cases, found $TEST_COUNT"
fi

# Check for race conditions
if echo "$TEST_OUTPUT" | grep -qi 'DATA RACE'; then
    fail "data race detected in tests"
else
    pass "no data races detected"
fi

echo ""
echo "--- ProcessName Edge Cases (via go test) ---"
# Verify specific test cases from spec are in the test file
check "Test: SMITH -> smith" "grep -q '\"SMITH\".*\"smith\"' $HASHERS_TEST" "test case missing"
check "Test: Müller -> muller" "grep -qE 'M.ller.*muller|\\\\u00fc' $HASHERS_TEST" "test case missing"
check "Test: François -> francois" "grep -qE 'Fran.ois.*francois|\\\\u00e7' $HASHERS_TEST" "test case missing"
check "Test: Muñoz -> munoz" "grep -qE 'Mu.oz.*munoz|\\\\u00f1' $HASHERS_TEST" "test case missing"
check "Test: O'Brien -> o brien" "grep -q \"O'Brien\" $HASHERS_TEST" "test case missing"
check "Test: Smith-Jones preserved" "grep -q 'Smith-Jones.*smith-jones' $HASHERS_TEST" "test case missing"
check "Test: empty string" "grep -qE '\"\",.*\"\"' $HASHERS_TEST" "test case missing"
check "Test: Björk -> bjork" "grep -qE 'Bj.rk.*bjork|\\\\u00f6' $HASHERS_TEST" "test case missing"
check "Test: Dvořák -> dvorak" "grep -qE 'Dvo..k.*dvorak|\\\\u0159' $HASHERS_TEST" "test case missing"
check "Test: Ñoño -> nono" "grep -qE '\\\\u00d1|Ñoño' $HASHERS_TEST" "test case missing"

echo ""
echo "--- Dependency Check ---"
check "Uses golang.org/x/crypto/argon2" "grep -q 'golang.org/x/crypto' go.mod" "argon2 dependency missing"
check "Uses golang.org/x/text" "grep -q 'golang.org/x/text' go.mod" "text dependency missing"

echo ""
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
echo "$RESULTS" | grep FAIL || true
