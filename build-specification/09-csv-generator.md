# 09 — Synthetic CSV Generator

**Dependencies:** 05-csv-framework.md (CSVType enum and slugs), 06-csv-type-definitions.md (column definitions for all 10 types).

**Produces:** `internal/csvgen/` package and `gen-csv` CLI subcommand.

---

## 1. CLI Subcommand

```
file-uploader gen-csv --type <slug> --rows <N> [--output <path>] [--inject-errors] [--error-rate <0.0-1.0>] [--seed <int>]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--type` | string | required | CSV type slug (e.g., `players`, `bets`, `casino-par-sheet`) |
| `--rows` | int | 100 | Number of data rows |
| `--output` | string | stdout | Output file path |
| `--inject-errors` | bool | false | Enable error injection |
| `--error-rate` | float | 0.1 | Fraction of error rows (0.0-1.0) |
| `--seed` | int64 | random | Random seed for deterministic output |

Register as a go-flags subcommand named `gen-csv`.

---

## 2. Library API

```go
package csvgen

type Option func(*generator)

func WithSeed(seed int64) Option
func WithErrorInjection(rate float64) Option

func GenerateCSV(csvType CSVType, rows int, opts ...Option) ([]byte, error)
```

Package: `internal/csvgen/`

---

## 3. Data Generation Rules

### Common Patterns

| Field Type | Generation Strategy |
|---|---|
| Player IDs | Sequential with prefix: `PLAYER-000001`, `PLAYER-000002`, ... |
| First names | Random from pool of ~50 including diacritics: `José`, `Müller`, `O'Brien`, `María`, `François`, `Björk` |
| Last names | Random from pool of ~50 including hyphens: `Smith-Jones`, `O'Connor`, `García`, `van der Berg` |
| Last4SSN | Random 4-digit string: `0000`–`9999` |
| Dates (past) | Random within last 5 years, formatted per column spec |
| US States | Random from 50 states + DC |
| Country | Always `"US"` |
| Dollar amounts | Random in realistic ranges: bets $1-$10,000, deposits $10-$50,000, bonuses $5-$500 |
| Integer counts | Small positive integers 1-100 |
| Float values | Percentages 0.0-100.0 for par sheet metrics |
| Boolean fields | Random using FlexBool-compatible values (`true`, `false`, `yes`, `no`, `t`, `f`) |
| Nullable fields | ~10% chance of empty value |

### Determinism

When `--seed` is provided, same seed + type + rows produces identical output. Use `math/rand` seeded via `rand.NewSource(seed)`.

### Type-Specific Generation

Each CSV type generates the exact input columns from its definition in spec 06. The header row matches the input columns exactly.

For **Players**: generate LastName, FirstName, Last4SSN, DOB, OrganizationPlayerID, OrganizationCountry, OrganizationState.

For **Bets**: generate all 28+ input columns with realistic values.

For **Casino Par Sheet**: generate Machine_ID, MCH_Casino_ID, MCH_Date (in MMDDYYYY format), and all par metrics.

And so on for all 10 types.

---

## 4. Error Injection

When `--inject-errors` is enabled, randomly introduce errors in the specified fraction of rows. Each error row gets exactly **one** error type:

| Error Type | What It Does |
|---|---|
| Missing required field | Leave a required column empty |
| Invalid type | String where number expected (e.g., `"abc"` in amount) |
| Future date | Date in the future where past-only required |
| Invalid state | Invalid US state code (`"XX"`, `"ZZ"`) |
| Negative amount | Negative number where non-negative required |
| Overlong string | Exceeds max length for string fields |

Error injection does **NOT** corrupt the header row — only data rows.

---

## Tests

| Test | Description |
|------|-------------|
| Deterministic output | Same seed → identical bytes |
| All 10 types generate | Each slug produces valid CSV with correct headers |
| Row count matches | `--rows 50` produces exactly 50 data rows + 1 header |
| Error injection rate | `--inject-errors --error-rate 0.5` → ~50% of rows have errors |
| No errors without flag | Default output has all valid rows |
| Header matches spec | Generated headers match input columns from spec 06 |

### Round-Trip Test

Generate CSV → feed through processor → verify:
- Output has correct output headers
- All PII columns are hashed
- Row count matches input
- MetaID values are consistent (same player → same MetaID)

## Acceptance Criteria

- [ ] All 10 CSV types supported
- [ ] Deterministic output with `--seed`
- [ ] Error injection produces realistic errors at specified rate
- [ ] Library API usable from tests: `csvgen.GenerateCSV(CSVPlayers, 100, WithSeed(42))`
- [ ] CLI subcommand registered and functional
- [ ] Generated CSVs pass through the processor without errors (when no error injection)
- [ ] All tests pass
