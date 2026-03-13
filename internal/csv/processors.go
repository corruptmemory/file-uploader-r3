package csv

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
)

// InColumnProcessor defines how to transform input CSV columns into output columns.
type InColumnProcessor interface {
	InputColumns() []string
	OutputColumns() []string
	OutputUniqueID() bool
	OutputMetaID() bool
	Process(rowdata RowData) ([]CSVOutputString, error)
}

// --- simpleInColumnProcessor ---

type simpleInColumnProcessor struct {
	inColumn    string
	outColumn   string
	processFunc func(in string) (CSVOutputString, error)
}

func (p *simpleInColumnProcessor) InputColumns() []string  { return []string{p.inColumn} }
func (p *simpleInColumnProcessor) OutputColumns() []string { return []string{p.outColumn} }
func (p *simpleInColumnProcessor) OutputUniqueID() bool    { return false }
func (p *simpleInColumnProcessor) OutputMetaID() bool      { return false }

func (p *simpleInColumnProcessor) Process(rowdata RowData) ([]CSVOutputString, error) {
	val := rowdata.RowMap[p.inColumn]
	result, err := p.processFunc(val)
	if err != nil {
		return nil, fmt.Errorf("row %d, column %q: %w", rowdata.RowIndex, p.inColumn, err)
	}
	return []CSVOutputString{result}, nil
}

// --- Boolean Processors ---

var truthyValues = map[string]bool{
	"true": true, "t": true, "yes": true, "y": true, "1": true,
}

var falsyValues = map[string]bool{
	"false": true, "f": true, "no": true, "n": true, "0": true,
}

func parseFlexBool(in string) (bool, error) {
	lower := strings.ToLower(strings.TrimSpace(in))
	if truthyValues[lower] {
		return true, nil
	}
	if falsyValues[lower] {
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean value")
}

// NonNilFlexBool returns a processor that parses flexible boolean values.
// Empty/whitespace input is an error.
func NonNilFlexBool(inColumn, outColumn string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  inColumn,
		outColumn: outColumn,
		processFunc: func(in string) (CSVOutputString, error) {
			if strings.TrimSpace(in) == "" {
				return nil, fmt.Errorf("value is required")
			}
			b, err := parseFlexBool(in)
			if err != nil {
				return nil, err
			}
			return Quoted(strconv.FormatBool(b)), nil
		},
	}
}

// NillableFlexBool returns a processor that parses flexible boolean values.
// Empty input returns EmptyString.
func NillableFlexBool(inColumn, outColumn string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  inColumn,
		outColumn: outColumn,
		processFunc: func(in string) (CSVOutputString, error) {
			if strings.TrimSpace(in) == "" {
				return EmptyString, nil
			}
			b, err := parseFlexBool(in)
			if err != nil {
				return nil, err
			}
			return Quoted(strconv.FormatBool(b)), nil
		},
	}
}

// --- Integer Processors ---

// NonNillableInt returns a processor for non-nullable integer values.
func NonNillableInt(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value")
			}
			return Raw(strconv.Itoa(v)), nil
		},
	}
}

// NonNillableNonNegInt returns a processor for non-nullable non-negative integer values.
func NonNillableNonNegInt(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value")
			}
			if v < 0 {
				return nil, fmt.Errorf("value must be non-negative, got %d", v)
			}
			return Raw(strconv.Itoa(v)), nil
		},
	}
}

// NillableInt returns a processor for nullable integer values.
func NillableInt(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return EmptyString, nil
			}
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value")
			}
			return Raw(strconv.Itoa(v)), nil
		},
	}
}

// NillableNonNegInt returns a processor for nullable non-negative integer values.
func NillableNonNegInt(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return EmptyString, nil
			}
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value")
			}
			if v < 0 {
				return nil, fmt.Errorf("value must be non-negative, got %d", v)
			}
			return Raw(strconv.Itoa(v)), nil
		},
	}
}

// --- Float Processors ---

// NonNilFloat64Full returns a processor for non-nullable float64 values.
func NonNilFloat64Full(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value")
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("NaN and Inf are not valid float values")
			}
			return Raw(fmt.Sprintf("%g", v)), nil
		},
	}
}

// NonNilNonNegFloat64Full returns a processor for non-nullable non-negative float64 values.
func NonNilNonNegFloat64Full(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value")
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("NaN and Inf are not valid float values")
			}
			if v < 0 {
				return nil, fmt.Errorf("value must be non-negative")
			}
			return Raw(fmt.Sprintf("%g", v)), nil
		},
	}
}

