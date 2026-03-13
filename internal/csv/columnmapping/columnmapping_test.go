package columnmapping

import (
	"testing"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// testHashers is a dummy implementation of hashers.Hashers for testing.
type testHashers struct{}

func (t *testHashers) PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string {
	return "uid:" + last4SSN + ":" + firstName + ":" + lastName + ":" + dob
}

func (t *testHashers) OrganizationPlayerIDHasher(playerID, country, state string) string {
	return "meta:" + playerID + ":" + country + ":" + state
}

func (t *testHashers) SaveDB() error { return nil }

func buildTestMetadata() []csv.CSVMetadata {
	h := &testHashers{}
	operatorID := csv.Quoted("test-operator")
	return BuildAllMetadata(h, operatorID)
}

// requiredHeaders returns the required input headers for each CSV type.
func requiredHeaders(ct csv.CSVType) []string {
	switch ct {
	case csv.CSVPlayers:
		return []string{"LastName", "FirstName", "Last4SSN", "DOB", "OrganizationPlayerID", "OrganizationCountry", "OrganizationState"}
	case csv.CSVBets:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"StartDate", "CouponNumber", "Currency", "DateOfLastTransaction", "DateOfPlacementWager",
			"EventRawDescription", "EventSport", "EventLeague", "EventHomeTeam", "EventScoreHomeTeam",
			"EventAwayTeam", "EventScoreAwayTeam", "EventWinningTeam", "EventDate", "EventClosed",
			"Ticket", "TicketsCanceled", "TicketsFailed", "TicketsResettled", "TicketsSettled",
			"TicketsSold", "TicketsVoided", "WagerRawDescription", "WagerType", "WagerDescription",
			"WagerOdds", "WagerOddsDate", "WagerOddsBookmaker"}
	case csv.CSVBonus:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"BonusDate", "CashableAmount", "NonCashableAmount", "ForfeitedAmount"}
	case csv.CSVCasino:
		return []string{"Casino_Player_Id", "Casino_Country", "Casino_State",
			"Casino_ID", "Accounting_Dt", "Gaming_Type_ID", "Start_Dttm", "End_Dttm",
			"MachineNumber", "Slot_Games_Played_Cnt", "SlotCoinIn", "SlotCoinOut",
			"Slot_Jackpot_Amt", "Slot_Actl_Player_Win_Amt", "Slot_Theo_Player_Win_Amt",
			"Slot_Tm_Played_Second_Cnt", "Slot_Xtra_Credit_Used_Amt", "Slot_Xtra_Credit_PTP_Earn_Amt",
			"Slot_Avg_Bet_Amt", "Slot_Total_Comp_Earn_Amt", "Slot_PTP_Slot_Play_Used_Amt",
			"Slot_Points_Multp_Cnt", "Slot_Points_Multpd_Cnt", "Slot_Exp_Comp_Earn_Amt",
			"Slot_Ranked_Point_Multpd_Cnt", "Abandoned_Card_Ind", "Manual_Edit_Ind",
			"Table_Game_Cd", "Table_Games_Played_Cnt", "Table_Chips_In_Amt", "Table_Chips_Out_Amt",
			"Table_Tm_Played_Second_Cnt", "Table_Avg_Bet_Amt", "Table_Actl_Player_Win_Amt",
			"Table_Theo_Player_Win_Amt", "Table_Total_Comp_Earn_Amt", "Table_Cash_Buy_In_Amt",
			"Table_Non_Cash_Buy_In_Amt", "Table_Exp_Comp_Earn_Amt", "Session_Base_Point_Cnt",
			"TransID", "TableDropNetMarkers"}
	case csv.CSVCasinoPlayers:
		return []string{"Casino_Player_Id", "Player_First_Name", "Player_Last_Name",
			"Player_Last_4SSN", "Player_DOB", "Casino_Country", "Casino_State",
			"Casino_ID", "Gender", "Zip_Cd", "City_Cd", "State_Cd", "Country_Id",
			"Tier_ID", "Tier_Name", "Enrolled_Date", "Address_Change"}
	case csv.CSVCasinoParSheet:
		return []string{"Machine_ID", "MCH_Casino_ID", "MCH_Date",
			"Number_ReelsLinesScatter", "Min_Wager", "Max_Wager", "Symbols_Per_Reel",
			"PaybackPCT", "Hit_FrequencyPCT", "Plays_Per_Jackpot", "Jackpot_Amount",
			"Plays_Per_Bonus", "Volatility_Index"}
	case csv.CSVComplaints:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"ComplaintDate", "Method", "Subject"}
	case csv.CSVDemographic:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"BirthYear", "Gender", "Country", "State", "AccountOpenDate", "Operator"}
	case csv.CSVDepositsWithdrawals:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"Type", "Date", "Amount", "Currency", "Success", "Method"}
	case csv.CSVResponsibleGaming:
		return []string{"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
			"LimitCreateDate", "Type", "Period", "Unit", "Purpose", "Value"}
	default:
		return nil
	}
}

