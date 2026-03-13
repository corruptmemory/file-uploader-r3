# 05 — CSV Framework

**Dependencies:** 04-hashing-and-normalization.md (country/state validation, date/time parsing concepts).

**Produces:** `internal/csv/` package — CSVType enum, CSVMetadata interface, RowData, CSVOutputString/Row, InColumnProcessor interface, all processor implementations.

---

## 1. CSVType Enum

```go
type CSVType int

const (
    CSVBets                CSVType = 2
    CSVPlayers             CSVType = 3
    CSVCasinoPlayers       CSVType = 4
    CSVBonus               CSVType = 5
    CSVCasino              CSVType = 6
    CSVCasinoParSheet      CSVType = 7
    CSVComplaints          CSVType = 8
    CSVDemographic         CSVType = 9
    CSVDepositsWithdrawals CSVType = 10
    CSVResponsibleGaming   CSVType = 11
)
```

| CSVType | Name | Slug |
|---|---|---|
| 2 | Bets | bets |
| 3 | Players | players |
| 4 | Casino Players | casino-players |
| 5 | Bonus | bonus |
| 6 | Casino | casino |
| 7 | Casino Par Sheet | casino-par-sheet |
| 8 | Complaints | complaints |
| 9 | Demographic | demographic |
| 10 | Deposits/Withdrawals | deposits-withdrawals |
| 11 | Responsible Gaming | responsible-gaming |

Provide:
- `func (t CSVType) String() string` — human-readable name
- `func (t CSVType) Slug() string` — slug
- `func CSVTypeFromSlug(slug string) (CSVType, error)` — reverse lookup
- `func AllCSVTypes() []CSVType` — all registered types

---

## 2. Core Types

### RowData

```go
type RowData struct {
    RowIndex int               // 1-based line number (excluding header)
    RowMap   map[string]string // column header → cell value
}
```

### CSVOutputString

```go
type CSVOutputString interface {
    String() string
}

type Quoted string   // CSV-escaped via encoding/csv Writer
type Raw string      // as-is, no escaping

const EmptyString = Raw("")
```

- `Quoted.String()`: use `encoding/csv` Writer writing to a buffer to properly escape.
- `Raw.String()`: return value as-is.

### CSVOutputRow

```go
type CSVOutputRow struct {
    Columns []CSVOutputString
}

func (r CSVOutputRow) RowString() string  // join with commas, no trailing newline
```

---

## 3. InColumnProcessor Interface

```go
type InColumnProcessor interface {
    InputColumns() []string
    OutputColumns() []string
    OutputUniqueID() bool
    OutputMetaID() bool
    Process(rowdata RowData) ([]CSVOutputString, error)
}
```

---

## 4. CSVMetadata Interface

```go
type CSVMetadata interface {
    Type() CSVType
    ColumnData() []InColumnProcessor
    MatchHeaders(headers []string) bool
    ProcessRow(rowdata RowData) (CSVOutputRow, error)
    OutputHeaders() []string
}
```

**Implementation: `simpleCSVMetadata`**

- `MatchHeaders`: collects ALL input columns from all processors. Returns `true` iff every required column appears in headers. Extra columns ignored. Case-sensitive exact match.
- `OutputHeaders`: iterates processors, collects output columns in order.
- `ProcessRow`: calls each processor's `Process()` in sequence, appending results. If any processor errors, stops and propagates.

---

## 5. Processor Implementations

### 5.1 SimpleInColumnProcessor

Single input → single output with transform function.

```go
type simpleInColumnProcessor struct {
    inColumn    string
    outColumn   string
    processFunc func(in string) (CSVOutputString, error)
}
```

#### Boolean Processors

**`NonNilFlexBool(inColumn, outColumn string) InColumnProcessor`**
- Truthy (case-insensitive): `true`, `t`, `yes`, `y`, `1`
- Falsy: `false`, `f`, `no`, `n`, `0`
- Empty/whitespace: **error**
- Output: `Quoted("true")` or `Quoted("false")`

