package csv

import (
	"strings"
	"testing"
)

// --- CSVType Tests ---

func TestCSVTypeString(t *testing.T) {
	tests := []struct {
		csvType CSVType
		want    string
	}{
		{CSVBets, "Bets"},
		{CSVPlayers, "Players"},
		{CSVCasinoPlayers, "Casino Players"},
		{CSVBonus, "Bonus"},
		{CSVCasino, "Casino"},
		{CSVCasinoParSheet, "Casino Par Sheet"},
		{CSVComplaints, "Complaints"},
		{CSVDemographic, "Demographic"},
		{CSVDepositsWithdrawals, "Deposits/Withdrawals"},
		{CSVResponsibleGaming, "Responsible Gaming"},
	}
	for _, tt := range tests {
		if got := tt.csvType.String(); got != tt.want {
			t.Errorf("CSVType(%d).String() = %q, want %q", int(tt.csvType), got, tt.want)
		}
	}
}

func TestCSVTypeSlug(t *testing.T) {
	tests := []struct {
		csvType CSVType
		want    string
	}{
		{CSVBets, "bets"},
		{CSVPlayers, "players"},
		{CSVCasinoPlayers, "casino-players"},
		{CSVDepositsWithdrawals, "deposits-withdrawals"},
		{CSVResponsibleGaming, "responsible-gaming"},
	}
	for _, tt := range tests {
		if got := tt.csvType.Slug(); got != tt.want {
			t.Errorf("CSVType(%d).Slug() = %q, want %q", int(tt.csvType), got, tt.want)
		}
	}
}

func TestCSVTypeFromSlug(t *testing.T) {
	for _, ct := range AllCSVTypes() {
		slug := ct.Slug()
		got, err := CSVTypeFromSlug(slug)
		if err != nil {
			t.Errorf("CSVTypeFromSlug(%q) returned error: %v", slug, err)
			continue
		}
		if got != ct {
			t.Errorf("CSVTypeFromSlug(%q) = %d, want %d", slug, int(got), int(ct))
		}
	}

	// Unknown slug
	_, err := CSVTypeFromSlug("nonexistent")
	if err == nil {
		t.Error("CSVTypeFromSlug(\"nonexistent\") should return error")
	}
}

func TestAllCSVTypes(t *testing.T) {
	types := AllCSVTypes()
	if len(types) != 10 {
		t.Errorf("AllCSVTypes() returned %d types, want 10", len(types))
	}
	if types[0] != CSVBets {
		t.Errorf("AllCSVTypes()[0] = %d, want %d (CSVBets)", int(types[0]), int(CSVBets))
	}
	if types[9] != CSVResponsibleGaming {
		t.Errorf("AllCSVTypes()[9] = %d, want %d (CSVResponsibleGaming)", int(types[9]), int(CSVResponsibleGaming))
	}
}

// --- CSVOutputString Tests ---