// NillableFloat64Full returns a processor for nullable float64 values.
func NillableFloat64Full(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return EmptyString, nil
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value")
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("NaN and Inf are not valid float values")
			}
			return Raw(fmt.Sprintf("%g", v)), nil
		},
	}
}

// NillableNonNegFloat64Full returns a processor for nullable non-negative float64 values.
func NillableNonNegFloat64Full(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return EmptyString, nil
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value")
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("NaN and Inf are not valid float values")
			}
			if v < 0 {
				return nil, fmt.Errorf("value must be non-negative")
			}
			return Raw(fmt.Sprintf("%g", v)), nil
		},
	}
}

// --- String Processors ---

// NonEmptyStringWithMax returns a processor for non-empty strings with a max character length.
func NonEmptyStringWithMax(in, out string, maxLen uint) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("value is required")
			}
			if uint(utf8.RuneCountInString(s)) > maxLen {
				return nil, fmt.Errorf("value exceeds max length of %d characters", maxLen)
			}
			return Quoted(s), nil
		},
	}
}

// NillableStringWithMax returns a processor for nullable strings with a max character length.
func NillableStringWithMax(in, out string, maxLen uint) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return EmptyString, nil
			}
			if uint(utf8.RuneCountInString(s)) > maxLen {
				return nil, fmt.Errorf("value exceeds max length of %d characters", maxLen)
			}
			return Quoted(s), nil
		},
	}
}

// --- Date/Time Processors ---

var dateTimeFormats = []string{
	"2006-01-02 15:04",
	"2006-1-2 15:04",
	"1/2/06 15:04",
	"01/02/06 15:04",
	"1/2/2006 15:04",
	"01/02/2006 15:04",
	time.RFC3339,
}

var dateOnlyFormats = []string{
	"2006-01-02",
	"01/02/2006",
	"1/2/2006",
	"01/02/06",
	"1/2/06",
	"20060102",
}

func parseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, format := range dateTimeFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse datetime value")
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, format := range dateOnlyFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date value")
}

// DateAndTimeNonZeroAndNotAfterNow returns a processor for non-nil datetimes.
// Rejects zero time and future dates. Output is RFC3339.
func DateAndTimeNonZeroAndNotAfterNow(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("value is required")
			}
			t, err := parseDateTime(s)
			if err != nil {
				return nil, err
			}
			if t.IsZero() {
				return nil, fmt.Errorf("datetime must not be zero")
			}
			if t.After(time.Now()) {
				return nil, fmt.Errorf("datetime must not be in the future")
			}
			return Quoted(t.Format(time.RFC3339)), nil
		},
	}
}

// NillableDateAndTimeNotAfterNow returns a processor for nullable datetimes.
// Empty input returns EmptyString. Rejects future dates.
func NillableDateAndTimeNotAfterNow(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return EmptyString, nil
			}
			t, err := parseDateTime(s)
			if err != nil {
				return nil, err
			}
			if t.IsZero() {
				return nil, fmt.Errorf("datetime must not be zero")
			}
			if t.After(time.Now()) {
				return nil, fmt.Errorf("datetime must not be in the future")
			}
			return Quoted(t.Format(time.RFC3339)), nil
		},
	}
}

// NonNillDate returns a processor for non-nil date-only values. Output is ISO 8601 date.
func NonNillDate(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("value is required")
			}
			t, err := parseDate(s)
			if err != nil {
				return nil, err
			}
			return Quoted(t.Format("2006-01-02")), nil
		},
	}
}

// NonNillMMDDYYYY returns a processor for strict 8-digit MMDDYYYY dates.
func NonNillMMDDYYYY(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			if len(s) != 8 {
				return nil, fmt.Errorf("expected 8-digit MMDDYYYY value")
			}
			t, err := time.Parse("01022006", s)
			if err != nil {
				return nil, fmt.Errorf("invalid MMDDYYYY date value")
			}
			return Quoted(t.Format("2006-01-02")), nil
		},
	}
}

// NonNilBirthYear returns a processor for non-nil dates, extracting the 4-digit year.
func NonNilBirthYear(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("value is required")
			}
			t, err := parseDate(s)
			if err != nil {
				return nil, err
			}
			return Quoted(t.Format("2006")), nil
		},
	}
}

