# 04 — Hashing and Name Normalization

**Dependencies:** 01 (project compiles). No other specs needed.

**Produces:** `internal/hashers/` package — Argon2 hashing, name normalization, `Hashers` interface.

---

## 1. Hashers Interface

```go
package hashers

type Hashers interface {
    PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string
    OrganizationPlayerIDHasher(playerID, country, state string) string
    SaveDB() error
}
```

---

## 2. Argon2 Parameters (Shared by Both Hashers)

| Parameter | Value |
|---|---|
| Algorithm | Argon2id (`golang.org/x/crypto/argon2.Key`) |
| Time cost | 3 iterations |
| Memory | 32 * 1024 KiB (32 MB) |
| Parallelism | 4 threads |
| Output length | 32 bytes |
| Salt | pepper bytes (`[]byte(pepper)`) |
| Output encoding | Standard base64 (`base64.StdEncoding`) |

---

## 3. PlayerDataHasher Implementation

```go
type PlayerDataHasher struct {
    useOnlyFirstLetterOfFirstName bool
    dbPath                        string
    uniqueIDPepper                string
    orgPlayerIDPepper             string
    nameProcessor                 func(string) string
    playersdb                     PlayerDB
}

func NewPlayerDataHasher(
    useOnlyFirstLetterOfFirstName bool,
    dbPath string,
    uniqueIDPepper, orgPlayerIDPepper string,
    nameProcessor func(string) string,
    playersdb PlayerDB,
) *PlayerDataHasher
```

The `nameProcessor` parameter is always `ProcessName` (Section 4). Injected for testability.

The `PlayerDB` interface is defined here for dependency inversion (implemented in spec 08):

```go
type PlayerDB interface {
    GetPlayerByOrgPlayerID(playerID, country, state string) (metaID string, found bool)
    AddEntry(metaID, playerID, country, state string)
}
```

### 3.1 PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string

1. If `useOnlyFirstLetterOfFirstName` is `true` and `len(firstName) > 1`: truncate `firstName` to first character.
2. Apply `nameProcessor` to `firstName`.
3. Apply `nameProcessor` to `lastName`.
4. Take only the first character of normalized `firstName`: `firstName = firstName[:1]`.
5. Construct cleartext: `uniqueIDPepper + last4SSN + firstName[:1] + lastName + dob` (simple concatenation, no delimiters).
6. Hash: `argon2.Key([]byte(cleartext), []byte(uniqueIDPepper), 3, 32*1024, 4, 32)`.
7. Encode: `base64.StdEncoding.EncodeToString(hash)`.
8. Return the base64 string.

### 3.2 OrganizationPlayerIDHasher(playerID, country, state string) string

1. Check PlayerDB cache: `playersdb.GetPlayerByOrgPlayerID(playerID, country, state)`.
2. If cache hit: return cached MetaID immediately.
3. If cache miss:
   a. Construct cleartext: `orgPlayerIDPepper + ":" + playerID + ":" + country + ":" + state`.
   b. Hash: `argon2.Key([]byte(cleartext), []byte(orgPlayerIDPepper), 3, 32*1024, 4, 32)`.
   c. Encode: `base64.StdEncoding.EncodeToString(hash)`.
   d. Cache: `playersdb.AddEntry(metaID, playerID, country, state)`.
   e. Return the base64 string.

**CRITICAL:** The hash input MUST incorporate country and state: `pepper + ":" + playerID + ":" + country + ":" + state`. Colon delimiters prevent ambiguity.

### 3.3 SaveDB() error

Persists the PlayerDB to disk at `dbPath`. Called after each file completes.

---

## 4. Name Normalization: ProcessName

```go
func ProcessName(in string) string
```

Iterative normalization — repeats until stable (output equals input).

**Each iteration performs these steps in order:**

1. **Process punctuation:**
   - Hyphens (`-`, U+002D): keep as-is.
   - M-dash (`—`, U+2014): convert to hyphen.
   - N-dash (`–`, U+2013): convert to hyphen.
   - All other `unicode.IsPunct(r)`: replace with space.

2. **Remove diacritical marks:**
   - Unicode NFD decomposition.
   - Strip combining marks (Unicode category `Mn`).
   - Recompose to NFC.
   - Implementation:
     ```go
     transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
     ```

