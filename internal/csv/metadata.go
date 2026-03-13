package csv

import "fmt"

// CSVMetadata defines the interface for CSV type metadata including column
// matching, header generation, and row processing.
type CSVMetadata interface {
	Type() CSVType
	ColumnData() []InColumnProcessor
	MatchHeaders(headers []string) bool
	ProcessRow(rowdata RowData) (CSVOutputRow, error)
	OutputHeaders() []string
}

// simpleCSVMetadata implements CSVMetadata with a list of processors.
type simpleCSVMetadata struct {
	csvType    CSVType
	processors []InColumnProcessor
}

// NewSimpleCSVMetadata creates a new simpleCSVMetadata with the given type and processors.
func NewSimpleCSVMetadata(csvType CSVType, processors []InColumnProcessor) CSVMetadata {
	return &simpleCSVMetadata{
		csvType:    csvType,
		processors: processors,
	}
}

func (m *simpleCSVMetadata) Type() CSVType {
	return m.csvType
}

func (m *simpleCSVMetadata) ColumnData() []InColumnProcessor {
	return m.processors
}

// MatchHeaders returns true if all required input columns from all processors
// are present in the given headers. Extra columns are ignored.
// Matching is case-sensitive exact match.
func (m *simpleCSVMetadata) MatchHeaders(headers []string) bool {
	headerSet := make(map[string]bool, len(headers))
	for _, h := range headers {
		headerSet[h] = true
	}

	for _, proc := range m.processors {
		for _, col := range proc.InputColumns() {
			if !headerSet[col] {
				return false
			}
		}
	}
	return true
}

// OutputHeaders collects all output column names from all processors in order.
func (m *simpleCSVMetadata) OutputHeaders() []string {
	var headers []string
	for _, proc := range m.processors {
		headers = append(headers, proc.OutputColumns()...)
	}
	return headers
}

// ProcessRow calls each processor's Process in sequence, appending results.
// Stops and returns an error if any processor fails.
func (m *simpleCSVMetadata) ProcessRow(rowdata RowData) (CSVOutputRow, error) {
	var columns []CSVOutputString
	for _, proc := range m.processors {
		results, err := proc.Process(rowdata)
		if err != nil {
			return CSVOutputRow{}, fmt.Errorf("processing %s: %w", m.csvType, err)
		}
		columns = append(columns, results...)
	}
	return CSVOutputRow{Columns: columns}, nil
}
