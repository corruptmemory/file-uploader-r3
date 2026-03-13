package csv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

// CSVType represents the type of CSV file being processed.
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

type csvTypeInfo struct {
	name string
	slug string
}

var csvTypeRegistry = map[CSVType]csvTypeInfo{
	CSVBets:                {"Bets", "bets"},
	CSVPlayers:             {"Players", "players"},
	CSVCasinoPlayers:       {"Casino Players", "casino-players"},
	CSVBonus:               {"Bonus", "bonus"},
	CSVCasino:              {"Casino", "casino"},
	CSVCasinoParSheet:      {"Casino Par Sheet", "casino-par-sheet"},
	CSVComplaints:          {"Complaints", "complaints"},
	CSVDemographic:         {"Demographic", "demographic"},
	CSVDepositsWithdrawals: {"Deposits/Withdrawals", "deposits-withdrawals"},
	CSVResponsibleGaming:   {"Responsible Gaming", "responsible-gaming"},
}

var slugToCSVType map[string]CSVType

func init() {
	slugToCSVType = make(map[string]CSVType, len(csvTypeRegistry))
	for t, info := range csvTypeRegistry {
		slugToCSVType[info.slug] = t
	}
}

// String returns the human-readable name of the CSVType.
func (t CSVType) String() string {
	if info, ok := csvTypeRegistry[t]; ok {
		return info.name
	}
	return fmt.Sprintf("CSVType(%d)", int(t))
}

// Slug returns the URL-safe slug for the CSVType.
func (t CSVType) Slug() string {
	if info, ok := csvTypeRegistry[t]; ok {
		return info.slug
	}
	return fmt.Sprintf("csvtype-%d", int(t))
}

// CSVTypeFromSlug returns the CSVType for the given slug, or an error if not found.
func CSVTypeFromSlug(slug string) (CSVType, error) {
	if t, ok := slugToCSVType[slug]; ok {
		return t, nil
	}
	return 0, fmt.Errorf("unknown CSV type slug: %q", slug)
}

// AllCSVTypes returns all registered CSV types in order.
func AllCSVTypes() []CSVType {
	return []CSVType{
		CSVBets,
		CSVPlayers,
		CSVCasinoPlayers,
		CSVBonus,
		CSVCasino,
		CSVCasinoParSheet,
		CSVComplaints,
		CSVDemographic,
		CSVDepositsWithdrawals,
		CSVResponsibleGaming,
	}
}

// RowData represents a single row of CSV data.
type RowData struct {
	RowIndex int               // 1-based line number (excluding header)
	RowMap   map[string]string // column header -> cell value
}

// CSVOutputString is the interface for output values that can be rendered as CSV.
type CSVOutputString interface {
	String() string
}

// Quoted wraps a string value that should be CSV-escaped.
type Quoted string

// String returns the CSV-escaped representation of the value.
func (q Quoted) String() string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{string(q)})
	w.Flush()
	// csv.Writer adds a trailing newline; trim it
	return strings.TrimRight(buf.String(), "\n")
}

// Raw wraps a string value that should be used as-is.
type Raw string

// String returns the raw string value.
func (r Raw) String() string {
	return string(r)
}

// EmptyString represents an empty/null CSV output.
const EmptyString = Raw("")

// CSVOutputRow represents a complete output row.
type CSVOutputRow struct {
	Columns []CSVOutputString
}

// RowString joins all columns with commas, no trailing newline.
func (r CSVOutputRow) RowString() string {
	parts := make([]string, len(r.Columns))
	for i, col := range r.Columns {
		parts[i] = col.String()
	}
	return strings.Join(parts, ",")
}