3. **Replace non-ASCII:** Any codepoint > 127 → space.

4. **Collapse whitespace:** Multiple consecutive whitespace → single space.

5. **Normalize hyphens:**
   - Remove leading hyphens.
   - Remove trailing hyphens.
   - Remove whitespace adjacent to hyphens (`" - "` → `"-"`, `"a - b"` → `"a-b"`).
   - Collapse multiple consecutive hyphens → single hyphen.

6. **Trim whitespace** from both ends.

7. **Lowercase** the entire string.

8. **Repeat from step 1.** Return when output equals input (stable).

### Examples

| Input | Output |
|---|---|
| `"Zoe-Andre O'Brien"` | `"zoe-andre o brien"` |
| `"  MARY--ANN  "` | `"mary-ann"` |
| `"Jean---Pierre"` | `"jean-pierre"` |
| `"Müller"` | `"muller"` |
| `"François"` | `"francois"` |
| `"Björk"` | `"bjork"` |
| `"Dvořák"` | `"dvorak"` |
| `""` | `""` |

**Dependencies:** `golang.org/x/text/transform`, `golang.org/x/text/unicode/norm`, `golang.org/x/text/runes`.

---

## 5. Country/State Validation

```go
func GetCountrySubdivisions(country string) ([]string, error)
```

- Input trimmed and uppercased.
- Recognized: `"US"`, `"USA"` → same 51 codes.
- Unrecognized → error.

### US State Codes (51)

```
AL, AK, AZ, AR, CA, CO, CT, DE, FL, GA,
HI, ID, IL, IN, IA, KS, KY, LA, ME, MD,
MA, MI, MN, MS, MO, MT, NE, NV, NH, NJ,
NM, NY, NC, ND, OH, OK, OR, PA, RI, SC,
SD, TN, TX, UT, VT, VA, WA, WV, WI, WY,
DC
```

---

## Tests

### Argon2 Hashing Tests

| Test | Description |
|------|-------------|
| Known input → known output | Hardcoded regression test with fixed pepper, input, expected base64 |
| Determinism | Same input + same pepper → same output |
| Different inputs → different outputs | Vary playerID, verify different hashes |
| Parameters match spec | time=3, memory=32*1024, parallelism=4, output=32 bytes |

### PlayerDataHasher Tests

| Test | Description |
|------|-------------|
| PlayerUniqueHasher deterministic | Same inputs → same output |
| OrgPlayerIDHasher uses cache | First call hashes, second call returns cached value |
| OrgPlayerIDHasher includes country/state | Same playerID, different state → different hash |
| First-letter-of-first-name flag | `useOnlyFirstLetterOfFirstName=true` → truncates before normalization |

### Name Normalization Tests

| Input | Expected |
|---|---|
| `"SMITH"` | `"smith"` |
| `"Müller"` | `"muller"` |
| `"François"` | `"francois"` |
| `"Muñoz"` | `"munoz"` |
| `"O'Brien"` | `"o brien"` |
| `"Smith-Jones"` | `"smith-jones"` |
| `"John  Smith"` | `"john smith"` |
| `"  Smith  "` | `"smith"` |
| `"Ñoño"` | `"nono"` |
| `""` | `""` |
| `"Björk"` | `"bjork"` |
| `"Dvořák"` | `"dvorak"` |

### Country/State Tests

| Test | Description |
|------|-------------|
| `"US"` returns 51 codes | Verify all 50 states + DC |
| `"USA"` same as `"US"` | Alias works |
| `"us"` works (case-insensitive) | Lowercased input accepted |
| `"XX"` returns error | Unknown country rejected |
| All 51 codes valid | Each code appears in the returned list |

## Acceptance Criteria

- [ ] Argon2 parameters match exactly (time=3, memory=32*1024, parallelism=4, keyLen=32)
- [ ] OrgPlayerIDHasher incorporates country+state in hash input with colon delimiters
- [ ] PlayerDB cache is checked before computing hash; new entries cached after
- [ ] ProcessName iterates until stable
- [ ] ProcessName correctly handles hyphens, diacriticals, non-ASCII, whitespace
- [ ] PlayerUniqueHasher applies name normalization internally
- [ ] Country/state validation accepts US/USA with 51 codes
- [ ] All tests pass