func TestDetectCSVType_ExactHeaders(t *testing.T) {
	allMeta := buildTestMetadata()

	for _, ct := range csv.AllCSVTypes() {
		t.Run(ct.String(), func(t *testing.T) {
			headers := requiredHeaders(ct)
			if headers == nil {
				t.Fatalf("no required headers defined for %s", ct)
			}
			meta, err := DetectCSVType(headers, allMeta)
			if err != nil {
				t.Fatalf("DetectCSVType(%s) error: %v", ct, err)
			}
			if meta.Type() != ct {
				t.Errorf("DetectCSVType(%s) = %s, want %s", ct, meta.Type(), ct)
			}
		})
	}
}

func TestDetectCSVType_ExtraColumns(t *testing.T) {
	allMeta := buildTestMetadata()

	for _, ct := range csv.AllCSVTypes() {
		t.Run(ct.String(), func(t *testing.T) {
			headers := append(requiredHeaders(ct), "ExtraColumn1", "ExtraColumn2")
			meta, err := DetectCSVType(headers, allMeta)
			if err != nil {
				t.Fatalf("DetectCSVType with extra columns error: %v", err)
			}
			if meta.Type() != ct {
				t.Errorf("DetectCSVType with extra columns = %s, want %s", meta.Type(), ct)
			}
		})
	}
}

func TestDetectCSVType_MissingRequiredColumn(t *testing.T) {
	allMeta := buildTestMetadata()

	for _, ct := range csv.AllCSVTypes() {
		t.Run(ct.String(), func(t *testing.T) {
			headers := requiredHeaders(ct)
			if len(headers) == 0 {
				t.Skip("no headers to remove")
			}
			// Remove the last required column
			incomplete := headers[:len(headers)-1]
			_, err := DetectCSVType(incomplete, allMeta)
			if err == nil {
				t.Errorf("DetectCSVType with missing column should return error for %s", ct)
			}
		})
	}
}

func TestDetectCSVType_NoMatch(t *testing.T) {
	allMeta := buildTestMetadata()

	headers := []string{"CompletelyUnrelated", "Headers", "Here"}
	_, err := DetectCSVType(headers, allMeta)
	if err == nil {
		t.Error("DetectCSVType with unrelated headers should return error")
	}
}

func TestDetectCSVType_EmptyHeaders(t *testing.T) {
	allMeta := buildTestMetadata()

	_, err := DetectCSVType([]string{}, allMeta)
	if err == nil {
		t.Error("DetectCSVType with empty headers should return error")
	}
}

func TestBuildAllMetadata_Returns10Types(t *testing.T) {
	allMeta := buildTestMetadata()
	if len(allMeta) != 10 {
		t.Errorf("BuildAllMetadata returned %d types, want 10", len(allMeta))
	}

	// Verify all 10 types are represented
	typeSet := make(map[csv.CSVType]bool)
	for _, m := range allMeta {
		typeSet[m.Type()] = true
	}
	for _, ct := range csv.AllCSVTypes() {
		if !typeSet[ct] {
			t.Errorf("missing CSV type %s in BuildAllMetadata result", ct)
		}
	}
}

func TestCasinoParSheet_NoPlayerIdentification(t *testing.T) {
	allMeta := buildTestMetadata()

	var parSheet csv.CSVMetadata
	for _, m := range allMeta {
		if m.Type() == csv.CSVCasinoParSheet {
			parSheet = m
			break
		}
	}
	if parSheet == nil {
		t.Fatal("Casino Par Sheet metadata not found")
	}

	for _, proc := range parSheet.ColumnData() {
		if proc.OutputMetaID() {
			t.Error("Casino Par Sheet should not output MetaID")
		}
		if proc.OutputUniqueID() {
			t.Error("Casino Par Sheet should not output UniqueID")
		}
	}

	// Verify no OrgID in output
	for _, h := range parSheet.OutputHeaders() {
		if h == "OrgID" {
			t.Error("Casino Par Sheet should not have OrgID output column")
		}
		if h == "MetaID" {
			t.Error("Casino Par Sheet should not have MetaID output column")
		}
	}
}

func TestCasinoTypes_UseCustomColumnNames(t *testing.T) {
	allMeta := buildTestMetadata()

	for _, m := range allMeta {
		if m.Type() != csv.CSVCasino && m.Type() != csv.CSVCasinoPlayers {
			continue
		}
		t.Run(m.Type().String(), func(t *testing.T) {
			// These types should require Casino_Player_Id, not OrganizationPlayerID
			headers := requiredHeaders(m.Type())
			hasCasinoPlayerID := false
			hasOrgPlayerID := false
			for _, h := range headers {
				if h == "Casino_Player_Id" {
					hasCasinoPlayerID = true
				}
				if h == "OrganizationPlayerID" {
					hasOrgPlayerID = true
				}
			}
			if !hasCasinoPlayerID {
				t.Error("expected Casino_Player_Id in required headers")
			}
			if hasOrgPlayerID {
				t.Error("should not have OrganizationPlayerID in required headers")
			}
		})
	}
}