**`NillableFlexBool(inColumn, outColumn string) InColumnProcessor`**
- Same as above, but empty → `EmptyString`

#### Integer Processors

**`NonNillableInt(in, out string)`** — non-nil, any value. Empty → error.
**`NonNillableNonNegInt(in, out string)`** — non-nil, ≥0. Negative → error.
**`NillableInt(in, out string)`** — nullable, any value. Empty → `EmptyString`.
**`NillableNonNegInt(in, out string)`** — nullable, ≥0. Negative → error.

All use `strconv.Atoi`. Output: `Raw`.

#### Float Processors

**`NonNilFloat64Full(in, out string)`** — non-nil. Empty → error. Output: `Raw` with `%g`.
**`NonNilNonNegFloat64Full(in, out string)`** — non-nil, ≥0.
**`NillableFloat64Full(in, out string)`** — nullable. Empty → `EmptyString`.
**`NillableNonNegFloat64Full(in, out string)`** — nullable, ≥0.

All use `strconv.ParseFloat(in, 64)`.

#### String Processors

**`NonEmptyStringWithMax(in, out string, maxLen uint)`** — non-empty, length (chars not bytes) ≤ maxLen. Output: `Quoted`.
**`NillableStringWithMax(in, out string, maxLen uint)`** — nullable. Empty → `EmptyString`. Output: `Quoted`.

#### Date/Time Processors

All trim whitespace before parsing. Try formats in sequence until one succeeds.

**`DateAndTimeNonZeroAndNotAfterNow(in, out string)`** — non-nil datetime. Must not be zero time. Must not be after `time.Now()`. Output: `Quoted` RFC3339.

**`NillableDateAndTimeNotAfterNow(in, out string)`** — same but empty → `EmptyString`.

**`NonNillDate(in, out string)`** — non-nil date only. Output: `Quoted` ISO 8601 date.

**`NonNillMMDDYYYY(in, out string)`** — strict 8-digit MMDDYYYY. Output: `Quoted` ISO 8601 date.

**`NonNilBirthYear(in, out string)`** — non-nil date, extracts 4-digit year only. Output: `Quoted` year string.

**`NonNillableHHMMSSTime(in, out string)`** — strict 6-digit HHMMSS. Output: `Quoted` `"15:04:05"`.

### 5.2 ConstOutColumnProcessor

```go
func ConstantString(outColumn string, value CSVOutputString) InColumnProcessor
```

No input columns. Returns constant value for every row.

### 5.3 MetaIDColumnProcessor

```go
func MetaID(inPlayerID, outMetaID, inCountry, inState, outCountry, outState string,
    hasher func(playerID, country, state string) string) InColumnProcessor

func MetaIDDefault(hasher func(playerID, country, state string) string) InColumnProcessor
func MetaIDDefaultIn(outMetaID, outCountry, outState string, hasher ...) InColumnProcessor
func MetaIDDefaultOut(inPlayerID, inCountry, inState string, hasher ...) InColumnProcessor
```

- InputColumns: `[inPlayerID, inCountry, inState]`
- OutputColumns: `[outMetaID, outCountry, outState]` (outCountry/outState omitted if empty)
- OutputMetaID: `true`
- Processing: validate country via `GetCountrySubdivisions`, validate state if country has subdivisions, call hasher, return `[Raw(metaID), Quoted(country), Quoted(state)]`

### 5.4 MetaIDFixedLocationColumnProcessor

```go
func MetaIDFixedLocation(inPlayerID, outMetaID, country, state string,
    hasher func(playerID, country, state string) string) (InColumnProcessor, error)

func MustMetaIDFixedLocation(...) InColumnProcessor  // panics on error
```

Country/state hardcoded, validated at construction. InputColumns: `[inPlayerID]` only. OutputColumns: `[outMetaID]` only.

### 5.5 UniqueIDProcessor

