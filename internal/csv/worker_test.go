package csv_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/csv/columnmapping"
	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
)

// --- Test helpers ---

// testLogger implements csv.Logger for testing.
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Printf(format string, v ...any) {
	l.t.Logf(format, v...)
}

// testEventSink records events for verification.
type testEventSink struct {
	mu         sync.Mutex
	starting   []csv.OutFileMetadata
	identified []csv.OutFileMetadata
	progress   []csv.ProgressRecord
	successes  []csv.OutFileMetadata
	failures   []struct {
		meta csv.OutFileMetadata
		err  error
	}
}

func (s *testEventSink) Starting(file csv.OutFileMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starting = append(s.starting, file)
}

func (s *testEventSink) Identified(file csv.OutFileMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identified = append(s.identified, file)
}

func (s *testEventSink) Progress(file csv.OutFileMetadata, record csv.ProgressRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = append(s.progress, record)
}

func (s *testEventSink) Success(file csv.OutFileMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.successes = append(s.successes, file)
}

func (s *testEventSink) Failure(file csv.OutFileMetadata, record csv.ProgressRecord, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, struct {
		meta csv.OutFileMetadata
		err  error
	}{file, err})
}

// testHashers implements hashers.Hashers for testing.
type testHashers struct {
	mu          sync.Mutex
	saveDBCalls int
}

func (h *testHashers) PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string {
	return fmt.Sprintf("uid:%s:%s:%s:%s", last4SSN, firstName, lastName, dob)
}

func (h *testHashers) OrganizationPlayerIDHasher(playerID, country, state string) string {
	return fmt.Sprintf("meta:%s:%s:%s", playerID, country, state)
}

func (h *testHashers) SaveDB() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.saveDBCalls++
	return nil
}

func (h *testHashers) getSaveDBCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.saveDBCalls
}

// wrapDetect adapts columnmapping.DetectCSVType to csv.DetectFunc.
func wrapDetect(headers []string, allMetadata []csv.CSVMetadata) (csv.CSVMetadata, error) {
	return columnmapping.DetectCSVType(headers, allMetadata)
}

// wrapBuildMeta adapts columnmapping.BuildAllMetadata to csv.BuildMetadataFunc.
func wrapBuildMeta(h hashers.Hashers, operatorID csv.CSVOutputString) []csv.CSVMetadata {
	return columnmapping.BuildAllMetadata(h, operatorID)
}

// writeCasinoParSheetCSV writes a Casino Par Sheet CSV file (no hashing needed, simplest type).
func writeCasinoParSheetCSV(t *testing.T, dir string, rowCount int) string {
	t.Helper()
	path := filepath.Join(dir, "test-parsheet.csv")
	var buf strings.Builder
	buf.WriteString("Machine_ID,MCH_Casino_ID,MCH_Date,Number_ReelsLinesScatter,Min_Wager,Max_Wager,Symbols_Per_Reel,PaybackPCT,Hit_FrequencyPCT,Plays_Per_Jackpot,Jackpot_Amount,Plays_Per_Bonus,Volatility_Index\n")
	for i := 1; i <= rowCount; i++ {
		buf.WriteString(fmt.Sprintf("MACH%03d,%d,%s,5,1.0,100.0,20,95.5,25.3,10000,5000.0,500,1.5\n",
			i, i, fmt.Sprintf("%02d%02d2023", (i%12)+1, (i%28)+1)))
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("writing test CSV: %v", err)
	}
	return path
}

// writePlayersCSV writes a Players CSV file.
func writePlayersCSV(t *testing.T, dir string, rowCount int) string {
	t.Helper()
	path := filepath.Join(dir, "test-players.csv")
	var buf strings.Builder
	buf.WriteString("LastName,FirstName,Last4SSN,DOB,OrganizationPlayerID,OrganizationCountry,OrganizationState\n")
	for i := 1; i <= rowCount; i++ {
		buf.WriteString(fmt.Sprintf("Smith%d,John%d,%04d,1990-01-15,P%d,US,NY\n",
			i, i, i%10000, i))
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("writing test CSV: %v", err)
	}
	return path
}

// writeBadCSV writes a CSV with headers that match no type.
func writeBadCSV(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test-bad.csv")
	content := "Foo,Bar,Baz\n1,2,3\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test CSV: %v", err)
	}
	return path
}

// writePlayersCSVWithBadRow writes a Players CSV with one bad row.
func writePlayersCSVWithBadRow(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test-players-bad.csv")
	var buf strings.Builder
	buf.WriteString("LastName,FirstName,Last4SSN,DOB,OrganizationPlayerID,OrganizationCountry,OrganizationState\n")
	// Good row
	buf.WriteString("Smith,John,1234,1990-01-15,P1,US,NY\n")
	// Bad row - empty last name
	buf.WriteString(",Jane,5678,1991-02-20,P2,US,CA\n")
	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("writing test CSV: %v", err)
	}
	return path
}