// NonNillableHHMMSSTime returns a processor for strict 6-digit HHMMSS times.
func NonNillableHHMMSSTime(in, out string) InColumnProcessor {
	return &simpleInColumnProcessor{
		inColumn:  in,
		outColumn: out,
		processFunc: func(s string) (CSVOutputString, error) {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("value is required")
			}
			if len(s) != 6 {
				return nil, fmt.Errorf("expected 6-digit HHMMSS value")
			}
			t, err := time.Parse("150405", s)
			if err != nil {
				return nil, fmt.Errorf("invalid HHMMSS time value")
			}
			return Quoted(t.Format("15:04:05")), nil
		},
	}
}

// --- ConstOutColumnProcessor ---

type constOutColumnProcessor struct {
	outColumn string
	value     CSVOutputString
}

func (p *constOutColumnProcessor) InputColumns() []string  { return nil }
func (p *constOutColumnProcessor) OutputColumns() []string { return []string{p.outColumn} }
func (p *constOutColumnProcessor) OutputUniqueID() bool    { return false }
func (p *constOutColumnProcessor) OutputMetaID() bool      { return false }
func (p *constOutColumnProcessor) Process(RowData) ([]CSVOutputString, error) {
	return []CSVOutputString{p.value}, nil
}

// ConstantString returns a processor that outputs a constant value for every row.
func ConstantString(outColumn string, value CSVOutputString) InColumnProcessor {
	return &constOutColumnProcessor{outColumn: outColumn, value: value}
}

// --- MetaIDColumnProcessor ---

type metaIDColumnProcessor struct {
	inPlayerID string
	outMetaID  string
	inCountry  string
	inState    string
	outCountry string
	outState   string
	hasher     func(playerID, country, state string) string
}

func (p *metaIDColumnProcessor) InputColumns() []string {
	return []string{p.inPlayerID, p.inCountry, p.inState}
}

func (p *metaIDColumnProcessor) OutputColumns() []string {
	cols := []string{p.outMetaID}
	if p.outCountry != "" {
		cols = append(cols, p.outCountry)
	}
	if p.outState != "" {
		cols = append(cols, p.outState)
	}
	return cols
}

func (p *metaIDColumnProcessor) OutputUniqueID() bool { return false }
func (p *metaIDColumnProcessor) OutputMetaID() bool   { return true }

func (p *metaIDColumnProcessor) Process(rowdata RowData) ([]CSVOutputString, error) {
	playerID := strings.TrimSpace(rowdata.RowMap[p.inPlayerID])
	if playerID == "" {
		return nil, fmt.Errorf("row %d: player ID column %q is empty", rowdata.RowIndex, p.inPlayerID)
	}

	country := strings.TrimSpace(rowdata.RowMap[p.inCountry])
	if country == "" {
		return nil, fmt.Errorf("row %d: country column %q is empty", rowdata.RowIndex, p.inCountry)
	}
	country = strings.ToUpper(country)

	state := strings.TrimSpace(rowdata.RowMap[p.inState])
	state = strings.ToUpper(state)

	subdivisions, err := hashers.GetCountrySubdivisions(country)
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowdata.RowIndex, err)
	}

	if len(subdivisions) > 0 {
		if state == "" {
			return nil, fmt.Errorf("row %d: state is required for country %q", rowdata.RowIndex, country)
		}
		if !slices.Contains(subdivisions, state) {
			return nil, fmt.Errorf("row %d: invalid state %q for country %q", rowdata.RowIndex, state, country)
		}
	}

	metaID := p.hasher(playerID, country, state)

	result := []CSVOutputString{Raw(metaID)}
	if p.outCountry != "" {
		result = append(result, Quoted(country))
	}
	if p.outState != "" {
		result = append(result, Quoted(state))
	}
	return result, nil
}

// MetaID creates a MetaID column processor with fully specified column names.
func MetaID(inPlayerID, outMetaID, inCountry, inState, outCountry, outState string,
	hasher func(playerID, country, state string) string) InColumnProcessor {
	return &metaIDColumnProcessor{
		inPlayerID: inPlayerID,
		outMetaID:  outMetaID,
		inCountry:  inCountry,
		inState:    inState,
		outCountry: outCountry,
		outState:   outState,
		hasher:     hasher,
	}
}

// MetaIDDefault creates a MetaID processor with default column names.
func MetaIDDefault(hasher func(playerID, country, state string) string) InColumnProcessor {
	return MetaID("PlayerID", "MetaID", "Country", "State", "Country", "State", hasher)
}

// MetaIDDefaultIn creates a MetaID processor with default input columns but custom output columns.
func MetaIDDefaultIn(outMetaID, outCountry, outState string,
	hasher func(playerID, country, state string) string) InColumnProcessor {
	return MetaID("PlayerID", outMetaID, "Country", "State", outCountry, outState, hasher)
}

