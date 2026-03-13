#!/usr/bin/env bash
# Spec 06: CSV Type Definitions — test harness
# Tests: column mapping registration, auto-detection, processor counts,
#        Casino custom columns, Casino Players XXXX fallback,
#        Casino Par Sheet no player ID, Demographic dual country/state,
#        spec compliance of processor types and column names.

set -uo pipefail
cd "$(dirname "$0")/.."

PASS=0
FAIL=0

check() {
    local desc="$1" result="$2" evidence="${3:-}"
    if [[ "$result" == "true" ]]; then
        echo "  PASS: $desc"
        ((PASS++))
    else
        echo "  FAIL: $desc -- $evidence"
        ((FAIL++))
    fi
}

# Count processor constructors in a function body (excludes NewSimpleCSVMetadata and func signature lines)
count_procs() {
    local fn="$1"
    sed -n "/func ${fn}(/,/^}/p" "$CM" | grep -v 'NewSimpleCSVMetadata' | grep -v "^func " | grep -c 'csv\.' || true
}

CM="internal/csv/columnmapping/columnmapping.go"
CM_TEST="internal/csv/columnmapping/columnmapping_test.go"
PROC="internal/csv/processors.go"

echo "=== Spec 06: CSV Type Definitions ==="
echo ""

# --- 1. Package Structure ---
echo "--- Package Structure ---"
check "columnmapping.go exists" "$(test -f "$CM" && echo true || echo false)"
check "columnmapping_test.go exists" "$(test -f "$CM_TEST" && echo true || echo false)"
check "Package is columnmapping" "$(head -1 "$CM" | grep -q 'package columnmapping' && echo true || echo false)"

# --- 2. BuildAllMetadata ---
echo ""
echo "--- BuildAllMetadata ---"
check "BuildAllMetadata function exists" "$(grep -q 'func BuildAllMetadata(' "$CM" && echo true || echo false)"
check "BuildAllMetadata takes hashers.Hashers" "$(grep 'func BuildAllMetadata(' "$CM" | grep -q 'hashers.Hashers' && echo true || echo false)"
check "BuildAllMetadata takes operatorID" "$(grep 'func BuildAllMetadata(' "$CM" | grep -q 'operatorID' && echo true || echo false)"
check "BuildAllMetadata returns []csv.CSVMetadata" "$(grep 'func BuildAllMetadata(' "$CM" | grep -q '\[\]csv.CSVMetadata' && echo true || echo false)"
# Count lowercase build functions (private builders)
BUILD_COUNT=$(grep -c '^func build[A-Z]' "$CM" || true)
check "Exactly 10 private build functions" "$([ "$BUILD_COUNT" -eq 10 ] && echo true || echo false)" "found $BUILD_COUNT"

# --- 3. DetectCSVType ---
echo ""
echo "--- DetectCSVType ---"
check "DetectCSVType function exists" "$(grep -q 'func DetectCSVType(' "$CM" && echo true || echo false)"
check "DetectCSVType takes headers []string" "$(grep 'func DetectCSVType(' "$CM" | grep -q 'headers \[\]string' && echo true || echo false)"
check "DetectCSVType takes allMetadata" "$(grep 'func DetectCSVType(' "$CM" | grep -q 'allMetadata' && echo true || echo false)"
check "DetectCSVType returns error for no match" "$(grep -q 'no CSV type matched the headers' "$CM" && echo true || echo false)"
check "DetectCSVType returns error for ambiguous" "$(grep -q 'multiple CSV types matched' "$CM" && echo true || echo false)"

# --- 4. Players (CSVPlayers = 3) ---
echo ""
echo "--- Players Type ---"
check "buildPlayers exists" "$(grep -q 'func buildPlayers(' "$CM" && echo true || echo false)"
check "Players uses UniqueIDDefault" "$(sed -n '/func buildPlayers/,/^}/p' "$CM" | grep -q 'csv.UniqueIDDefault' && echo true || echo false)"
check "Players uses MetaIDDefault" "$(sed -n '/func buildPlayers/,/^}/p' "$CM" | grep -q 'csv.MetaIDDefault' && echo true || echo false)"
check "Players uses ConstantString OrgID" "$(sed -n '/func buildPlayers/,/^}/p' "$CM" | grep -q 'csv.ConstantString("OrgID"' && echo true || echo false)"
P_COUNT=$(count_procs buildPlayers)
check "Players has exactly 3 processors" "$([ "$P_COUNT" -eq 3 ] && echo true || echo false)" "found $P_COUNT"