// writeBOMFile writes a file with a UTF-8 BOM prefix.
func writeBOMFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test-bom.csv")
	data := append([]byte{0xEF, 0xBB, 0xBF},
		[]byte("Machine_ID,MCH_Casino_ID,MCH_Date,Number_ReelsLinesScatter,Min_Wager,Max_Wager,Symbols_Per_Reel,PaybackPCT,Hit_FrequencyPCT,Plays_Per_Jackpot,Jackpot_Amount,Plays_Per_Bonus,Volatility_Index\nMACH001,1,01012023,5,1.0,100.0,20,95.5,25.3,10000,5000.0,500,1.5\n")...)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing BOM test CSV: %v", err)
	}
	return path
}

// --- RowErrors Tests ---

func TestRowErrorsFormat(t *testing.T) {
	re := csv.RowErrors{
		RowIndex: 5,
		Errors:   []error{fmt.Errorf("bad value"), fmt.Errorf("missing field")},
	}
	got := re.Error()
	if got != "Row[5]: [bad value, missing field]" {
		t.Errorf("RowErrors.Error() = %q, want %q", got, "Row[5]: [bad value, missing field]")
	}
}

// --- CSVToChanMaps Tests ---

func TestCSVToChanMapsCorrectRowCount(t *testing.T) {
	// Build CSV data without header (CSVToChanMaps expects reader positioned after header)
	var buf strings.Builder
	for i := 1; i <= 10; i++ {
		buf.WriteString(fmt.Sprintf("MACH%03d,%d,%s,5,1.0,100.0,20,95.5,25.3,10000,5000.0,500,1.5\n",
			i, i, fmt.Sprintf("%02d%02d2023", (i%12)+1, (i%28)+1)))
	}
	headers := []string{"Machine_ID", "MCH_Casino_ID", "MCH_Date", "Number_ReelsLinesScatter", "Min_Wager", "Max_Wager", "Symbols_Per_Reel", "PaybackPCT", "Hit_FrequencyPCT", "Plays_Per_Jackpot", "Jackpot_Amount", "Plays_Per_Bonus", "Volatility_Index"}

	out := make(chan csv.RowData, 20)
	ctx := context.Background()

	go func() {
		if err := csv.CSVToChanMaps(ctx, strings.NewReader(buf.String()), headers, out); err != nil {
			t.Errorf("CSVToChanMaps error: %v", err)
		}
	}()

	count := 0
	for range out {
		count++
	}
	if count != 10 {
		t.Errorf("got %d rows, want 10", count)
	}
}

func TestCSVToChanMapsHeaderMapping(t *testing.T) {
	// Data only, no header line (CSVToChanMaps expects reader positioned after header)
	csvData := "Alice,30,NYC\nBob,25,LA\n"

	headers := []string{"Name", "Age", "City"}
	out := make(chan csv.RowData, 10)
	ctx := context.Background()

	go func() {
		if err := csv.CSVToChanMaps(ctx, strings.NewReader(csvData), headers, out); err != nil {
			t.Errorf("CSVToChanMaps error: %v", err)
		}
	}()

	rows := make([]csv.RowData, 0)
	for rd := range out {
		rows = append(rows, rd)
	}

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	if rows[0].RowMap["Name"] != "Alice" {
		t.Errorf("row 0 Name = %q, want Alice", rows[0].RowMap["Name"])
	}
	if rows[0].RowMap["Age"] != "30" {
		t.Errorf("row 0 Age = %q, want 30", rows[0].RowMap["Age"])
	}
	if rows[0].RowMap["City"] != "NYC" {
		t.Errorf("row 0 City = %q, want NYC", rows[0].RowMap["City"])
	}
	if rows[0].RowIndex != 1 {
		t.Errorf("row 0 RowIndex = %d, want 1", rows[0].RowIndex)
	}
	if rows[1].RowIndex != 2 {
		t.Errorf("row 1 RowIndex = %d, want 2", rows[1].RowIndex)
	}
}