// MetaIDDefaultOut creates a MetaID processor with custom input columns but default output columns.
func MetaIDDefaultOut(inPlayerID, inCountry, inState string,
	hasher func(playerID, country, state string) string) InColumnProcessor {
	return MetaID(inPlayerID, "MetaID", inCountry, inState, "Country", "State", hasher)
}

// --- MetaIDFixedLocationColumnProcessor ---

type metaIDFixedLocationProcessor struct {
	inPlayerID string
	outMetaID  string
	country    string
	state      string
	hasher     func(playerID, country, state string) string
}

func (p *metaIDFixedLocationProcessor) InputColumns() []string  { return []string{p.inPlayerID} }
func (p *metaIDFixedLocationProcessor) OutputColumns() []string { return []string{p.outMetaID} }
func (p *metaIDFixedLocationProcessor) OutputUniqueID() bool    { return false }
func (p *metaIDFixedLocationProcessor) OutputMetaID() bool      { return true }

func (p *metaIDFixedLocationProcessor) Process(rowdata RowData) ([]CSVOutputString, error) {
	playerID := strings.TrimSpace(rowdata.RowMap[p.inPlayerID])
	if playerID == "" {
		return nil, fmt.Errorf("row %d: player ID column %q is empty", rowdata.RowIndex, p.inPlayerID)
	}
	metaID := p.hasher(playerID, p.country, p.state)
	return []CSVOutputString{Raw(metaID)}, nil
}

// MetaIDFixedLocation creates a MetaID processor with hardcoded country and state.
// Country and state are validated at construction time.
func MetaIDFixedLocation(inPlayerID, outMetaID, country, state string,
	hasher func(playerID, country, state string) string) (InColumnProcessor, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	state = strings.ToUpper(strings.TrimSpace(state))

	subdivisions, err := hashers.GetCountrySubdivisions(country)
	if err != nil {
		return nil, fmt.Errorf("invalid country %q: %w", country, err)
	}

	if len(subdivisions) > 0 {
		if state == "" {
			return nil, fmt.Errorf("state is required for country %q", country)
		}
		if !slices.Contains(subdivisions, state) {
			return nil, fmt.Errorf("invalid state %q for country %q", state, country)
		}
	}

	return &metaIDFixedLocationProcessor{
		inPlayerID: inPlayerID,
		outMetaID:  outMetaID,
		country:    country,
		state:      state,
		hasher:     hasher,
	}, nil
}

// MustMetaIDFixedLocation creates a MetaID processor with hardcoded location, panicking on error.
func MustMetaIDFixedLocation(inPlayerID, outMetaID, country, state string,
	hasher func(playerID, country, state string) string) InColumnProcessor {
	p, err := MetaIDFixedLocation(inPlayerID, outMetaID, country, state, hasher)
	if err != nil {
		panic(err)
	}
	return p
}

// --- UniqueIDProcessor ---

type uniqueIDProcessor struct {
	inLastName        string
	inFirstName       string
	inLast4SSN        string
	fallbackLast4SSN  string
	inDOB             string
	outUniquePlayerID string
	hasher            func(lastName, firstName, last4SSN, dob string) string
}

func (p *uniqueIDProcessor) InputColumns() []string {
	return []string{p.inLastName, p.inFirstName, p.inLast4SSN, p.inDOB}
}

func (p *uniqueIDProcessor) OutputColumns() []string {
	return []string{p.outUniquePlayerID}
}

func (p *uniqueIDProcessor) OutputUniqueID() bool { return true }
func (p *uniqueIDProcessor) OutputMetaID() bool   { return false }

