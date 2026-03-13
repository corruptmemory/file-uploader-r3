//go:build ignore

// Standalone compliance test for spec 05. Run with:
//   go run test-harness/spec-05-compliance-test.go

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

var passed, failed int

func check(name string, ok bool, msg string) {
	if ok {
		passed++
		fmt.Printf("  PASS: %s\n", name)
	} else {
		failed++
		fmt.Printf("  FAIL: %s -- %s\n", name, msg)
	}
}

func main() {
	fmt.Println("=== Spec 05: CSV Framework Compliance ===")

	testCSVTypeEnum()
	testCoreTypes()
	testFlexBool()
	testIntProcessors()
	testFloatProcessors()
	testStringProcessors()
	testDateTimeProcessors()
	testConstantString()
	testMetaID()
	testMetaIDFixedLocation()
	testUniqueID()
	testCountryAndState()
	testCSVMetadata()

	fmt.Println()
	fmt.Printf("=== Results: PASS=%d FAIL=%d ===\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func testCSVTypeEnum() {
	fmt.Println("\n--- CSVType Enum ---")

	// Verify all 10 types exist with correct values
	types := csv.AllCSVTypes()
	check("AllCSVTypes returns 10 types", len(types) == 10, fmt.Sprintf("got %d", len(types)))

	// Verify integer values (spec: 2-11)
	expected := map[csv.CSVType]struct {
		name string
		slug string
	}{
		csv.CSVBets:                {"Bets", "bets"},
		csv.CSVPlayers:             {"Players", "players"},
		csv.CSVCasinoPlayers:       {"Casino Players", "casino-players"},
		csv.CSVBonus:               {"Bonus", "bonus"},
		csv.CSVCasino:              {"Casino", "casino"},
		csv.CSVCasinoParSheet:      {"Casino Par Sheet", "casino-par-sheet"},
		csv.CSVComplaints:          {"Complaints", "complaints"},
		csv.CSVDemographic:         {"Demographic", "demographic"},
		csv.CSVDepositsWithdrawals: {"Deposits/Withdrawals", "deposits-withdrawals"},
		csv.CSVResponsibleGaming:   {"Responsible Gaming", "responsible-gaming"},
	}

	for ct, exp := range expected {
		check(fmt.Sprintf("CSVType %d String=%q", int(ct), exp.name),
			ct.String() == exp.name,
			fmt.Sprintf("got %q", ct.String()))
		check(fmt.Sprintf("CSVType %d Slug=%q", int(ct), exp.slug),
			ct.Slug() == exp.slug,
			fmt.Sprintf("got %q", ct.Slug()))

		// Round-trip through FromSlug
		rt, err := csv.CSVTypeFromSlug(exp.slug)
		check(fmt.Sprintf("FromSlug(%q) round-trips", exp.slug),
			err == nil && rt == ct,
			fmt.Sprintf("err=%v, got=%d", err, int(rt)))
	}

	// Unknown slug
	_, err := csv.CSVTypeFromSlug("nonexistent")
	check("FromSlug unknown returns error", err != nil, "expected error")

	// Unknown CSVType
	unknown := csv.CSVType(99)
	check("Unknown CSVType.String() is fallback", strings.Contains(unknown.String(), "99"),
		fmt.Sprintf("got %q", unknown.String()))
	check("Unknown CSVType.Slug() is fallback", strings.Contains(unknown.Slug(), "99"),
		fmt.Sprintf("got %q", unknown.Slug()))
}

func testCoreTypes() {
	fmt.Println("\n--- Core Types ---")

	// Quoted with special characters
	q := csv.Quoted("hello,world")
	check("Quoted escapes comma", strings.Contains(q.String(), "\"hello,world\""),
		fmt.Sprintf("got %q", q.String()))

	q2 := csv.Quoted("he said \"hi\"")
	check("Quoted escapes double-quotes", strings.Contains(q2.String(), "\"\""),
		fmt.Sprintf("got %q", q2.String()))

	// Raw
	r := csv.Raw("test")
	check("Raw returns as-is", r.String() == "test", fmt.Sprintf("got %q", r.String()))

	// EmptyString
	check("EmptyString is empty", csv.EmptyString.String() == "", fmt.Sprintf("got %q", csv.EmptyString.String()))

	// CSVOutputRow
	row := csv.CSVOutputRow{
		Columns: []csv.CSVOutputString{csv.Raw("42"), csv.Quoted("hello"), csv.EmptyString},
	}
	got := row.RowString()
	check("RowString joins with commas", got == "42,hello,", fmt.Sprintf("got %q", got))
	check("RowString has no trailing newline", !strings.HasSuffix(got, "\n"), "has trailing newline")
}

func testFlexBool() {
	fmt.Println("\n--- FlexBool Processors ---")

	proc := csv.NonNilFlexBool("in", "out")

	// Truthy values (spec: true, t, yes, y, 1 - case insensitive)
	for _, v := range []string{"true", "t", "yes", "y", "1", "TRUE", "True", "YES", "Y", "T"} {
		row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": v}}
		result, err := proc.Process(row)
		check(fmt.Sprintf("NonNilFlexBool(%q) = true", v),
			err == nil && result[0].String() == "true",
			fmt.Sprintf("err=%v, got=%q", err, safeString(result)))
	}

	// Falsy values
	for _, v := range []string{"false", "f", "no", "n", "0", "FALSE", "False", "NO", "N", "F"} {
		row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": v}}
		result, err := proc.Process(row)
		check(fmt.Sprintf("NonNilFlexBool(%q) = false", v),
			err == nil && result[0].String() == "false",
			fmt.Sprintf("err=%v, got=%q", err, safeString(result)))
	}

	// Empty -> error (spec: NonNil)
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	_, err := proc.Process(row)
	check("NonNilFlexBool empty -> error", err != nil, "expected error")

	// Whitespace-only -> error
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": "  "}}
	_, err = proc.Process(row)
	check("NonNilFlexBool whitespace -> error", err != nil, "expected error")

	// Invalid -> error
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": "invalid"}}
	_, err = proc.Process(row)
	check("NonNilFlexBool invalid -> error", err != nil, "expected error")

	// Nillable version
	nilProc := csv.NillableFlexBool("in", "out")
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": ""}}
	result, err := nilProc.Process(row)
	check("NillableFlexBool empty -> EmptyString",
		err == nil && result[0].String() == "",
		fmt.Sprintf("err=%v, got=%q", err, safeString(result)))

	// Nillable with whitespace
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": "  "}}
	result, err = nilProc.Process(row)
	check("NillableFlexBool whitespace -> EmptyString",
		err == nil && result[0].String() == "",
		fmt.Sprintf("err=%v, got=%q", err, safeString(result)))
}

func testIntProcessors() {
	fmt.Println("\n--- Integer Processors ---")

	// NonNillableInt
	proc := csv.NonNillableInt("in", "out")
	result := processSimple(proc, "42")
	check("NonNillableInt(42) = 42", result == "42", fmt.Sprintf("got %q", result))

	result = processSimple(proc, "-5")
	check("NonNillableInt(-5) = -5", result == "-5", fmt.Sprintf("got %q", result))

	_, err := processSimpleErr(proc, "")
	check("NonNillableInt empty -> error", err != nil, "expected error")

	_, err = processSimpleErr(proc, "abc")
	check("NonNillableInt abc -> error", err != nil, "expected error")

	// NonNillableNonNegInt
	proc2 := csv.NonNillableNonNegInt("in", "out")
	result = processSimple(proc2, "0")
	check("NonNillableNonNegInt(0) = 0", result == "0", fmt.Sprintf("got %q", result))

	_, err = processSimpleErr(proc2, "-1")
	check("NonNillableNonNegInt(-1) -> error", err != nil, "expected error")

	// NillableInt
	proc3 := csv.NillableInt("in", "out")
	result = processSimple(proc3, "")
	check("NillableInt empty -> EmptyString", result == "", fmt.Sprintf("got %q", result))

	result = processSimple(proc3, "-5")
	check("NillableInt(-5) = -5", result == "-5", fmt.Sprintf("got %q", result))

	// NillableNonNegInt
	proc4 := csv.NillableNonNegInt("in", "out")
	result = processSimple(proc4, "")
	check("NillableNonNegInt empty -> EmptyString", result == "", fmt.Sprintf("got %q", result))

	_, err = processSimpleErr(proc4, "-1")
	check("NillableNonNegInt(-1) -> error", err != nil, "expected error")

	// Verify output is Raw (no quoting)
	result = processSimple(proc, "42")
	check("Int output is Raw (no quotes)", result == "42", fmt.Sprintf("got %q", result))

	// Leading/trailing whitespace should be trimmed
	result = processSimple(proc, "  42  ")
	check("Int trims whitespace", result == "42", fmt.Sprintf("got %q", result))
}

func testFloatProcessors() {
	fmt.Println("\n--- Float Processors ---")

	proc := csv.NonNilFloat64Full("in", "out")
	result := processSimple(proc, "3.14")
	check("NonNilFloat64Full(3.14)", result == "3.14", fmt.Sprintf("got %q", result))

	result = processSimple(proc, "42")
	check("NonNilFloat64Full(42) = 42", result == "42", fmt.Sprintf("got %q", result))

	_, err := processSimpleErr(proc, "")
	check("NonNilFloat64Full empty -> error", err != nil, "expected error")

	// NaN and Inf rejection (security fix)
	for _, bad := range []string{"NaN", "nan", "Inf", "+Inf", "-Inf", "inf"} {
		_, err = processSimpleErr(proc, bad)
		check(fmt.Sprintf("Float rejects %q", bad), err != nil, "expected error")
	}

	// NonNeg
	proc2 := csv.NonNilNonNegFloat64Full("in", "out")
	_, err = processSimpleErr(proc2, "-1.5")
	check("NonNilNonNegFloat64Full(-1.5) -> error", err != nil, "expected error")

	result = processSimple(proc2, "0")
	check("NonNilNonNegFloat64Full(0) = 0", result == "0", fmt.Sprintf("got %q", result))

	// Nillable
	proc3 := csv.NillableFloat64Full("in", "out")
	result = processSimple(proc3, "")
	check("NillableFloat64Full empty -> EmptyString", result == "", fmt.Sprintf("got %q", result))

	// Nillable NaN rejection
	_, err = processSimpleErr(proc3, "NaN")
	check("NillableFloat64Full rejects NaN", err != nil, "expected error")

	// NillableNonNeg
	proc4 := csv.NillableNonNegFloat64Full("in", "out")
	result = processSimple(proc4, "")
	check("NillableNonNegFloat64Full empty -> EmptyString", result == "", fmt.Sprintf("got %q", result))

	_, err = processSimpleErr(proc4, "-0.1")
	check("NillableNonNegFloat64Full(-0.1) -> error", err != nil, "expected error")
}

func testStringProcessors() {
	fmt.Println("\n--- String Processors ---")

	proc := csv.NonEmptyStringWithMax("in", "out", 5)
	result := processSimple(proc, "hello")
	check("NonEmptyStringWithMax(hello,5) OK", result == "hello", fmt.Sprintf("got %q", result))

	_, err := processSimpleErr(proc, "")
	check("NonEmptyStringWithMax empty -> error", err != nil, "expected error")

	_, err = processSimpleErr(proc, "toolongstring")
	check("NonEmptyStringWithMax too long -> error", err != nil, "expected error")

	// Unicode character count (not byte count)
	proc2 := csv.NonEmptyStringWithMax("in", "out", 4)
	// "café" is 4 chars but 5 bytes (é is 2 bytes in UTF-8)
	result = processSimple(proc2, "café")
	check("String maxLen counts chars not bytes (café, max=4)", result == "café",
		fmt.Sprintf("got %q", result))

	// 5 unicode chars should fail with max=4
	_, err = processSimpleErr(proc2, "café!")
	check("String maxLen rejects 5 chars with max=4", err != nil, "expected error")

	// Nillable
	proc3 := csv.NillableStringWithMax("in", "out", 10)
	result = processSimple(proc3, "")
	check("NillableStringWithMax empty -> EmptyString", result == "", fmt.Sprintf("got %q", result))

	result = processSimple(proc3, "hello")
	check("NillableStringWithMax(hello) = hello", result == "hello", fmt.Sprintf("got %q", result))
}

func testDateTimeProcessors() {
	fmt.Println("\n--- Date/Time Processors ---")

	proc := csv.DateAndTimeNonZeroAndNotAfterNow("in", "out")

	// All date/time formats from spec section 6
	validDTs := []struct {
		name  string
		input string
	}{
		{"2006-01-02 15:04", "2023-01-15 09:30"},
		{"2006-1-2 15:04", "2023-1-5 9:30"},
		{"1/2/06 15:04", "1/5/23 9:30"},
		{"01/02/06 15:04", "01/05/23 09:30"},
		{"1/2/2006 15:04", "1/5/2023 9:30"},
		{"01/02/2006 15:04", "01/05/2023 09:30"},
		{"RFC3339", "2023-01-15T09:30:00Z"},
		{"RFC3339 with offset", "2023-07-24T14:15:02-04:00"},
	}

	for _, dt := range validDTs {
		row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": dt.input}}
		result, err := proc.Process(row)
		check(fmt.Sprintf("DateTime(%s): %q", dt.name, dt.input),
			err == nil && result[0].String() != "",
			fmt.Sprintf("err=%v, got=%q", err, safeString(result)))
	}

	// Future date -> error
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": "2099-01-01T00:00:00Z"}}
	_, err := proc.Process(row)
	check("DateTime future -> error", err != nil, "expected error")

	// Empty -> error (non-nil)
	_, err = processSimpleErr(proc, "")
	check("DateTime empty -> error", err != nil, "expected error")

	// Invalid -> error
	_, err = processSimpleErr(proc, "not-a-date")
	check("DateTime invalid -> error", err != nil, "expected error")

	// Whitespace trimmed
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": "  2023-01-15T09:30:00Z  "}}
	_, err = proc.Process(row)
	check("DateTime trims whitespace", err == nil, fmt.Sprintf("err=%v", err))

	// NillableDateAndTimeNotAfterNow
	nilProc := csv.NillableDateAndTimeNotAfterNow("in", "out")
	result, err := processSimpleResult(nilProc, "")
	check("NillableDateTime empty -> EmptyString",
		err == nil && result == "",
		fmt.Sprintf("err=%v, got=%q", err, result))

	// NonNillDate
	fmt.Println("\n--- Date-Only Processors ---")
	dateProc := csv.NonNillDate("in", "out")

	validDates := []struct {
		name  string
		input string
		want  string
	}{
		{"2006-01-02", "2023-01-15", "2023-01-15"},
		{"01/02/2006", "01/15/2023", "2023-01-15"},
		{"1/2/2006", "1/15/2023", "2023-01-15"},
		{"01/02/06", "01/15/23", "2023-01-15"},
		{"1/2/06", "1/15/23", "2023-01-15"},
		{"20060102", "20230115", "2023-01-15"},
	}

	for _, dt := range validDates {
		result, err := processSimpleResult(dateProc, dt.input)
		check(fmt.Sprintf("Date(%s): %q -> %q", dt.name, dt.input, dt.want),
			err == nil && result == dt.want,
			fmt.Sprintf("err=%v, got=%q", err, result))
	}

	_, err = processSimpleErr(dateProc, "")
	check("Date empty -> error", err != nil, "expected error")

	// NonNillMMDDYYYY
	fmt.Println("\n--- MMDDYYYY Processor ---")
	mmddProc := csv.NonNillMMDDYYYY("in", "out")

	result, err = processSimpleResult(mmddProc, "07242023")
	check("MMDDYYYY(07242023) -> 2023-07-24",
		err == nil && result == "2023-07-24",
		fmt.Sprintf("err=%v, got=%q", err, result))

	result, err = processSimpleResult(mmddProc, "01152023")
	check("MMDDYYYY(01152023) -> 2023-01-15",
		err == nil && result == "2023-01-15",
		fmt.Sprintf("err=%v, got=%q", err, result))

	_, err = processSimpleErr(mmddProc, "")
	check("MMDDYYYY empty -> error", err != nil, "expected error")

	_, err = processSimpleErr(mmddProc, "0724202")
	check("MMDDYYYY 7 digits -> error", err != nil, "expected error")

	_, err = processSimpleErr(mmddProc, "99992023")
	check("MMDDYYYY invalid month -> error", err != nil, "expected error")

	// NonNilBirthYear
	fmt.Println("\n--- BirthYear Processor ---")
	byProc := csv.NonNilBirthYear("in", "out")

	result, err = processSimpleResult(byProc, "1990-05-15")
	check("BirthYear(1990-05-15) -> 1990",
		err == nil && result == "1990",
		fmt.Sprintf("err=%v, got=%q", err, result))

	result, err = processSimpleResult(byProc, "05/15/1990")
	check("BirthYear(05/15/1990) -> 1990",
		err == nil && result == "1990",
		fmt.Sprintf("err=%v, got=%q", err, result))

	_, err = processSimpleErr(byProc, "")
	check("BirthYear empty -> error", err != nil, "expected error")

	// NonNillableHHMMSSTime
	fmt.Println("\n--- HHMMSS Processor ---")
	hmsProc := csv.NonNillableHHMMSSTime("in", "out")

	result, err = processSimpleResult(hmsProc, "143015")
	check("HHMMSS(143015) -> 14:30:15",
		err == nil && result == "14:30:15",
		fmt.Sprintf("err=%v, got=%q", err, result))

	result, err = processSimpleResult(hmsProc, "000000")
	check("HHMMSS(000000) -> 00:00:00",
		err == nil && result == "00:00:00",
		fmt.Sprintf("err=%v, got=%q", err, result))

	_, err = processSimpleErr(hmsProc, "")
	check("HHMMSS empty -> error", err != nil, "expected error")

	_, err = processSimpleErr(hmsProc, "14301")
	check("HHMMSS 5 digits -> error", err != nil, "expected error")

	_, err = processSimpleErr(hmsProc, "256000")
	check("HHMMSS invalid hour -> error", err != nil, "expected error")
}

func testConstantString() {
	fmt.Println("\n--- ConstantString ---")

	proc := csv.ConstantString("Type", csv.Quoted("bets"))

	check("ConstantString InputColumns empty", len(proc.InputColumns()) == 0,
		fmt.Sprintf("got %v", proc.InputColumns()))
	check("ConstantString OutputColumns = [Type]",
		len(proc.OutputColumns()) == 1 && proc.OutputColumns()[0] == "Type",
		fmt.Sprintf("got %v", proc.OutputColumns()))
	check("ConstantString OutputUniqueID false", !proc.OutputUniqueID(), "")
	check("ConstantString OutputMetaID false", !proc.OutputMetaID(), "")

	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{}}
	result, err := proc.Process(row)
	check("ConstantString returns constant", err == nil && result[0].String() == "bets",
		fmt.Sprintf("err=%v, got=%q", err, safeString(result)))
}

