package csvgen

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	csvpkg "github.com/corruptmemory/file-uploader-r3/internal/csv"
)

func TestDeterministicOutput(t *testing.T) {
	seed := int64(42)
	for _, ct := range csvpkg.AllCSVTypes() {
		t.Run(ct.Slug(), func(t *testing.T) {
			data1, err := GenerateCSV(ct, 20, WithSeed(seed))
			if err != nil {
				t.Fatalf("first generation failed: %v", err)
			}
			data2, err := GenerateCSV(ct, 20, WithSeed(seed))
			if err != nil {
				t.Fatalf("second generation failed: %v", err)
			}
			if !bytes.Equal(data1, data2) {
				t.Errorf("same seed produced different output for type %s", ct.Slug())
			}
		})
	}
}

func TestAllTypesGenerateWithCorrectHeaders(t *testing.T) {
	for _, ct := range csvpkg.AllCSVTypes() {
		t.Run(ct.Slug(), func(t *testing.T) {
			data, err := GenerateCSV(ct, 5, WithSeed(1))
			if err != nil {
				t.Fatalf("generation failed: %v", err)
			}

			r := csv.NewReader(bytes.NewReader(data))
			headers, err := r.Read()
			if err != nil {
				t.Fatalf("reading headers: %v", err)
			}

			expectedCols, err := InputColumnsForType(ct)
			if err != nil {
				t.Fatalf("getting expected columns: %v", err)
			}

			if len(headers) != len(expectedCols) {
				t.Fatalf("header count mismatch: got %d, want %d", len(headers), len(expectedCols))
			}

			for i, h := range headers {
				if h != expectedCols[i] {
					t.Errorf("header[%d] = %q, want %q", i, h, expectedCols[i])
				}
			}
		})
	}
}

func TestRowCount(t *testing.T) {
	tests := []struct {
		rows int
	}{
		{0},
		{1},
		{50},
		{100},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("rows_%d", tc.rows), func(t *testing.T) {
			data, err := GenerateCSV(csvpkg.CSVPlayers, tc.rows, WithSeed(99))
			if err != nil {
				t.Fatalf("generation failed: %v", err)
			}

			r := csv.NewReader(bytes.NewReader(data))
			// Read header
			_, err = r.Read()
			if err != nil {
				t.Fatalf("reading header: %v", err)
			}

			count := 0
			for {
				_, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("reading row: %v", err)
				}
				count++
			}

			if count != tc.rows {
				t.Errorf("row count = %d, want %d", count, tc.rows)
			}
		})
	}
}

func TestErrorInjectionRate(t *testing.T) {
	// Generate with 50% error rate and check approximate distribution.
	// We use players type since it has well-known required fields.
	seed := int64(123)
	rows := 1000
	data, err := GenerateCSV(csvpkg.CSVPlayers, rows, WithSeed(seed), WithErrorInjection(0.5))
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	r := csv.NewReader(bytes.NewReader(data))
	headers, err := r.Read()
	if err != nil {
		t.Fatalf("reading headers: %v", err)
	}

	// Count rows that look "corrupted" (have empty required fields,
	// non-numeric values in SSN, future dates, etc.)
	errorRows := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading row: %v", err)
		}

		rowMap := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(record) {
				rowMap[h] = record[i]
			}
		}

		if isCorruptedRow(rowMap, headers) {
			errorRows++
		}
	}

	// With 50% error rate and 1000 rows, we expect ~500 errors.
	// Allow wide tolerance for randomness: 350-650.
	errorPct := float64(errorRows) / float64(rows)
	if errorPct < 0.35 || errorPct > 0.65 {
		t.Errorf("error rate = %.2f (errors=%d/%d), expected ~0.50", errorPct, errorRows, rows)
	}
}

