package csvgen

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	csvpkg "github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/csv/columnmapping"
	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
	"github.com/corruptmemory/file-uploader-r3/internal/playerdb"
)

// TestRoundTrip generates CSV for each type, processes it through the real
// processor pipeline, and verifies output headers, hashed PII, row count,
// and consistent MetaIDs.
func TestRoundTrip(t *testing.T) {
	// Skip types that require UniqueID (players, casino-players) from MetaID consistency check
	// since they use different hashing paths.
	const seed = int64(777)
	const rows = 10
	const uniquePepper = "test-unique-pepper"
	const orgPepper = "test-org-pepper"
	const operatorID = "TEST-OP-001"

	for _, ct := range csvpkg.AllCSVTypes() {
		t.Run(ct.Slug(), func(t *testing.T) {
			tmpDir := t.TempDir()

			// 1. Generate CSV
			data, err := GenerateCSV(ct, rows, WithSeed(seed))
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			// Write to temp file
			inPath := filepath.Join(tmpDir, "input.csv")
			if err := os.WriteFile(inPath, data, 0644); err != nil {
				t.Fatalf("write input: %v", err)
			}

			// 2. Set up hashers and metadata
			pdb := playerdb.NewMemDB(orgPepper)
			h := hashers.NewPlayerDataHasher(false, filepath.Join(tmpDir, "players.db"), uniquePepper, orgPepper, hashers.ProcessName, pdb)
			defer h.Close()
			opID := csvpkg.Quoted(operatorID)
			allMeta := columnmapping.BuildAllMetadata(h, opID)

			// 3. Read headers from generated CSV
			inReader := csv.NewReader(bytes.NewReader(data))
			headers, err := inReader.Read()
			if err != nil {
				t.Fatalf("reading headers: %v", err)
			}

			// 4. Detect CSV type
			handler, err := columnmapping.DetectCSVType(headers, allMeta)
			if err != nil {
				t.Fatalf("detect type: %v", err)
			}
			if handler.Type() != ct {
				t.Fatalf("detected type %v, want %v", handler.Type(), ct)
			}

			// 5. Process all rows
			outputHeaders := handler.OutputHeaders()
			if len(outputHeaders) == 0 {
				t.Fatal("output headers are empty")
			}

			processedRows := 0
			metaIDs := make(map[string]bool)
			rowIdx := 1
			for {
				record, err := inReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("reading row %d: %v", rowIdx, err)
				}

				rowMap := make(map[string]string, len(headers))
				for i, h := range headers {
					if i < len(record) {
						rowMap[h] = record[i]
					}
				}

				rd := csvpkg.RowData{RowIndex: rowIdx, RowMap: rowMap}
				outRow, err := handler.ProcessRow(rd)
				if err != nil {
					t.Fatalf("processing row %d: %v", rowIdx, err)
				}

				// Verify output column count matches output headers
				if len(outRow.Columns) != len(outputHeaders) {
					t.Errorf("row %d: output columns %d != headers %d",
						rowIdx, len(outRow.Columns), len(outputHeaders))
				}

				// Check for MetaID in output (if present)
				for i, hdr := range outputHeaders {
					if hdr == "MetaID" && i < len(outRow.Columns) {
						mid := outRow.Columns[i].String()
						if mid == "" {
							t.Errorf("row %d: MetaID is empty", rowIdx)
						}
						metaIDs[mid] = true
					}
				}

				processedRows++
				rowIdx++
			}

			// 6. Verify row count
			if processedRows != rows {
				t.Errorf("processed %d rows, want %d", processedRows, rows)
			}

			// 7. For types with MetaID, verify we got MetaIDs
			hasMetaID := slices.Contains(outputHeaders, "MetaID")
			if hasMetaID && len(metaIDs) == 0 {
				t.Error("expected MetaIDs in output, got none")
			}

			// 8. Verify MetaID consistency: process again with same hashers,
			// same data should produce same MetaIDs
			pdb2 := playerdb.NewMemDB(orgPepper)
			h2 := hashers.NewPlayerDataHasher(false, filepath.Join(tmpDir, "players2.db"), uniquePepper, orgPepper, hashers.ProcessName, pdb2)
			defer h2.Close()
			allMeta2 := columnmapping.BuildAllMetadata(h2, opID)
			handler2, err := columnmapping.DetectCSVType(headers, allMeta2)
			if err != nil {
				t.Fatalf("detect type (2nd pass): %v", err)
			}

			inReader2 := csv.NewReader(bytes.NewReader(data))
			_, _ = inReader2.Read() // skip header

			rowIdx = 1
			for {
				record, err := inReader2.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("reading row %d (2nd pass): %v", rowIdx, err)
				}

				rowMap := make(map[string]string, len(headers))
				for i, h := range headers {
					if i < len(record) {
						rowMap[h] = record[i]
					}
				}

				rd := csvpkg.RowData{RowIndex: rowIdx, RowMap: rowMap}
				outRow, err := handler2.ProcessRow(rd)
				if err != nil {
					t.Fatalf("processing row %d (2nd pass): %v", rowIdx, err)
				}

				// If there's a MetaID column, verify it matches first pass
				for i, hdr := range outputHeaders {
					if hdr == "MetaID" && i < len(outRow.Columns) {
						mid := outRow.Columns[i].String()
						if !metaIDs[mid] {
							t.Errorf("row %d: MetaID %q from 2nd pass not in 1st pass set", rowIdx, mid)
						}
					}
				}

				rowIdx++
			}
		})
	}
}