func TestCSVToChanMapsContextCancellation(t *testing.T) {
	// Build 1000 rows of data without header
	var buf strings.Builder
	for i := 1; i <= 1000; i++ {
		buf.WriteString(fmt.Sprintf("MACH%03d,%d,%s,5,1.0,100.0,20,95.5,25.3,10000,5000.0,500,1.5\n",
			i, i, fmt.Sprintf("%02d%02d2023", (i%12)+1, (i%28)+1)))
	}
	headers := []string{"Machine_ID", "MCH_Casino_ID", "MCH_Date", "Number_ReelsLinesScatter", "Min_Wager", "Max_Wager", "Symbols_Per_Reel", "PaybackPCT", "Hit_FrequencyPCT", "Plays_Per_Jackpot", "Jackpot_Amount", "Plays_Per_Bonus", "Volatility_Index"}

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan csv.RowData, 5)

	var feedErr error
	done := make(chan struct{})
	go func() {
		feedErr = csv.CSVToChanMaps(ctx, strings.NewReader(buf.String()), headers, out)
		done <- struct{}{}
	}()

	// Read a few rows then cancel
	count := 0
	for range out {
		count++
		if count >= 3 {
			cancel()
			break
		}
	}
	// Drain remaining
	for range out {
	}
	<-done

	// feedErr should be context.Canceled (or nil if all rows processed before cancel)
	if feedErr != nil && feedErr != context.Canceled {
		t.Errorf("expected context.Canceled or nil, got %v", feedErr)
	}
}

// --- Worker Pool Tests ---

func TestSingleFileProcessesCompletely(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writeCasinoParSheetCSV(t, dir, 5)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-1",
		OriginalFilename: "parsheet.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.starting) != 1 {
		t.Errorf("expected 1 Starting event, got %d", len(sink.starting))
	}
	if len(sink.identified) != 1 {
		t.Errorf("expected 1 Identified event, got %d", len(sink.identified))
	}
	if len(sink.successes) != 1 {
		if len(sink.failures) > 0 {
			for _, f := range sink.failures {
				t.Logf("failure: %v", f.err)
			}
		}
		t.Fatalf("expected 1 Success event, got %d (failures: %d)", len(sink.successes), len(sink.failures))
	}
	if len(sink.failures) != 0 {
		for _, f := range sink.failures {
			t.Errorf("unexpected failure: %v", f.err)
		}
	}

	// Verify output file exists and has correct row count
	outPath := sink.successes[0].OutPath
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// 1 header + 5 data rows = 6 lines
	if len(lines) != 6 {
		t.Errorf("output has %d lines, want 6", len(lines))
	}

	// Check CSVType is Casino Par Sheet
	if sink.identified[0].CSVType != csv.CSVCasinoParSheet {
		t.Errorf("CSVType = %v, want CSVCasinoParSheet", sink.identified[0].CSVType)
	}
}

func TestRowErrorAbortsFile(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writePlayersCSVWithBadRow(t, dir)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-bad",
		OriginalFilename: "players-bad.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.failures) == 0 {
		t.Fatal("expected at least 1 Failure event")
	}
	if len(sink.successes) != 0 {
		t.Error("expected no Success events")
	}
}

func TestOutputHasHashedIdentifiers(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writePlayersCSV(t, dir, 3)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-hash",
		OriginalFilename: "players.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.successes) != 1 {
		if len(sink.failures) > 0 {
			t.Fatalf("expected success but got failure: %v", sink.failures[0].err)
		}
		t.Fatal("expected 1 Success event")
	}

	outPath := sink.successes[0].OutPath
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	output := string(data)

	// Raw player IDs (P1, P2, P3) should not appear; hashed versions should
	for i := 1; i <= 3; i++ {
		rawID := fmt.Sprintf(",P%d,", i)
		if strings.Contains(output, rawID) {
			t.Errorf("output contains raw player ID %q", rawID)
		}
	}

	// Hashed MetaIDs should be present (our test hasher produces "meta:...")
	if !strings.Contains(output, "meta:") {
		t.Error("output does not contain hashed MetaIDs")
	}
}

func TestSaveDBCalledAfterFile(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writeCasinoParSheetCSV(t, dir, 2)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-savedb",
		OriginalFilename: "parsheet.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	if h.getSaveDBCalls() != 1 {
		t.Errorf("SaveDB called %d times, want 1", h.getSaveDBCalls())
	}
}

func TestSaveDBCalledOnFailure(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writeBadCSV(t, dir)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-fail-savedb",
		OriginalFilename: "bad.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	if h.getSaveDBCalls() != 1 {
		t.Errorf("SaveDB called %d times on failure, want 1", h.getSaveDBCalls())
	}
}

func TestMultipleFilesQueued(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	for i := 0; i < 3; i++ {
		subDir := filepath.Join(dir, fmt.Sprintf("f%d", i))
		os.MkdirAll(subDir, 0755)
		csvPath := writeCasinoParSheetCSV(t, subDir, 3)

		proc.AddWork(csv.FileMetadata{
			ID:               fmt.Sprintf("file-%d", i),
			OriginalFilename: fmt.Sprintf("parsheet-%d.csv", i),
			LocalFilePath:    csvPath,
			UploadedAt:       time.Now(),
		}, h, sink, csv.Quoted("OP1"))
	}

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.starting) != 3 {
		t.Errorf("expected 3 Starting events, got %d", len(sink.starting))
	}
	if len(sink.successes) != 3 {
		t.Errorf("expected 3 Success events, got %d (failures: %d)", len(sink.successes), len(sink.failures))
	}
	if h.getSaveDBCalls() != 3 {
		t.Errorf("SaveDB called %d times, want 3", h.getSaveDBCalls())
	}
}