# --- 5. Bets (CSVBets = 2) ---
echo ""
echo "--- Bets Type ---"
check "buildBets exists" "$(grep -q 'func buildBets(' "$CM" && echo true || echo false)"
check "Bets uses MetaIDDefault" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'csv.MetaIDDefault' && echo true || echo false)"
check "Bets has ConstantString OrgID" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'csv.ConstantString("OrgID"' && echo true || echo false)"
check "Bets has StartDate DateAndTimeNonZero" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'csv.DateAndTimeNonZeroAndNotAfterNow("StartDate"' && echo true || echo false)"
check "Bets has Currency maxLen 3" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Currency", "Currency", 3)' && echo true || echo false)"
check "Bets has CouponNumber maxLen 256" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("CouponNumber", "CouponNumber", 256)' && echo true || echo false)"
check "Bets has EventRawDescription maxLen 1024" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NillableStringWithMax("EventRawDescription", "EventRawDescription", 1024)' && echo true || echo false)"
check "Bets has EventScoreHomeTeam NillableNonNegInt" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NillableNonNegInt("EventScoreHomeTeam"' && echo true || echo false)"
check "Bets has NillableFlexBool EventClosed" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NillableFlexBool("EventClosed"' && echo true || echo false)"
check "Bets has TicketsCanceled NonNilFloat64Full" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NonNilFloat64Full("TicketsCanceled"' && echo true || echo false)"
check "Bets has NillableNonNegFloat64Full WagerOdds" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NillableNonNegFloat64Full("WagerOdds"' && echo true || echo false)"
check "Bets has WagerOddsBookmaker maxLen 256" "$(sed -n '/func buildBets/,/^}/p' "$CM" | grep -q 'NillableStringWithMax("WagerOddsBookmaker", "WagerOddsBookmaker", 256)' && echo true || echo false)"
BETS_COUNT=$(count_procs buildBets)
check "Bets has exactly 30 processors" "$([ "$BETS_COUNT" -eq 30 ] && echo true || echo false)" "found $BETS_COUNT"

# --- 6. Bonus (CSVBonus = 5) ---
echo ""
echo "--- Bonus Type ---"
check "buildBonus exists" "$(grep -q 'func buildBonus(' "$CM" && echo true || echo false)"
check "Bonus maps BonusDate to Date" "$(sed -n '/func buildBonus/,/^}/p' "$CM" | grep -q 'DateAndTimeNonZeroAndNotAfterNow("BonusDate", "Date")' && echo true || echo false)"
check "Bonus CashableAmount NillableNonNegFloat64Full" "$(sed -n '/func buildBonus/,/^}/p' "$CM" | grep -q 'NillableNonNegFloat64Full("CashableAmount"' && echo true || echo false)"
BONUS_COUNT=$(count_procs buildBonus)
check "Bonus has exactly 6 processors" "$([ "$BONUS_COUNT" -eq 6 ] && echo true || echo false)" "found $BONUS_COUNT"

# --- 7. Casino (CSVCasino = 6) ---
echo ""
echo "--- Casino Type ---"
check "buildCasino exists" "$(grep -q 'func buildCasino(' "$CM" && echo true || echo false)"
check "Casino uses custom MetaID (not Default)" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'csv.MetaID("Casino_Player_Id"' && echo true || echo false)"
check "Casino MetaID outputs Casino_Country" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep 'csv.MetaID' | grep -q '"Casino_Country"' && echo true || echo false)"
check "Casino MetaID outputs Casino_State" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep 'csv.MetaID' | grep -q '"Casino_State"' && echo true || echo false)"
check "Casino has NO ConstantString OrgID" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'ConstantString' && echo false || echo true)"
check "Casino has Casino_ID NonNillableNonNegInt" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NonNillableNonNegInt("Casino_ID"' && echo true || echo false)"
check "Casino has Accounting_Dt NonNillDate" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NonNillDate("Accounting_Dt"' && echo true || echo false)"
check "Casino has Start_Dttm NonNillableHHMMSSTime" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NonNillableHHMMSSTime("Start_Dttm"' && echo true || echo false)"
check "Casino has End_Dttm NonNillableHHMMSSTime" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NonNillableHHMMSSTime("End_Dttm"' && echo true || echo false)"
check "Casino has MachineNumber maxLen 25" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NillableStringWithMax("MachineNumber", "MachineNumber", 25)' && echo true || echo false)"
check "Casino has TransID NonNillableNonNegInt" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NonNillableNonNegInt("TransID"' && echo true || echo false)"
check "Casino has Abandoned_Card_Ind NillableFlexBool" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NillableFlexBool("Abandoned_Card_Ind"' && echo true || echo false)"
check "Casino has Manual_Edit_Ind NillableFlexBool" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NillableFlexBool("Manual_Edit_Ind"' && echo true || echo false)"
check "Casino has Table_Game_Cd maxLen 256" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NillableStringWithMax("Table_Game_Cd", "Table_Game_Cd", 256)' && echo true || echo false)"
check "Casino has TableDropNetMarkers NillableNonNegInt" "$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -q 'NillableNonNegInt("TableDropNetMarkers"' && echo true || echo false)"
# Use the exact function name with ( to avoid matching buildCasinoPlayers/buildCasinoParSheet
CASINO_COUNT=$(sed -n '/func buildCasino(/,/^}/p' "$CM" | grep -v 'NewSimpleCSVMetadata' | grep -v "^func " | grep -c 'csv\.' || true)
check "Casino has exactly 40 processors" "$([ "$CASINO_COUNT" -eq 40 ] && echo true || echo false)" "found $CASINO_COUNT"

# --- 8. Casino Players (CSVCasinoPlayers = 4) ---
echo ""
echo "--- Casino Players Type ---"
check "buildCasinoPlayers exists" "$(grep -q 'func buildCasinoPlayers(' "$CM" && echo true || echo false)"
check "Casino Players uses UniqueID (not Default)" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'csv.UniqueID("Player_Last_Name"' && echo true || echo false)"
check "Casino Players has XXXX SSN fallback" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q '"XXXX"' && echo true || echo false)"
check "Casino Players outputs Player_Id" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q '"Player_Id"' && echo true || echo false)"
check "Casino Players uses custom MetaID" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'csv.MetaID("Casino_Player_Id"' && echo true || echo false)"
check "Casino Players has NO ConstantString OrgID" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'ConstantString' && echo false || echo true)"
check "Casino Players has Gender maxLen 1" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Gender", "Gender", 1)' && echo true || echo false)"
check "Casino Players has Zip_Cd maxLen 16" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Zip_Cd", "Zip_Cd", 16)' && echo true || echo false)"
check "Casino Players has City_Cd maxLen 64" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("City_Cd", "City_Cd", 64)' && echo true || echo false)"
check "Casino Players has State_Cd maxLen 3" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("State_Cd", "State_Cd", 3)' && echo true || echo false)"
check "Casino Players has Country_Id maxLen 50" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Country_Id", "Country_Id", 50)' && echo true || echo false)"
check "Casino Players has Tier_Name maxLen 50" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Tier_Name", "Tier_Name", 50)' && echo true || echo false)"
check "Casino Players has Enrolled_Date NonNillDate" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NonNillDate("Enrolled_Date"' && echo true || echo false)"
check "Casino Players has Address_Change NillableNonNegInt" "$(sed -n '/func buildCasinoPlayers/,/^}/p' "$CM" | grep -q 'NillableNonNegInt("Address_Change"' && echo true || echo false)"
CP_COUNT=$(count_procs buildCasinoPlayers)
check "Casino Players has exactly 12 processors" "$([ "$CP_COUNT" -eq 12 ] && echo true || echo false)" "found $CP_COUNT"

# --- 9. Casino Par Sheet (CSVCasinoParSheet = 7) ---
echo ""
echo "--- Casino Par Sheet Type ---"
check "buildCasinoParSheet exists" "$(grep -q 'func buildCasinoParSheet(' "$CM" && echo true || echo false)"
check "Par Sheet takes no hashers parameter" "$(grep 'func buildCasinoParSheet(' "$CM" | grep -q 'hashers\|operatorID' && echo false || echo true)"
check "Par Sheet has NO MetaID" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'MetaID' && echo false || echo true)"
check "Par Sheet has NO UniqueID" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'UniqueID' && echo false || echo true)"
check "Par Sheet has NO ConstantString" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'ConstantString' && echo false || echo true)"
check "Par Sheet has Machine_ID maxLen 25" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Machine_ID", "Machine_ID", 25)' && echo true || echo false)"
check "Par Sheet has MCH_Casino_ID NonNillableNonNegInt" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'NonNillableNonNegInt("MCH_Casino_ID"' && echo true || echo false)"
check "Par Sheet has MCH_Date NonNillMMDDYYYY" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'NonNillMMDDYYYY("MCH_Date"' && echo true || echo false)"
check "Par Sheet has Volatility_Index NillableFloat64Full" "$(sed -n '/func buildCasinoParSheet/,/^}/p' "$CM" | grep -q 'NillableFloat64Full("Volatility_Index"' && echo true || echo false)"
PS_COUNT=$(count_procs buildCasinoParSheet)
check "Par Sheet has exactly 13 processors" "$([ "$PS_COUNT" -eq 13 ] && echo true || echo false)" "found $PS_COUNT"

