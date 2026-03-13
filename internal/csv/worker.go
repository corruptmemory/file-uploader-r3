package csv

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
	"github.com/dimchansky/utfbom"
)

// DetectFunc is a function type that detects the CSV type from headers and metadata.
// It should return exactly one matching CSVMetadata or an error.
type DetectFunc func(headers []string, allMetadata []CSVMetadata) (CSVMetadata, error)

// BuildMetadataFunc is a function type that builds all CSVMetadata handlers
// from hashers and an operator ID.
type BuildMetadataFunc func(h hashers.Hashers, operatorID CSVOutputString) []CSVMetadata

// Logger is a minimal logging interface for the CSV processor.
type Logger interface {
	Printf(format string, v ...any)
}

// FileMetadata describes an uploaded file to be processed.
type FileMetadata struct {
	ID               string
	UploadedBy       string
	OriginalFilename string
	LocalFilePath    string
	UploadedAt       time.Time
}

// OutFileMetadata extends FileMetadata with processing results.
type OutFileMetadata struct {
	InFile  FileMetadata
	CSVType CSVType
	OutPath string
}

// ProgressRecord tracks row processing progress.
type ProgressRecord struct {
	RowsProcessed int
	TotalRows     int
	Percent       float64
}

// EventSink receives progress events during file processing.
type EventSink interface {
	Starting(file OutFileMetadata)
	Identified(file OutFileMetadata)
	Progress(file OutFileMetadata, record ProgressRecord)
	Success(file OutFileMetadata)
	Failure(file OutFileMetadata, record ProgressRecord, err error)
}

// RowErrors collects errors from processing a single row.
type RowErrors struct {
	RowIndex int
	Errors   []error
}

func (e RowErrors) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("Row[%d]: [%s]", e.RowIndex, strings.Join(msgs, ", "))
}

// workUnit is an internal struct for the processing queue.
type workUnit struct {
	file       FileMetadata
	hashers    hashers.Hashers
	eventSink  EventSink
	operatorID CSVOutputString
}

// workerResult holds the output of processing a single row.
type workerResult struct {
	row CSVOutputRow
	err error
}

// Processor manages a queue-based CSV file processor with a configurable worker pool.
type Processor struct {
	queue            chan workUnit
	workerCount      int
	workingDirectory string
	wg               sync.WaitGroup
	log              Logger
	detectFunc       DetectFunc
	buildMetadata    BuildMetadataFunc
}

// NewProcessor creates a Processor and starts a single goroutine that reads
// from the queue and processes files serially. The detectFunc and buildMeta
// parameters break the import cycle between csv and csv/columnmapping:
// callers pass columnmapping.DetectCSVType and columnmapping.BuildAllMetadata.
func NewProcessor(ctx context.Context, log Logger, queueSize, workerCount int, workingDirectory string, detectFunc DetectFunc, buildMeta BuildMetadataFunc) *Processor {
	p := &Processor{
		queue:            make(chan workUnit, queueSize),
		workerCount:      workerCount,
		workingDirectory: workingDirectory,
		log:              log,
		detectFunc:       detectFunc,
		buildMetadata:    buildMeta,
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for wu := range p.queue {
			p.processFile(ctx, wu)
		}
	}()
	return p
}

// AddWork sends a work unit to the queue. Blocks if queue is full.
func (p *Processor) AddWork(file FileMetadata, h hashers.Hashers, eventSink EventSink, operatorID CSVOutputString) {
	p.queue <- workUnit{
		file:       file,
		hashers:    h,
		eventSink:  eventSink,
		operatorID: operatorID,
	}
}

// Stop closes the queue channel. No more work can be added.
func (p *Processor) Stop() {
	close(p.queue)
}

// Wait blocks until all queued files have been processed.
func (p *Processor) Wait() {
	p.wg.Wait()
}