func isCorruptedRow(row map[string]string, headers []string) bool {
	for _, col := range headers {
		val := row[col]
		// Check for empty required field (many columns are required)
		if val == "" {
			return true
		}
		// Check for our specific injection marker values
		if val == "not_a_number" {
			return true
		}
		if val == "-999.99" {
			return true
		}
		if len(val) > 1000 {
			return true
		}
		// Check for invalid state codes
		if val == "XX" || val == "ZZ" || val == "QQ" {
			return true
		}
		// Check for future dates injected by error injection (5 years from now)
		futureYear := fmt.Sprintf("%d-", time.Now().AddDate(5, 0, 0).Year())
		if strings.Contains(val, futureYear) {
			return true
		}
	}
	return false
}

func TestNoErrorsWithoutFlag(t *testing.T) {
	// Generate without error injection - all required fields should be filled.
	for _, ct := range csvpkg.AllCSVTypes() {
		t.Run(ct.Slug(), func(t *testing.T) {
			data, err := GenerateCSV(ct, 50, WithSeed(42))
			if err != nil {
				t.Fatalf("generation failed: %v", err)
			}

			r := csv.NewReader(bytes.NewReader(data))
			headers, err := r.Read()
			if err != nil {
				t.Fatalf("reading headers: %v", err)
			}

			rowNum := 0
			for {
				record, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("reading row %d: %v", rowNum, err)
				}
				rowNum++

				// Field count matches header count
				if len(record) != len(headers) {
					t.Errorf("row %d: field count %d != header count %d", rowNum, len(record), len(headers))
				}

				// No injected error markers in any field
				for i, val := range record {
					if val == "not_a_number" {
						t.Errorf("row %d, col %q: found error marker 'not_a_number'", rowNum, headers[i])
					}
					if val == "-999.99" {
						t.Errorf("row %d, col %q: found error marker '-999.99'", rowNum, headers[i])
					}
					if len(val) > 1500 {
						t.Errorf("row %d, col %q: suspiciously long value (%d chars)", rowNum, headers[i], len(val))
					}
					if val == "XX" || val == "ZZ" || val == "QQ" {
						t.Errorf("row %d, col %q: found invalid state code %q", rowNum, headers[i], val)
					}
				}
			}
		})
	}
}

func TestHeaderMatchesSpec(t *testing.T) {
	// Verify that the generated headers match the inputColumns map
	// (which is derived from spec 06).
	for _, ct := range csvpkg.AllCSVTypes() {
		t.Run(ct.Slug(), func(t *testing.T) {
			expectedCols, err := InputColumnsForType(ct)
			if err != nil {
				t.Fatalf("getting expected columns: %v", err)
			}

			data, err := GenerateCSV(ct, 1, WithSeed(1))
			if err != nil {
				t.Fatalf("generation failed: %v", err)
			}

			r := csv.NewReader(bytes.NewReader(data))
			gotHeaders, err := r.Read()
			if err != nil {
				t.Fatalf("reading headers: %v", err)
			}

			if len(gotHeaders) != len(expectedCols) {
				t.Fatalf("header count %d != expected %d", len(gotHeaders), len(expectedCols))
			}

			for i := range gotHeaders {
				if gotHeaders[i] != expectedCols[i] {
					t.Errorf("header[%d]: got %q, want %q", i, gotHeaders[i], expectedCols[i])
				}
			}
		})
	}
}

func TestUnknownType(t *testing.T) {
	_, err := GenerateCSV(csvpkg.CSVType(999), 10)
	if err == nil {
		t.Error("expected error for unknown CSV type, got nil")
	}
}

func TestInputColumnsForType(t *testing.T) {
	for _, ct := range csvpkg.AllCSVTypes() {
		cols, err := InputColumnsForType(ct)
		if err != nil {
			t.Errorf("InputColumnsForType(%s): %v", ct.Slug(), err)
			continue
		}
		if len(cols) == 0 {
			t.Errorf("InputColumnsForType(%s): returned empty columns", ct.Slug())
		}
	}

	_, err := InputColumnsForType(csvpkg.CSVType(999))
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