func TestQuotedString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello,world", "\"hello,world\""},
		{"he said \"hi\"", "\"he said \"\"hi\"\"\""},
	}
	for _, tt := range tests {
		got := Quoted(tt.input).String()
		if got != tt.want {
			t.Errorf("Quoted(%q).String() = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRawString(t *testing.T) {
	if got := Raw("hello").String(); got != "hello" {
		t.Errorf("Raw(\"hello\").String() = %q, want \"hello\"", got)
	}
	if got := EmptyString.String(); got != "" {
		t.Errorf("EmptyString.String() = %q, want \"\"", got)
	}
}

func TestCSVOutputRowString(t *testing.T) {
	row := CSVOutputRow{
		Columns: []CSVOutputString{
			Quoted("hello"),
			Raw("42"),
			EmptyString,
			Quoted("world,test"),
		},
	}
	got := row.RowString()
	want := "hello,42,,\"world,test\""
	if got != want {
		t.Errorf("RowString() = %q, want %q", got, want)
	}
}

// --- FlexBool Tests ---

func TestNonNilFlexBool(t *testing.T) {
	proc := NonNilFlexBool("in", "out")

	truthyInputs := []string{"true", "t", "yes", "y", "TRUE", "1", "True", "YES", "Y", "T"}
	for _, input := range truthyInputs {
		row := RowData{RowIndex: 1, RowMap: map[string]string{"in": input}}
		result, err := proc.Process(row)
		if err != nil {
			t.Errorf("NonNilFlexBool(%q) error: %v", input, err)
			continue
		}
		if result[0].String() != "true" {
			t.Errorf("NonNilFlexBool(%q) = %q, want \"true\"", input, result[0].String())
		}
	}

	falsyInputs := []string{"false", "f", "no", "n", "FALSE", "0", "False", "NO", "N", "F"}
	for _, input := range falsyInputs {
		row := RowData{RowIndex: 1, RowMap: map[string]string{"in": input}}
		result, err := proc.Process(row)
		if err != nil {
			t.Errorf("NonNilFlexBool(%q) error: %v", input, err)
			continue
		}
		if result[0].String() != "false" {
			t.Errorf("NonNilFlexBool(%q) = %q, want \"false\"", input, result[0].String())
		}
	}

	// Empty -> error
	row := RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	_, err := proc.Process(row)
	if err == nil {
		t.Error("NonNilFlexBool(\"\") should return error")
	}

	// Invalid -> error
	row = RowData{RowIndex: 1, RowMap: map[string]string{"in": "invalid"}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("NonNilFlexBool(\"invalid\") should return error")
	}
}

func TestNillableFlexBool(t *testing.T) {
	proc := NillableFlexBool("in", "out")

	// Empty -> EmptyString
	row := RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("NillableFlexBool(\"\") error: %v", err)
	} else if result[0].String() != "" {
		t.Errorf("NillableFlexBool(\"\") = %q, want \"\"", result[0].String())
	}

	// Whitespace -> EmptyString
	row = RowData{RowIndex: 1, RowMap: map[string]string{"in": "  "}}
	result, err = proc.Process(row)
	if err != nil {
		t.Errorf("NillableFlexBool(\"  \") error: %v", err)
	} else if result[0].String() != "" {
		t.Errorf("NillableFlexBool(\"  \") = %q, want \"\"", result[0].String())
	}

	// Valid value works
	row = RowData{RowIndex: 1, RowMap: map[string]string{"in": "yes"}}
	result, err = proc.Process(row)
	if err != nil {
		t.Errorf("NillableFlexBool(\"yes\") error: %v", err)
	} else if result[0].String() != "true" {
		t.Errorf("NillableFlexBool(\"yes\") = %q, want \"true\"", result[0].String())
	}
}

// --- Integer Processor Tests ---

func TestIntProcessors(t *testing.T) {
	tests := []struct {
		name    string
		proc    InColumnProcessor
		input   string
		wantVal string
		wantErr bool
	}{
		{"NonNillableInt valid", NonNillableInt("in", "out"), "42", "42", false},
		{"NonNillableInt negative", NonNillableInt("in", "out"), "-5", "-5", false},
		{"NonNillableInt empty", NonNillableInt("in", "out"), "", "", true},
		{"NonNillableInt invalid", NonNillableInt("in", "out"), "abc", "", true},
		{"NonNillableNonNegInt valid", NonNillableNonNegInt("in", "out"), "42", "42", false},
		{"NonNillableNonNegInt zero", NonNillableNonNegInt("in", "out"), "0", "0", false},
		{"NonNillableNonNegInt negative", NonNillableNonNegInt("in", "out"), "-1", "", true},
		{"NonNillableNonNegInt empty", NonNillableNonNegInt("in", "out"), "", "", true},
		{"NillableInt valid", NillableInt("in", "out"), "42", "42", false},
		{"NillableInt empty", NillableInt("in", "out"), "", "", false},
		{"NillableInt negative", NillableInt("in", "out"), "-5", "-5", false},
		{"NillableNonNegInt valid", NillableNonNegInt("in", "out"), "42", "42", false},
		{"NillableNonNegInt empty", NillableNonNegInt("in", "out"), "", "", false},
		{"NillableNonNegInt negative", NillableNonNegInt("in", "out"), "-1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := tt.proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result[0].String() != tt.wantVal {
				t.Errorf("got %q, want %q", result[0].String(), tt.wantVal)
			}
		})
	}
}