func (p *uniqueIDProcessor) Process(rowdata RowData) ([]CSVOutputString, error) {
	lastName := strings.TrimSpace(rowdata.RowMap[p.inLastName])
	if lastName == "" {
		return nil, fmt.Errorf("row %d: last name column %q is empty", rowdata.RowIndex, p.inLastName)
	}

	firstName := strings.TrimSpace(rowdata.RowMap[p.inFirstName])
	if firstName == "" {
		return nil, fmt.Errorf("row %d: first name column %q is empty", rowdata.RowIndex, p.inFirstName)
	}

	last4SSN := strings.TrimSpace(rowdata.RowMap[p.inLast4SSN])
	usedFallback := false
	if last4SSN == "" {
		if p.fallbackLast4SSN == "" {
			return nil, fmt.Errorf("row %d: last 4 SSN column %q is empty", rowdata.RowIndex, p.inLast4SSN)
		}
		last4SSN = p.fallbackLast4SSN
		usedFallback = true
	}

	if !usedFallback {
		if len(last4SSN) != 4 {
			return nil, fmt.Errorf("row %d: last 4 SSN in column %q must be exactly 4 ASCII digits", rowdata.RowIndex, p.inLast4SSN)
		}
		for _, r := range last4SSN {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("row %d: last 4 SSN in column %q must be exactly 4 ASCII digits", rowdata.RowIndex, p.inLast4SSN)
			}
		}
	}

	dobStr := strings.TrimSpace(rowdata.RowMap[p.inDOB])
	if dobStr == "" {
		return nil, fmt.Errorf("row %d: DOB column %q is empty", rowdata.RowIndex, p.inDOB)
	}
	dobTime, err := parseDate(dobStr)
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowdata.RowIndex, err)
	}
	dob := dobTime.Format("2006-01-02")

	// Pass raw names to hasher (normalization happens inside the hasher)
	uniqueID := p.hasher(lastName, firstName, last4SSN, dob)
	return []CSVOutputString{Quoted(uniqueID)}, nil
}

// UniqueID creates a UniqueID processor with fully specified column names.
func UniqueID(inLastName, inFirstName, inLast4SSN, fallbackLast4SSN, inDOB, outUniquePlayerID string,
	hasher func(lastName, firstName, last4SSN, dob string) string) InColumnProcessor {
	return &uniqueIDProcessor{
		inLastName:        inLastName,
		inFirstName:       inFirstName,
		inLast4SSN:        inLast4SSN,
		fallbackLast4SSN:  fallbackLast4SSN,
		inDOB:             inDOB,
		outUniquePlayerID: outUniquePlayerID,
		hasher:            hasher,
	}
}

// UniqueIDDefault creates a UniqueID processor with default column names.
func UniqueIDDefault(hasher func(lastName, firstName, last4SSN, dob string) string) InColumnProcessor {
	return UniqueID("LastName", "FirstName", "Last4SSN", "", "DOB", "UniquePlayerID", hasher)
}

// UniqueIDDefaultNullableLast4SSN creates a UniqueID processor with default column names
// and "XXXX" as fallback for missing Last4SSN.
func UniqueIDDefaultNullableLast4SSN(hasher func(lastName, firstName, last4SSN, dob string) string) InColumnProcessor {
	return UniqueID("LastName", "FirstName", "Last4SSN", "XXXX", "DOB", "UniquePlayerID", hasher)
}

// --- OrgCountryProcessor ---

type orgCountryProcessor struct {
	inCountry  string
	inState    string
	outCountry string
	outState   string
}

func (p *orgCountryProcessor) InputColumns() []string  { return []string{p.inCountry, p.inState} }
func (p *orgCountryProcessor) OutputColumns() []string { return []string{p.outCountry, p.outState} }
func (p *orgCountryProcessor) OutputUniqueID() bool    { return false }
func (p *orgCountryProcessor) OutputMetaID() bool      { return false }

func (p *orgCountryProcessor) Process(rowdata RowData) ([]CSVOutputString, error) {
	country := strings.TrimSpace(rowdata.RowMap[p.inCountry])
	if country == "" {
		return nil, fmt.Errorf("row %d: country column %q is empty", rowdata.RowIndex, p.inCountry)
	}
	country = strings.ToUpper(country)

	state := strings.TrimSpace(rowdata.RowMap[p.inState])
	state = strings.ToUpper(state)

	subdivisions, err := hashers.GetCountrySubdivisions(country)
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowdata.RowIndex, err)
	}

	if len(subdivisions) > 0 {
		if state == "" {
			return nil, fmt.Errorf("row %d: state is required for country %q", rowdata.RowIndex, country)
		}
		if !slices.Contains(subdivisions, state) {
			return nil, fmt.Errorf("row %d: invalid state %q for country %q", rowdata.RowIndex, state, country)
		}
	}

	return []CSVOutputString{Quoted(country), Quoted(state)}, nil
}

// CountryAndState creates a country/state validation processor.
func CountryAndState(inCountry, inState, outCountry, outState string) InColumnProcessor {
	return &orgCountryProcessor{
		inCountry:  inCountry,
		inState:    inState,
		outCountry: outCountry,
		outState:   outState,
	}
}

// CountryAndStateDefault creates a country/state processor with default column names.
func CountryAndStateDefault() InColumnProcessor {
	return CountryAndState("Country", "State", "Country", "State")
}