```go
func UniqueID(inLastName, inFirstName, inLast4SSN, fallbackLast4SSN, inDOB, outUniquePlayerID string,
    hasher func(lastName, firstName, last4SSN, dob string) string) InColumnProcessor

func UniqueIDDefault(hasher func(...) string) InColumnProcessor
func UniqueIDDefaultNullableLast4SSN(hasher func(...) string) InColumnProcessor  // fallback="XXXX"
```

Processing:
1. Read lastName, firstName — trim, error if empty.
2. Read last4SSN — use fallback if missing. Must be exactly 4 ASCII digits.
3. Read DOB — parse as date, format as ISO 8601.
4. Call `hasher(lastName, firstName, last4SSN, dob)` — hasher receives **raw** names (normalization happens inside the hasher).
5. Return `[Quoted(uniquePlayerID)]`.

### 5.6 OrgCountryProcessor

```go
func CountryAndState(inCountry, inState, outCountry, outState string) InColumnProcessor
func CountryAndStateDefault() InColumnProcessor
```

Same validation as MetaID but no hashing. Returns `[Quoted(country), Quoted(state)]`.

---

## 6. Date/Time Format Tables

### DateTime Formats

| Format | Example |
|---|---|
| `2006-01-02 15:04` | `2023-01-15 09:30` |
| `2006-1-2 15:04` | `2023-1-5 9:30` |
| `1/2/06 15:04` | `1/5/23 9:30` |
| `01/02/06 15:04` | `01/05/23 09:30` |
| `1/2/2006 15:04` | `1/5/2023 9:30` |
| `01/02/2006 15:04` | `01/05/2023 09:30` |
| RFC3339 | `2023-01-15T09:30:00Z` |

### Date-Only Formats

| Format | Example |
|---|---|
| `2006-01-02` | `2023-01-15` |
| `01/02/2006` | `01/15/2023` |
| `1/2/2006` | `1/15/2023` |
| `01/02/06` | `01/15/23` |
| `1/2/06` | `1/15/23` |
| `20060102` | `20230115` |

### Special: MMDDYYYY

| Format | Example | Used By |
|---|---|---|
| `01022006` | `01152023` | Casino Par Sheet |

---

## Tests

### FlexBool Tests

| Input | Expected |
|---|---|
| `"true"`, `"t"`, `"yes"`, `"y"`, `"TRUE"`, `"1"` | `true` |
| `"false"`, `"f"`, `"no"`, `"n"`, `"FALSE"`, `"0"` | `false` |
| `""` (NonNil) | error |
| `""` (Nillable) | `EmptyString` |
| `"invalid"` | error |

### Processor Tests

Test each processor variant with valid input, empty input, boundary values, and invalid input. Use table-driven tests.

### Date/Time Tests

| Input | Processor | Valid? |
|---|---|---|
| `"2023-07-24T14:15:02-04:00"` | DateAndTime | Yes |
| `"2023-07-24T14:15:02Z"` | DateAndTime | Yes |
| `"07242023"` (MMDDYYYY) | NonNillMMDDYYYY | Yes |
| `"2099-01-01T00:00:00Z"` | DateAndTime | Error (future) |
| `""` | NonNill* | Error |
| `"not-a-date"` | Any | Error |

### CSVMetadata Tests

| Test | Description |
|---|---|
| MatchHeaders exact match | All required columns present → true |
| MatchHeaders extra columns OK | Extra columns ignored → true |
| MatchHeaders missing column | One required column missing → false |
| ProcessRow success | All processors succeed → complete CSVOutputRow |
| ProcessRow error propagation | First processor fails → error returned |

## Acceptance Criteria

- [ ] All processor variants validate according to documented rules
- [ ] FlexBool accepts all specified truthy/falsy values case-insensitively
- [ ] String max length measured in characters, not bytes
- [ ] All date/time formats accepted
- [ ] `DateAndTimeNonZeroAndNotAfterNow` rejects zero time and future dates
- [ ] MetaID processor validates country/state and calls hasher
- [ ] UniqueID passes raw names to hasher
- [ ] CSVMetadata.MatchHeaders is case-sensitive exact match
- [ ] All tests pass