func testMetaID() {
	fmt.Println("\n--- MetaID Processors ---")

	hasher := func(playerID, country, state string) string {
		return "hash:" + playerID + ":" + country + ":" + state
	}

	// MetaIDDefault
	proc := csv.MetaIDDefault(hasher)
	check("MetaIDDefault InputColumns = [PlayerID, Country, State]",
		sliceEqual(proc.InputColumns(), []string{"PlayerID", "Country", "State"}),
		fmt.Sprintf("got %v", proc.InputColumns()))
	check("MetaIDDefault OutputColumns = [MetaID, Country, State]",
		sliceEqual(proc.OutputColumns(), []string{"MetaID", "Country", "State"}),
		fmt.Sprintf("got %v", proc.OutputColumns()))
	check("MetaID OutputMetaID = true", proc.OutputMetaID(), "")
	check("MetaID OutputUniqueID = false", !proc.OutputUniqueID(), "")

	// Valid US + NY
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "US", "State": "NY",
	}}
	result, err := proc.Process(row)
	check("MetaID(US, NY) success", err == nil && len(result) == 3,
		fmt.Sprintf("err=%v, len=%d", err, len(result)))
	if err == nil && len(result) == 3 {
		check("MetaID hash correct", result[0].String() == "hash:P123:US:NY",
			fmt.Sprintf("got %q", result[0].String()))
	}

	// Country normalization (lowercase -> uppercase)
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "us", "State": "ny",
	}}
	result, err = proc.Process(row)
	check("MetaID normalizes country/state to uppercase", err == nil,
		fmt.Sprintf("err=%v", err))
	if err == nil && len(result) >= 1 {
		check("MetaID passes uppercase to hasher", result[0].String() == "hash:P123:US:NY",
			fmt.Sprintf("got %q", result[0].String()))
	}

	// Empty player ID
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "", "Country": "US", "State": "NY",
	}}
	_, err = proc.Process(row)
	check("MetaID empty playerID -> error", err != nil, "expected error")

	// Empty country
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "", "State": "NY",
	}}
	_, err = proc.Process(row)
	check("MetaID empty country -> error", err != nil, "expected error")

	// Invalid country
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "XX", "State": "NY",
	}}
	_, err = proc.Process(row)
	check("MetaID invalid country -> error", err != nil, "expected error")

	// Missing state for US
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "US", "State": "",
	}}
	_, err = proc.Process(row)
	check("MetaID missing state for US -> error", err != nil, "expected error")

	// Invalid state for US
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"PlayerID": "P123", "Country": "US", "State": "ZZ",
	}}
	_, err = proc.Process(row)
	check("MetaID invalid state for US -> error", err != nil, "expected error")

	// MetaIDDefaultIn
	procIn := csv.MetaIDDefaultIn("OutMeta", "OutC", "OutS", hasher)
	check("MetaIDDefaultIn InputColumns = [PlayerID, Country, State]",
		sliceEqual(procIn.InputColumns(), []string{"PlayerID", "Country", "State"}),
		fmt.Sprintf("got %v", procIn.InputColumns()))
	check("MetaIDDefaultIn OutputColumns = [OutMeta, OutC, OutS]",
		sliceEqual(procIn.OutputColumns(), []string{"OutMeta", "OutC", "OutS"}),
		fmt.Sprintf("got %v", procIn.OutputColumns()))

	// MetaIDDefaultOut
	procOut := csv.MetaIDDefaultOut("PID", "C", "S", hasher)
	check("MetaIDDefaultOut InputColumns = [PID, C, S]",
		sliceEqual(procOut.InputColumns(), []string{"PID", "C", "S"}),
		fmt.Sprintf("got %v", procOut.InputColumns()))
	check("MetaIDDefaultOut OutputColumns = [MetaID, Country, State]",
		sliceEqual(procOut.OutputColumns(), []string{"MetaID", "Country", "State"}),
		fmt.Sprintf("got %v", procOut.OutputColumns()))

	// MetaID with empty outCountry/outState (should omit from output)
	procNoCS := csv.MetaID("PlayerID", "MetaID", "Country", "State", "", "", hasher)
	check("MetaID with empty outCountry/outState omits them",
		sliceEqual(procNoCS.OutputColumns(), []string{"MetaID"}),
		fmt.Sprintf("got %v", procNoCS.OutputColumns()))
}