# --- 10. Complaints (CSVComplaints = 8) ---
echo ""
echo "--- Complaints Type ---"
check "buildComplaints exists" "$(grep -q 'func buildComplaints(' "$CM" && echo true || echo false)"
check "Complaints maps ComplaintDate to Date" "$(sed -n '/func buildComplaints/,/^}/p' "$CM" | grep -q 'DateAndTimeNonZeroAndNotAfterNow("ComplaintDate", "Date")' && echo true || echo false)"
check "Complaints has Method maxLen 256" "$(sed -n '/func buildComplaints/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Method", "Method", 256)' && echo true || echo false)"
check "Complaints has Subject maxLen 256" "$(sed -n '/func buildComplaints/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Subject", "Subject", 256)' && echo true || echo false)"
CO_COUNT=$(count_procs buildComplaints)
check "Complaints has exactly 5 processors" "$([ "$CO_COUNT" -eq 5 ] && echo true || echo false)" "found $CO_COUNT"

# --- 11. Demographic (CSVDemographic = 9) ---
echo ""
echo "--- Demographic Type ---"
check "buildDemographic exists" "$(grep -q 'func buildDemographic(' "$CM" && echo true || echo false)"
check "Demographic has MetaIDDefault" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'csv.MetaIDDefault' && echo true || echo false)"
check "Demographic has BirthYear NonNilBirthYear" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'NonNilBirthYear("BirthYear"' && echo true || echo false)"
check "Demographic has Gender maxLen 256" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Gender", "Gender", 256)' && echo true || echo false)"
check "Demographic has CountryAndState (separate from MetaID)" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'CountryAndState("Country", "State"' && echo true || echo false)"
check "Demographic maps AccountOpenDate to Date" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'DateAndTimeNonZeroAndNotAfterNow("AccountOpenDate", "Date")' && echo true || echo false)"
check "Demographic has Operator maxLen 256" "$(sed -n '/func buildDemographic/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Operator", "Operator", 256)' && echo true || echo false)"
DE_COUNT=$(count_procs buildDemographic)
check "Demographic has exactly 7 processors" "$([ "$DE_COUNT" -eq 7 ] && echo true || echo false)" "found $DE_COUNT"