// processFile handles a single file from the queue.
func (p *Processor) processFile(ctx context.Context, wu workUnit) {
	outMeta := OutFileMetadata{InFile: wu.file}
	progress := ProgressRecord{}

	// Always call SaveDB after processing, regardless of outcome.
	defer func() {
		if err := wu.hashers.SaveDB(); err != nil {
			p.log.Printf("error saving player DB: %v", err)
		}
	}()

	// 1. Emit Starting
	wu.eventSink.Starting(outMeta)

	// 2. Open file with BOM stripping, read headers
	headers, err := p.readHeaders(wu.file.LocalFilePath)
	if err != nil {
		wu.eventSink.Failure(outMeta, progress, fmt.Errorf("reading headers: %w", err))
		return
	}

	// 3. Build all metadata handlers and auto-detect
	allMeta := p.buildMetadata(wu.hashers, wu.operatorID)
	handler, err := p.detectFunc(headers, allMeta)
	if err != nil {
		wu.eventSink.Failure(outMeta, progress, err)
		return
	}

	outMeta.CSVType = handler.Type()

	// 4. Count total data rows (excluding header)
	totalRows, err := countDataRows(wu.file.LocalFilePath)
	if err != nil {
		wu.eventSink.Failure(outMeta, progress, fmt.Errorf("counting rows: %w", err))
		return
	}

	progress.TotalRows = totalRows

	// 5. Create output file
	outFile, err := os.CreateTemp(p.workingDirectory, fmt.Sprintf("out-%s-*.csv", wu.file.ID))
	if err != nil {
		wu.eventSink.Failure(outMeta, progress, fmt.Errorf("creating output file: %w", err))
		return
	}
	outMeta.OutPath = outFile.Name()

	// 6. Write quoted output headers
	outWriter := bufio.NewWriter(outFile)
	headerLine := quoteHeaders(handler.OutputHeaders())
	if _, err := outWriter.WriteString(headerLine + "\n"); err != nil {
		outFile.Close()
		wu.eventSink.Failure(outMeta, progress, fmt.Errorf("writing headers: %w", err))
		return
	}

	// 7. Emit Identified
	wu.eventSink.Identified(outMeta)

	// 8. Set up parallel processing
	procCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	inputCh := make(chan RowData, p.workerCount*3)
	outputCh := make(chan workerResult, p.workerCount*3)

	// Launch workers
	var workerWg sync.WaitGroup
	for i := 0; i < p.workerCount; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for rd := range inputCh {
				select {
				case <-procCtx.Done():
					return
				default:
				}
				row, err := handler.ProcessRow(rd)
				if err != nil {
					outputCh <- workerResult{err: &RowErrors{RowIndex: rd.RowIndex, Errors: []error{err}}}
				} else {
					outputCh <- workerResult{row: row}
				}
			}
		}()
	}

	// Launch feeder goroutine
	go func() {
		err := CSVToChanMaps(procCtx, wu.file.LocalFilePath, headers, inputCh)
		if err != nil && err != context.Canceled {
			p.log.Printf("feeder error: %v", err)
		}
	}()

	// Collector: wait for all workers to finish, then close outputCh
	go func() {
		workerWg.Wait()
		close(outputCh)
	}()

	// 9. Main processing loop: read from outputCh
	rowsProcessed := 0
	var processingErr error
	for result := range outputCh {
		if result.err != nil {
			processingErr = result.err
			cancel() // Cancel workers + feeder
			wu.eventSink.Failure(outMeta, progress, processingErr)
			// Drain remaining output
			for range outputCh {
			}
			break
		}
		line := result.row.RowString() + "\n"
		if _, err := outWriter.WriteString(line); err != nil {
			processingErr = fmt.Errorf("writing output row: %w", err)
			cancel()
			wu.eventSink.Failure(outMeta, progress, processingErr)
			for range outputCh {
			}
			break
		}
		rowsProcessed++
		progress.RowsProcessed = rowsProcessed
		if totalRows > 0 {
			progress.Percent = float64(rowsProcessed) / float64(totalRows) * 100.0
		}
		wu.eventSink.Progress(outMeta, progress)
	}

	if err := outWriter.Flush(); err != nil && processingErr == nil {
		processingErr = fmt.Errorf("flushing output: %w", err)
	}
	outFile.Close()

	if processingErr == nil {
		wu.eventSink.Success(outMeta)
	}
}

// readHeaders opens a file with BOM stripping and reads the first CSV row as headers.
func (p *Processor) readHeaders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(utfbom.SkipOnly(f))
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV headers: %w", err)
	}
	// Trim whitespace from headers
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}
	return headers, nil
}

// countDataRows counts lines in the file excluding the header line.
func countDataRows(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	// Subtract header line
	if count > 0 {
		count--
	}
	return count, nil
}

// quoteHeaders formats output headers as a CSV header row (each quoted).
func quoteHeaders(headers []string) string {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write(headers)
	w.Flush()
	return strings.TrimRight(buf.String(), "\n")
}

// CSVToChanMaps reads a CSV file (with BOM stripping), skips the header row,
// and sends each data row as a RowData on the out channel.
// It closes the out channel when done.
func CSVToChanMaps(ctx context.Context, filePath string, headers []string, out chan<- RowData) error {
	defer close(out)

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := csv.NewReader(utfbom.SkipOnly(f))

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("skipping header: %w", err)
	}

	rowIndex := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading row %d: %w", rowIndex, err)
		}

		rowMap := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(record) {
				rowMap[h] = record[i]
			}
		}

		rd := RowData{
			RowIndex: rowIndex,
			RowMap:   rowMap,
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- rd:
		}

		rowIndex++
	}
}