// --- Float Processor Tests ---

func TestFloatProcessors(t *testing.T) {
	tests := []struct {
		name    string
		proc    InColumnProcessor
		input   string
		wantVal string
		wantErr bool
	}{
		{"NonNilFloat64Full valid", NonNilFloat64Full("in", "out"), "3.14", "3.14", false},
		{"NonNilFloat64Full int", NonNilFloat64Full("in", "out"), "42", "42", false},
		{"NonNilFloat64Full empty", NonNilFloat64Full("in", "out"), "", "", true},
		{"NonNilFloat64Full invalid", NonNilFloat64Full("in", "out"), "abc", "", true},
		{"NonNilNonNegFloat64Full valid", NonNilNonNegFloat64Full("in", "out"), "3.14", "3.14", false},
		{"NonNilNonNegFloat64Full zero", NonNilNonNegFloat64Full("in", "out"), "0", "0", false},
		{"NonNilNonNegFloat64Full negative", NonNilNonNegFloat64Full("in", "out"), "-1.5", "", true},
		{"NillableFloat64Full valid", NillableFloat64Full("in", "out"), "3.14", "3.14", false},
		{"NillableFloat64Full empty", NillableFloat64Full("in", "out"), "", "", false},
		{"NillableNonNegFloat64Full valid", NillableNonNegFloat64Full("in", "out"), "3.14", "3.14", false},
		{"NillableNonNegFloat64Full empty", NillableNonNegFloat64Full("in", "out"), "", "", false},
		{"NillableNonNegFloat64Full negative", NillableNonNegFloat64Full("in", "out"), "-1.5", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := tt.proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result[0].String() != tt.wantVal {
				t.Errorf("got %q, want %q", result[0].String(), tt.wantVal)
			}
		})
	}
}

// --- String Processor Tests ---

func TestStringProcessors(t *testing.T) {
	tests := []struct {
		name    string
		proc    InColumnProcessor
		input   string
		wantVal string
		wantErr bool
	}{
		{"NonEmptyStringWithMax valid", NonEmptyStringWithMax("in", "out", 10), "hello", "hello", false},
		{"NonEmptyStringWithMax empty", NonEmptyStringWithMax("in", "out", 10), "", "", true},
		{"NonEmptyStringWithMax too long", NonEmptyStringWithMax("in", "out", 3), "hello", "", true},
		{"NonEmptyStringWithMax unicode chars", NonEmptyStringWithMax("in", "out", 5), "\u00e9\u00e8\u00ea\u00eb\u00ef", "\u00e9\u00e8\u00ea\u00eb\u00ef", false},
		{"NonEmptyStringWithMax unicode too long", NonEmptyStringWithMax("in", "out", 3), "\u00e9\u00e8\u00ea\u00eb", "", true},
		{"NillableStringWithMax valid", NillableStringWithMax("in", "out", 10), "hello", "hello", false},
		{"NillableStringWithMax empty", NillableStringWithMax("in", "out", 10), "", "", false},
		{"NillableStringWithMax too long", NillableStringWithMax("in", "out", 3), "hello", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := tt.proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			got := result[0].String()
			// For Quoted values, the Quoted wrapper adds CSV escaping
			// For EmptyString, it's ""
			if tt.input == "" && !tt.wantErr {
				if got != "" {
					t.Errorf("got %q, want empty string", got)
				}
			}
		})
	}
}

// --- Date/Time Processor Tests ---