# --- 12. Deposits/Withdrawals (CSVDepositsWithdrawals = 10) ---
echo ""
echo "--- Deposits/Withdrawals Type ---"
check "buildDepositsWithdrawals exists" "$(grep -q 'func buildDepositsWithdrawals(' "$CM" && echo true || echo false)"
check "DepWith has Amount NonNilNonNegFloat64Full" "$(sed -n '/func buildDepositsWithdrawals/,/^}/p' "$CM" | grep -q 'NonNilNonNegFloat64Full("Amount"' && echo true || echo false)"
check "DepWith has Currency maxLen 256" "$(sed -n '/func buildDepositsWithdrawals/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Currency", "Currency", 256)' && echo true || echo false)"
check "DepWith has Success NonNilFlexBool" "$(sed -n '/func buildDepositsWithdrawals/,/^}/p' "$CM" | grep -q 'NonNilFlexBool("Success"' && echo true || echo false)"
DW_COUNT=$(count_procs buildDepositsWithdrawals)
check "DepWith has exactly 8 processors" "$([ "$DW_COUNT" -eq 8 ] && echo true || echo false)" "found $DW_COUNT"

# --- 13. Responsible Gaming (CSVResponsibleGaming = 11) ---
echo ""
echo "--- Responsible Gaming Type ---"
check "buildResponsibleGaming exists" "$(grep -q 'func buildResponsibleGaming(' "$CM" && echo true || echo false)"
check "RG has LimitCreateDate DateAndTimeNonZero" "$(sed -n '/func buildResponsibleGaming/,/^}/p' "$CM" | grep -q 'DateAndTimeNonZeroAndNotAfterNow("LimitCreateDate"' && echo true || echo false)"
check "RG has Value NonNilFloat64Full" "$(sed -n '/func buildResponsibleGaming/,/^}/p' "$CM" | grep -q 'NonNilFloat64Full("Value"' && echo true || echo false)"
check "RG has Purpose maxLen 256" "$(sed -n '/func buildResponsibleGaming/,/^}/p' "$CM" | grep -q 'NonEmptyStringWithMax("Purpose", "Purpose", 256)' && echo true || echo false)"
RG_COUNT=$(count_procs buildResponsibleGaming)
check "RG has exactly 8 processors" "$([ "$RG_COUNT" -eq 8 ] && echo true || echo false)" "found $RG_COUNT"