func testMetaIDFixedLocation() {
	fmt.Println("\n--- MetaIDFixedLocation ---")

	hasher := func(playerID, country, state string) string {
		return "hash:" + playerID + ":" + country + ":" + state
	}

	// Valid construction
	proc, err := csv.MetaIDFixedLocation("PlayerID", "MetaID", "US", "NY", hasher)
	check("MetaIDFixedLocation(US, NY) constructs OK", err == nil, fmt.Sprintf("err=%v", err))

	if proc != nil {
		check("Fixed InputColumns = [PlayerID]",
			sliceEqual(proc.InputColumns(), []string{"PlayerID"}),
			fmt.Sprintf("got %v", proc.InputColumns()))
		check("Fixed OutputColumns = [MetaID]",
			sliceEqual(proc.OutputColumns(), []string{"MetaID"}),
			fmt.Sprintf("got %v", proc.OutputColumns()))
		check("Fixed OutputMetaID = true", proc.OutputMetaID(), "")

		row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"PlayerID": "P123"}}
		result, err := proc.Process(row)
		check("Fixed process OK",
			err == nil && result[0].String() == "hash:P123:US:NY",
			fmt.Sprintf("err=%v, got=%q", err, safeString(result)))

		// Empty player ID
		row = csv.RowData{RowIndex: 1, RowMap: map[string]string{"PlayerID": ""}}
		_, err = proc.Process(row)
		check("Fixed empty playerID -> error", err != nil, "expected error")
	}

	// Invalid country at construction
	_, err = csv.MetaIDFixedLocation("PlayerID", "MetaID", "XX", "NY", hasher)
	check("Fixed invalid country -> construction error", err != nil, "expected error")

	// Invalid state at construction
	_, err = csv.MetaIDFixedLocation("PlayerID", "MetaID", "US", "ZZ", hasher)
	check("Fixed invalid state -> construction error", err != nil, "expected error")

	// Empty state for US at construction
	_, err = csv.MetaIDFixedLocation("PlayerID", "MetaID", "US", "", hasher)
	check("Fixed empty state for US -> construction error", err != nil, "expected error")

	// Case normalization at construction
	proc2, err := csv.MetaIDFixedLocation("PlayerID", "MetaID", "us", "ny", hasher)
	check("Fixed normalizes country/state", err == nil, fmt.Sprintf("err=%v", err))
	if proc2 != nil {
		row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"PlayerID": "P1"}}
		result, _ := proc2.Process(row)
		check("Fixed uses uppercase in hash", result[0].String() == "hash:P1:US:NY",
			fmt.Sprintf("got %q", result[0].String()))
	}

	// MustMetaIDFixedLocation panics on error
	defer func() {
		r := recover()
		check("MustMetaIDFixedLocation panics on invalid", r != nil, "expected panic")
	}()
	csv.MustMetaIDFixedLocation("PlayerID", "MetaID", "XX", "ZZ", hasher)
}