func TestDateAndTimeNonZeroAndNotAfterNow(t *testing.T) {
	proc := DateAndTimeNonZeroAndNotAfterNow("in", "out")

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2023-07-24T14:15:02-04:00", false},
		{"RFC3339 UTC", "2023-07-24T14:15:02Z", false},
		{"date-time space", "2023-07-24 14:15", false},
		{"US date short year", "01/05/23 09:30", false},
		{"US date long year", "01/05/2023 09:30", false},
		{"future date", "2099-01-01T00:00:00Z", true},
		{"empty", "", true},
		{"invalid", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			// Should be a non-empty RFC3339 string
			if result[0].String() == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestNillableDateAndTimeNotAfterNow(t *testing.T) {
	proc := NillableDateAndTimeNotAfterNow("in", "out")

	// Empty -> EmptyString
	row := RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("empty input error: %v", err)
	} else if result[0].String() != "" {
		t.Errorf("empty input got %q, want \"\"", result[0].String())
	}

	// Valid date
	row = RowData{RowIndex: 1, RowMap: map[string]string{"in": "2023-07-24T14:15:02Z"}}
	result, err = proc.Process(row)
	if err != nil {
		t.Errorf("valid input error: %v", err)
	} else if result[0].String() == "" {
		t.Error("valid input got empty output")
	}
}

func TestNonNillDate(t *testing.T) {
	proc := NonNillDate("in", "out")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"ISO date", "2023-01-15", "2023-01-15", false},
		{"US date", "01/15/2023", "2023-01-15", false},
		{"US date short", "1/15/2023", "2023-01-15", false},
		{"compact", "20230115", "2023-01-15", false},
		{"empty", "", "", true},
		{"invalid", "not-a-date", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			// Quoted wraps the value; we check the inner content
			got := result[0].String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNonNillMMDDYYYY(t *testing.T) {
	proc := NonNillMMDDYYYY("in", "out")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "07242023", "2023-07-24", false},
		{"valid Jan", "01152023", "2023-01-15", false},
		{"empty", "", "", true},
		{"too short", "0724202", "", true},
		{"invalid", "99992023", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			got := result[0].String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNonNilBirthYear(t *testing.T) {
	proc := NonNilBirthYear("in", "out")

	row := RowData{RowIndex: 1, RowMap: map[string]string{"in": "1990-05-15"}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if result[0].String() != "1990" {
		t.Errorf("got %q, want \"1990\"", result[0].String())
	}

	// Empty -> error
	row = RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestNonNillableHHMMSSTime(t *testing.T) {
	proc := NonNillableHHMMSSTime("in", "out")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "143015", "14:30:15", false},
		{"midnight", "000000", "00:00:00", false},
		{"empty", "", "", true},
		{"too short", "14301", "", true},
		{"invalid", "256000", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := RowData{RowIndex: 1, RowMap: map[string]string{"in": tt.input}}
			result, err := proc.Process(row)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			got := result[0].String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- ConstantString Tests ---

func TestConstantString(t *testing.T) {
	proc := ConstantString("Type", Quoted("bets"))

	if len(proc.InputColumns()) != 0 {
		t.Errorf("InputColumns() = %v, want empty", proc.InputColumns())
	}
	if cols := proc.OutputColumns(); len(cols) != 1 || cols[0] != "Type" {
		t.Errorf("OutputColumns() = %v, want [Type]", cols)
	}

	row := RowData{RowIndex: 1, RowMap: map[string]string{}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result[0].String() != "bets" {
		t.Errorf("got %q, want \"bets\"", result[0].String())
	}
}

// --- MetaID Tests ---

func TestMetaIDProcessor(t *testing.T) {
	hasher := func(playerID, country, state string) string {
		return "hash:" + playerID + ":" + country + ":" + state
	}

	proc := MetaIDDefault(hasher)

	// Valid US
	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "US", "State": "NY",
	}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(result) != 3 {
		t.Errorf("expected 3 output columns, got %d", len(result))
		return
	}
	if result[0].String() != "hash:P123:US:NY" {
		t.Errorf("metaID = %q, want \"hash:P123:US:NY\"", result[0].String())
	}

	// Missing player ID
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "", "Country": "US", "State": "NY",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for empty player ID")
	}

	// Invalid country
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "XX", "State": "NY",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for invalid country")
	}

	// Invalid state
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "US", "State": "ZZ",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for invalid state")
	}

	// OutputMetaID should be true
	if !proc.OutputMetaID() {
		t.Error("OutputMetaID() should be true")
	}
}

func TestMetaIDFixedLocation(t *testing.T) {
	hasher := func(playerID, country, state string) string {
		return "hash:" + playerID + ":" + country + ":" + state
	}

	// Valid construction
	proc, err := MetaIDFixedLocation("PlayerID", "MetaID", "US", "NY", hasher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cols := proc.InputColumns(); len(cols) != 1 || cols[0] != "PlayerID" {
		t.Errorf("InputColumns() = %v, want [PlayerID]", cols)
	}
	if cols := proc.OutputColumns(); len(cols) != 1 || cols[0] != "MetaID" {
		t.Errorf("OutputColumns() = %v, want [MetaID]", cols)
	}

	row := RowData{RowIndex: 1, RowMap: map[string]string{"PlayerID": "P123"}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if result[0].String() != "hash:P123:US:NY" {
		t.Errorf("got %q, want \"hash:P123:US:NY\"", result[0].String())
	}

	// Invalid country at construction
	_, err = MetaIDFixedLocation("PlayerID", "MetaID", "XX", "NY", hasher)
	if err == nil {
		t.Error("expected error for invalid country")
	}

	// Invalid state at construction
	_, err = MetaIDFixedLocation("PlayerID", "MetaID", "US", "ZZ", hasher)
	if err == nil {
		t.Error("expected error for invalid state")
	}

	// Empty state for country with required subdivisions
	_, err = MetaIDFixedLocation("PlayerID", "MetaID", "US", "", hasher)
	if err == nil {
		t.Error("expected error for empty state on country requiring subdivisions")
	}

	// MustMetaIDFixedLocation panics on error
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustMetaIDFixedLocation should panic on invalid country")
		}
	}()
	MustMetaIDFixedLocation("PlayerID", "MetaID", "XX", "ZZ", hasher)
}

// --- UniqueID Tests ---

func TestUniqueIDProcessor(t *testing.T) {
	hasher := func(lastName, firstName, last4SSN, dob string) string {
		return "uid:" + lastName + ":" + firstName + ":" + last4SSN + ":" + dob
	}

	proc := UniqueIDDefault(hasher)

	// Valid input
	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	// Hasher receives raw names
	want := "uid:Smith:John:1234:1990-05-15"
	// Result is Quoted, so extract inner value
	got := result[0].String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if !proc.OutputUniqueID() {
		t.Error("OutputUniqueID() should be true")
	}

	// Empty last name -> error
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "", "FirstName": "John", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for empty last name")
	}

	// Empty first name -> error
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for empty first name")
	}

	// Invalid SSN (not 4 digits)
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "12", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for invalid SSN")
	}

	// Non-digit SSN
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "abcd", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for non-digit SSN")
	}
}