func TestNoMatchHeadersFailure(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writeBadCSV(t, dir)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-nomatch",
		OriginalFilename: "bad.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.failures) == 0 {
		t.Error("expected Failure event for no match")
	}
	if len(sink.successes) != 0 {
		t.Error("expected no Success events")
	}
}

func TestFeederReopenFailureEmitsFailure(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a valid Casino Par Sheet CSV
	realPath := writeCasinoParSheetCSV(t, dir, 3)
	// Create a symlink — processor will read through the symlink
	linkPath := filepath.Join(dir, "link.csv")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	// Custom detect func that removes the real file after detection succeeds,
	// so the feeder's re-open of the symlink will fail.
	deletingDetect := func(headers []string, allMetadata []csv.CSVMetadata) (csv.CSVMetadata, error) {
		meta, err := columnmapping.DetectCSVType(headers, allMetadata)
		if err != nil {
			return nil, err
		}
		// Remove real file — the symlink now points to nothing
		os.Remove(realPath)
		return meta, nil
	}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, deletingDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-feeder-fail",
		OriginalFilename: "link.csv",
		LocalFilePath:    linkPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.successes) != 0 {
		t.Error("expected no Success events when feeder cannot re-open file")
	}
	if len(sink.failures) == 0 {
		t.Error("expected Failure event when feeder cannot re-open file")
	} else if !strings.Contains(sink.failures[0].err.Error(), "feeder open error") {
		t.Errorf("expected feeder open error, got: %v", sink.failures[0].err)
	}
}

func TestBOMStripping(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	csvPath := writeBOMFile(t, dir)
	sink := &testEventSink{}
	h := &testHashers{}
	log := &testLogger{t: t}

	ctx := context.Background()
	proc := csv.NewProcessor(ctx, log, 10, 2, workDir, wrapDetect, wrapBuildMeta)

	proc.AddWork(csv.FileMetadata{
		ID:               "test-bom",
		OriginalFilename: "bom.csv",
		LocalFilePath:    csvPath,
		UploadedAt:       time.Now(),
	}, h, sink, csv.Quoted("OP1"))

	proc.Stop()
	proc.Wait()

	sink.mu.Lock()
	defer sink.mu.Unlock()

	if len(sink.successes) != 1 {
		if len(sink.failures) > 0 {
			t.Fatalf("expected success but got failure: %v", sink.failures[0].err)
		}
		t.Fatal("expected 1 Success event for BOM file")
	}
}

// --- Auto-Detection Tests (via DetectCSVType) ---

func TestDetectValidPlayersHeaders(t *testing.T) {
	h := &testHashers{}
	allMeta := columnmapping.BuildAllMetadata(h, csv.Quoted("OP1"))

	headers := []string{"LastName", "FirstName", "Last4SSN", "DOB", "OrganizationPlayerID", "OrganizationCountry", "OrganizationState"}
	meta, err := columnmapping.DetectCSVType(headers, allMeta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Type() != csv.CSVPlayers {
		t.Errorf("detected %v, want CSVPlayers", meta.Type())
	}
}

func TestDetectExtraColumnsStillMatch(t *testing.T) {
	h := &testHashers{}
	allMeta := columnmapping.BuildAllMetadata(h, csv.Quoted("OP1"))

	headers := []string{"LastName", "FirstName", "Last4SSN", "DOB", "OrganizationPlayerID", "OrganizationCountry", "OrganizationState", "ExtraColumn"}
	meta, err := columnmapping.DetectCSVType(headers, allMeta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Type() != csv.CSVPlayers {
		t.Errorf("detected %v, want CSVPlayers", meta.Type())
	}
}

func TestDetectMissingColumnNoMatch(t *testing.T) {
	h := &testHashers{}
	allMeta := columnmapping.BuildAllMetadata(h, csv.Quoted("OP1"))

	// Missing DOB column
	headers := []string{"LastName", "FirstName", "Last4SSN", "OrganizationPlayerID", "OrganizationCountry", "OrganizationState"}
	_, err := columnmapping.DetectCSVType(headers, allMeta)
	if err == nil {
		t.Error("expected error for missing required column")
	}
}

func TestDetectNoMatchClearError(t *testing.T) {
	h := &testHashers{}
	allMeta := columnmapping.BuildAllMetadata(h, csv.Quoted("OP1"))

	headers := []string{"Foo", "Bar", "Baz"}
	_, err := columnmapping.DetectCSVType(headers, allMeta)
	if err == nil {
		t.Error("expected error for no match")
	}
	if !strings.Contains(err.Error(), "no CSV type match") {
		t.Errorf("error should mention no match: %v", err)
	}
}