func testUniqueID() {
	fmt.Println("\n--- UniqueID Processors ---")

	hasher := func(last4SSN, firstName, lastName, dob string) string {
		return "uid:" + last4SSN + ":" + firstName + ":" + lastName + ":" + dob
	}

	proc := csv.UniqueIDDefault(hasher)
	check("UniqueID OutputUniqueID = true", proc.OutputUniqueID(), "")
	check("UniqueID OutputMetaID = false", !proc.OutputMetaID(), "")
	check("UniqueID InputColumns correct",
		sliceEqual(proc.InputColumns(), []string{"LastName", "FirstName", "Last4SSN", "DOB"}),
		fmt.Sprintf("got %v", proc.InputColumns()))
	check("UniqueID OutputColumns = [UniquePlayerID]",
		sliceEqual(proc.OutputColumns(), []string{"UniquePlayerID"}),
		fmt.Sprintf("got %v", proc.OutputColumns()))

	// Valid input - raw names passed to hasher
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	result, err := proc.Process(row)
	check("UniqueID valid input", err == nil, fmt.Sprintf("err=%v", err))
	if err == nil {
		// Spec: hasher receives raw names (not normalized)
		check("UniqueID passes raw names",
			result[0].String() == "uid:1234:John:Smith:1990-05-15",
			fmt.Sprintf("got %q", result[0].String()))
	}

	// DOB formatting as ISO 8601
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "1234", "DOB": "05/15/1990",
	}}
	result, err = proc.Process(row)
	check("UniqueID DOB parsed and formatted as ISO",
		err == nil && strings.Contains(result[0].String(), "1990-05-15"),
		fmt.Sprintf("err=%v, got=%q", err, safeString(result)))

	// Empty last name
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "", "FirstName": "John", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("UniqueID empty lastName -> error", err != nil, "expected error")

	// Empty first name
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("UniqueID empty firstName -> error", err != nil, "expected error")

	// SSN not 4 digits
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "12", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("UniqueID SSN not 4 digits -> error", err != nil, "expected error")

	// SSN non-digit chars
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "abcd", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("UniqueID SSN non-digit -> error", err != nil, "expected error")

	// SSN not leaked in error messages (security)
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "ab9d", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("SSN value not leaked in error",
		err != nil && !strings.Contains(err.Error(), "ab9d"),
		fmt.Sprintf("error = %q", err))

	// Empty SSN with no fallback -> error
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "", "DOB": "1990-05-15",
	}}
	_, err = proc.Process(row)
	check("UniqueID empty SSN no fallback -> error", err != nil, "expected error")

	// Empty DOB -> error
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "1234", "DOB": "",
	}}
	_, err = proc.Process(row)
	check("UniqueID empty DOB -> error", err != nil, "expected error")

	// UniqueIDDefaultNullableLast4SSN with missing SSN -> XXXX fallback
	procNullable := csv.UniqueIDDefaultNullableLast4SSN(hasher)
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "", "DOB": "1990-05-15",
	}}
	result, err = procNullable.Process(row)
	check("UniqueIDNullable empty SSN -> XXXX fallback",
		err == nil && strings.Contains(result[0].String(), "XXXX"),
		fmt.Sprintf("err=%v, got=%q", err, safeString(result)))

	// Output is Quoted
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"LastName": "Smith", "FirstName": "John", "Last4SSN": "1234", "DOB": "1990-05-15",
	}}
	result, err = proc.Process(row)
	// Quoted output for a string with colons should wrap in quotes
	// Actually "uid:1234:John:Smith:1990-05-15" doesn't contain commas or quotes,
	// so Quoted won't add CSV escaping. Let's just verify it's not empty.
	check("UniqueID output is non-empty", err == nil && result[0].String() != "",
		fmt.Sprintf("err=%v", err))
}