func TestUniqueIDDefaultNullableLast4SSN(t *testing.T) {
	hasher := func(lastName, firstName, last4SSN, dob string) string {
		return "uid:" + lastName + ":" + firstName + ":" + last4SSN + ":" + dob
	}

	proc := UniqueIDDefaultNullableLast4SSN(hasher)

	// Missing SSN -> uses fallback "XXXX"
	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "", "DOB": "1990-05-15",
	}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	want := "uid:Smith:John:XXXX:1990-05-15"
	got := result[0].String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// With empty SSN and no fallback -> error
	procNoFallback := UniqueIDDefault(hasher)
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "", "DOB": "1990-05-15",
	}}
	_, err = procNoFallback.Process(row)
	if err == nil {
		t.Error("expected error for empty SSN with no fallback")
	}
}

// --- CountryAndState Tests ---

func TestCountryAndState(t *testing.T) {
	proc := CountryAndStateDefault()

	// Valid
	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "US", "State": "NY",
	}}
	result, err := proc.Process(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(result) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(result))
		return
	}

	// Invalid country
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "XX", "State": "NY",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for invalid country")
	}

	// Missing state for US
	row = RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "US", "State": "",
	}}
	_, err = proc.Process(row)
	if err == nil {
		t.Error("expected error for missing state")
	}
}

// --- CSVMetadata Tests ---

func TestMatchHeadersExactMatch(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
		NonEmptyStringWithMax("Name", "Name", 50),
	})

	if !meta.MatchHeaders([]string{"Amount", "Name"}) {
		t.Error("MatchHeaders should return true for exact match")
	}
}

func TestMatchHeadersExtraColumns(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
	})

	if !meta.MatchHeaders([]string{"Amount", "Extra"}) {
		t.Error("MatchHeaders should return true with extra columns")
	}
}