# --- 14. Spec Compliance: Casino types have NO OrgID ---
echo ""
echo "--- Spec Compliance: No OrgID for Casino Types ---"
check "buildCasino has no operatorID param" "$(grep 'func buildCasino(' "$CM" | grep -q 'operatorID' && echo false || echo true)"
check "buildCasinoPlayers has no operatorID param" "$(grep 'func buildCasinoPlayers(' "$CM" | grep -q 'operatorID' && echo false || echo true)"
check "buildCasinoParSheet has no operatorID param" "$(grep 'func buildCasinoParSheet(' "$CM" | grep -q 'operatorID' && echo false || echo true)"

# --- 15. Spec Compliance: Standard types all have OrgID ---
echo ""
echo "--- Spec Compliance: OrgID for Standard Types ---"
for fn in buildPlayers buildBets buildBonus buildComplaints buildDemographic buildDepositsWithdrawals buildResponsibleGaming; do
    check "$fn has ConstantString OrgID" "$(sed -n "/func $fn/,/^}/p" "$CM" | grep -q 'ConstantString("OrgID"' && echo true || echo false)"
done

# --- 16. Test Coverage ---
echo ""
echo "--- Test Coverage ---"
check "Test: exact headers for all types" "$(grep -q 'TestDetectCSVType_ExactHeaders' "$CM_TEST" && echo true || echo false)"
check "Test: extra columns" "$(grep -q 'TestDetectCSVType_ExtraColumns' "$CM_TEST" && echo true || echo false)"
check "Test: missing required column" "$(grep -q 'TestDetectCSVType_MissingRequiredColumn' "$CM_TEST" && echo true || echo false)"
check "Test: no match" "$(grep -q 'TestDetectCSVType_NoMatch' "$CM_TEST" && echo true || echo false)"
check "Test: empty headers" "$(grep -q 'TestDetectCSVType_EmptyHeaders' "$CM_TEST" && echo true || echo false)"
check "Test: 10 types returned" "$(grep -q 'TestBuildAllMetadata_Returns10Types' "$CM_TEST" && echo true || echo false)"
check "Test: par sheet no player identification" "$(grep -q 'TestCasinoParSheet_NoPlayerIdentification' "$CM_TEST" && echo true || echo false)"
check "Test: casino custom column names" "$(grep -q 'TestCasinoTypes_UseCustomColumnNames' "$CM_TEST" && echo true || echo false)"
check "Test: row processing integration" "$(grep -q 'TestRowProcessingIntegration' "$CM_TEST" && echo true || echo false)"
check "Test: hasher parameter order" "$(grep -q 'TestRowProcessingHasherParameterOrder' "$CM_TEST" && echo true || echo false)"

# --- 17. Missing Column Tests: first/middle/last ---
echo ""
echo "--- Missing Column Tests ---"
check "Test removes last column" "$(grep -q 'headers\[:len(headers)-1\]' "$CM_TEST" && echo true || echo false)"
check "Test removes first column" "$(grep -q 'headers\[1:\]' "$CM_TEST" && echo true || echo false)"
check "Test removes middle column" "$(grep -q 'mid := len(headers) / 2' "$CM_TEST" && echo true || echo false)"

# --- 18. Run Go tests ---
echo ""
echo "--- Go Test Execution ---"
TEST_OUTPUT=$(go test -race -count=1 ./internal/csv/columnmapping/ 2>&1)
TEST_EXIT=$?
check "All columnmapping tests pass" "$([ $TEST_EXIT -eq 0 ] && echo true || echo false)" "$TEST_OUTPUT"

VERBOSE_OUTPUT=$(go test -race -count=1 -v ./internal/csv/columnmapping/ 2>&1)
SUBTEST_COUNT=$(echo "$VERBOSE_OUTPUT" | grep -c -- '--- PASS:' || true)
check "At least 15 test cases pass" "$([ "$SUBTEST_COUNT" -ge 15 ] && echo true || echo false)" "$SUBTEST_COUNT subtests passed"

check "No data races detected" "$(echo "$VERBOSE_OUTPUT" | grep -q 'DATA RACE' && echo false || echo true)"

echo ""
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"

exit $FAIL