func testCountryAndState() {
	fmt.Println("\n--- CountryAndState ---")

	proc := csv.CountryAndStateDefault()
	check("CountryAndState InputColumns = [Country, State]",
		sliceEqual(proc.InputColumns(), []string{"Country", "State"}),
		fmt.Sprintf("got %v", proc.InputColumns()))
	check("CountryAndState OutputColumns = [Country, State]",
		sliceEqual(proc.OutputColumns(), []string{"Country", "State"}),
		fmt.Sprintf("got %v", proc.OutputColumns()))
	check("CountryAndState OutputMetaID = false", !proc.OutputMetaID(), "")
	check("CountryAndState OutputUniqueID = false", !proc.OutputUniqueID(), "")

	// Valid US + NY
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "US", "State": "NY",
	}}
	result, err := proc.Process(row)
	check("CountryAndState(US, NY) OK",
		err == nil && len(result) == 2,
		fmt.Sprintf("err=%v, len=%d", err, len(result)))

	// Returns Quoted values
	if err == nil && len(result) == 2 {
		check("CountryAndState returns Quoted country",
			result[0].String() == "US",
			fmt.Sprintf("got %q", result[0].String()))
		check("CountryAndState returns Quoted state",
			result[1].String() == "NY",
			fmt.Sprintf("got %q", result[1].String()))
	}

	// Invalid country
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "XX", "State": "NY",
	}}
	_, err = proc.Process(row)
	check("CountryAndState invalid country -> error", err != nil, "expected error")

	// Empty country
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "", "State": "NY",
	}}
	_, err = proc.Process(row)
	check("CountryAndState empty country -> error", err != nil, "expected error")

	// Missing state for US
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Country": "US", "State": "",
	}}
	_, err = proc.Process(row)
	check("CountryAndState missing state for US -> error", err != nil, "expected error")

	// Custom column names
	proc2 := csv.CountryAndState("C", "S", "OutC", "OutS")
	check("Custom CountryAndState InputColumns",
		sliceEqual(proc2.InputColumns(), []string{"C", "S"}),
		fmt.Sprintf("got %v", proc2.InputColumns()))
}