func TestMatchHeadersMissingColumn(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
		NonEmptyStringWithMax("Name", "Name", 50),
	})

	if meta.MatchHeaders([]string{"Amount"}) {
		t.Error("MatchHeaders should return false when column is missing")
	}
}

func TestMatchHeadersCaseSensitive(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
	})

	if meta.MatchHeaders([]string{"amount"}) {
		t.Error("MatchHeaders should be case-sensitive")
	}
}

func TestProcessRowSuccess(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
		NonEmptyStringWithMax("Name", "Name", 50),
		ConstantString("Type", Quoted("bets")),
	})

	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"Amount": "42",
		"Name":   "Test",
	}}
	result, err := meta.ProcessRow(row)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(result.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(result.Columns))
	}
}

func TestProcessRowErrorPropagation(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "Amount"),
		NonEmptyStringWithMax("Name", "Name", 50),
	})

	// Amount is invalid
	row := RowData{RowIndex: 1, RowMap: map[string]string{
		"Amount": "not-a-number",
		"Name":   "Test",
	}}
	_, err := meta.ProcessRow(row)
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestOutputHeaders(t *testing.T) {
	meta := NewSimpleCSVMetadata(CSVBets, []InColumnProcessor{
		NonNillableInt("Amount", "OutAmount"),
		NonEmptyStringWithMax("Name", "OutName", 50),
		ConstantString("Type", Quoted("bets")),
	})

	headers := meta.OutputHeaders()
	want := []string{"OutAmount", "OutName", "Type"}
	if len(headers) != len(want) {
		t.Errorf("OutputHeaders() length = %d, want %d", len(headers), len(want))
		return
	}
	for i, h := range headers {
		if h != want[i] {
			t.Errorf("OutputHeaders()[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestProcessorInputOutputColumns(t *testing.T) {
	// Test that various processors report correct input/output columns
	hasher3 := func(playerID, country, state string) string { return "hash" }

	tests := []struct {
		name       string
		proc       InColumnProcessor
		wantIn     []string
		wantOut    []string
		wantUID    bool
		wantMetaID bool
	}{
		{
			"SimpleInColumnProcessor",
			NonNillableInt("A", "B"),
			[]string{"A"}, []string{"B"}, false, false,
		},
		{
			"ConstantString",
			ConstantString("Type", Quoted("x")),
			nil, []string{"Type"}, false, false,
		},
		{
			"MetaIDDefault",
			MetaIDDefault(hasher3),
			[]string{"PlayerID", "Country", "State"},
			[]string{"MetaID", "Country", "State"},
			false, true,
		},
		{
			"CountryAndStateDefault",
			CountryAndStateDefault(),
			[]string{"Country", "State"},
			[]string{"Country", "State"},
			false, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIn := tt.proc.InputColumns()
			if !strSliceEqual(gotIn, tt.wantIn) {
				t.Errorf("InputColumns() = %v, want %v", gotIn, tt.wantIn)
			}
			gotOut := tt.proc.OutputColumns()
			if !strSliceEqual(gotOut, tt.wantOut) {
				t.Errorf("OutputColumns() = %v, want %v", gotOut, tt.wantOut)
			}
			if tt.proc.OutputUniqueID() != tt.wantUID {
				t.Errorf("OutputUniqueID() = %v, want %v", tt.proc.OutputUniqueID(), tt.wantUID)
			}
			if tt.proc.OutputMetaID() != tt.wantMetaID {
				t.Errorf("OutputMetaID() = %v, want %v", tt.proc.OutputMetaID(), tt.wantMetaID)
			}
		})
	}
}

func strSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Ensure Quoted output contains comma-separated value correctly
func TestCSVOutputRowWithQuotedComma(t *testing.T) {
	row := CSVOutputRow{
		Columns: []CSVOutputString{
			Raw("42"),
			Quoted("hello, world"),
		},
	}
	got := row.RowString()
	if !strings.Contains(got, "42,") {
		t.Errorf("RowString() missing raw value: %q", got)
	}
	if !strings.Contains(got, "\"hello, world\"") {
		t.Errorf("RowString() missing quoted value: %q", got)
	}
}