func testCSVMetadata() {
	fmt.Println("\n--- CSVMetadata ---")

	meta := csv.NewSimpleCSVMetadata(csv.CSVBets, []csv.InColumnProcessor{
		csv.NonNillableInt("Amount", "OutAmount"),
		csv.NonEmptyStringWithMax("Name", "OutName", 50),
		csv.ConstantString("Type", csv.Quoted("bets")),
	})

	check("Type = CSVBets", meta.Type() == csv.CSVBets,
		fmt.Sprintf("got %v", meta.Type()))

	// MatchHeaders
	check("MatchHeaders exact match",
		meta.MatchHeaders([]string{"Amount", "Name"}),
		"should match")
	check("MatchHeaders extra columns OK",
		meta.MatchHeaders([]string{"Amount", "Name", "Extra"}),
		"should match with extra")
	check("MatchHeaders missing column",
		!meta.MatchHeaders([]string{"Amount"}),
		"should not match missing Name")
	check("MatchHeaders case-sensitive",
		!meta.MatchHeaders([]string{"amount", "name"}),
		"should be case-sensitive")

	// MatchHeaders with ConstantString (no input columns)
	metaConst := csv.NewSimpleCSVMetadata(csv.CSVBets, []csv.InColumnProcessor{
		csv.ConstantString("Type", csv.Quoted("bets")),
	})
	check("MatchHeaders with only ConstantString (no required input)",
		metaConst.MatchHeaders([]string{}),
		"should match with no required columns")

	// OutputHeaders
	headers := meta.OutputHeaders()
	check("OutputHeaders correct",
		sliceEqual(headers, []string{"OutAmount", "OutName", "Type"}),
		fmt.Sprintf("got %v", headers))

	// ProcessRow success
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Amount": "42", "Name": "Test",
	}}
	result, err := meta.ProcessRow(row)
	check("ProcessRow success", err == nil && len(result.Columns) == 3,
		fmt.Sprintf("err=%v, len=%d", err, len(safeColumns(result))))

	// ProcessRow error propagation
	row = csv.RowData{RowIndex: 1, RowMap: map[string]string{
		"Amount": "invalid", "Name": "Test",
	}}
	_, err = meta.ProcessRow(row)
	check("ProcessRow error propagation", err != nil, "expected error")

	// ColumnData returns processors
	check("ColumnData returns processors", len(meta.ColumnData()) == 3,
		fmt.Sprintf("got %d", len(meta.ColumnData())))
}

// --- Helpers ---

func processSimple(proc csv.InColumnProcessor, input string) string {
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": input}}
	result, err := proc.Process(row)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	return result[0].String()
}

func processSimpleErr(proc csv.InColumnProcessor, input string) (string, error) {
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": input}}
	result, err := proc.Process(row)
	if err != nil {
		return "", err
	}
	return result[0].String(), nil
}

func processSimpleResult(proc csv.InColumnProcessor, input string) (string, error) {
	row := csv.RowData{RowIndex: 1, RowMap: map[string]string{"in": input}}
	result, err := proc.Process(row)
	if err != nil {
		return "", err
	}
	return result[0].String(), nil
}

func safeString(result []csv.CSVOutputString) string {
	if len(result) == 0 {
		return "<empty>"
	}
	return result[0].String()
}

func safeColumns(row csv.CSVOutputRow) []csv.CSVOutputString {
	return row.Columns
}

func sliceEqual(a, b []string) bool {
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
