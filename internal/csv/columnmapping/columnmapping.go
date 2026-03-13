package columnmapping

import (
	"fmt"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
)

// BuildAllMetadata creates CSVMetadata for all 10 CSV types using the given hashers and operatorID.
func BuildAllMetadata(h hashers.Hashers, operatorID csv.CSVOutputString) []csv.CSVMetadata {
	return []csv.CSVMetadata{
		buildPlayers(h, operatorID),
		buildBets(h, operatorID),
		buildBonus(h, operatorID),
		buildCasino(h),
		buildCasinoPlayers(h),
		buildCasinoParSheet(),
		buildComplaints(h, operatorID),
		buildDemographic(h, operatorID),
		buildDepositsWithdrawals(h, operatorID),
		buildResponsibleGaming(h, operatorID),
	}
}

// DetectCSVType examines headers and returns the matching CSVMetadata.
// Returns an error if zero or more than one type matches.
func DetectCSVType(headers []string, allMetadata []csv.CSVMetadata) (csv.CSVMetadata, error) {
	var matches []csv.CSVMetadata
	for _, m := range allMetadata {
		if m.MatchHeaders(headers) {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no CSV type matched the headers in this file")
	case 1:
		return matches[0], nil
	default:
		types := make([]string, len(matches))
		for i, m := range matches {
			types[i] = m.Type().String()
		}
		return nil, fmt.Errorf("multiple CSV types matched: %v", types)
	}
}

// --- Players (CSVPlayers = 3) ---

func buildPlayers(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVPlayers, []csv.InColumnProcessor{
		csv.UniqueIDDefault(h.PlayerUniqueHasher),
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
	})
}

// --- Bets (CSVBets = 2) ---

func buildBets(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVBets, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.DateAndTimeNonZeroAndNotAfterNow("StartDate", "StartDate"),
		csv.NonEmptyStringWithMax("CouponNumber", "CouponNumber", 256),
		csv.NonEmptyStringWithMax("Currency", "Currency", 3),
		csv.DateAndTimeNonZeroAndNotAfterNow("DateOfLastTransaction", "DateOfLastTransaction"),
		csv.DateAndTimeNonZeroAndNotAfterNow("DateOfPlacementWager", "DateOfPlacementWager"),
		csv.NillableStringWithMax("EventRawDescription", "EventRawDescription", 1024),
		csv.NillableStringWithMax("EventSport", "EventSport", 256),
		csv.NillableStringWithMax("EventLeague", "EventLeague", 256),
		csv.NillableStringWithMax("EventHomeTeam", "EventHomeTeam", 256),
		csv.NillableNonNegInt("EventScoreHomeTeam", "EventScoreHomeTeam"),
		csv.NillableStringWithMax("EventAwayTeam", "EventAwayTeam", 256),
		csv.NillableNonNegInt("EventScoreAwayTeam", "EventScoreAwayTeam"),
		csv.NillableStringWithMax("EventWinningTeam", "EventWinningTeam", 256),
		csv.NillableDateAndTimeNotAfterNow("EventDate", "EventDate"),
		csv.NillableFlexBool("EventClosed", "EventClosed"),
		csv.NonEmptyStringWithMax("Ticket", "Ticket", 256),
		csv.NonNilFloat64Full("TicketsCanceled", "TicketsCanceled"),
		csv.NonNilFloat64Full("TicketsFailed", "TicketsFailed"),
		csv.NonNilFloat64Full("TicketsResettled", "TicketsResettled"),
		csv.NonNilFloat64Full("TicketsSettled", "TicketsSettled"),
		csv.NonNilFloat64Full("TicketsSold", "TicketsSold"),
		csv.NonNilFloat64Full("TicketsVoided", "TicketsVoided"),
		csv.NillableStringWithMax("WagerRawDescription", "WagerRawDescription", 1024),
		csv.NillableStringWithMax("WagerType", "WagerType", 256),
		csv.NillableStringWithMax("WagerDescription", "WagerDescription", 256),
		csv.NillableNonNegFloat64Full("WagerOdds", "WagerOdds"),
		csv.NillableDateAndTimeNotAfterNow("WagerOddsDate", "WagerOddsDate"),
		csv.NillableStringWithMax("WagerOddsBookmaker", "WagerOddsBookmaker", 256),
	})
}

// --- Bonus (CSVBonus = 5) ---

func buildBonus(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVBonus, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.DateAndTimeNonZeroAndNotAfterNow("BonusDate", "Date"),
		csv.NillableNonNegFloat64Full("CashableAmount", "CashableAmount"),
		csv.NillableNonNegFloat64Full("NonCashableAmount", "NonCashableAmount"),
		csv.NillableNonNegFloat64Full("ForfeitedAmount", "ForfeitedAmount"),
	})
}

// --- Casino (CSVCasino = 6) ---

func buildCasino(h hashers.Hashers) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVCasino, []csv.InColumnProcessor{
		csv.MetaID("Casino_Player_Id", "MetaID", "Casino_Country", "Casino_State", "Casino_Country", "Casino_State", h.OrganizationPlayerIDHasher),
		csv.NonNillableNonNegInt("Casino_ID", "Casino_ID"),
		csv.NonNillDate("Accounting_Dt", "Accounting_Dt"),
		csv.NonNillableNonNegInt("Gaming_Type_ID", "Gaming_Type_ID"),
		csv.NonNillableHHMMSSTime("Start_Dttm", "Start_Dttm"),
		csv.NonNillableHHMMSSTime("End_Dttm", "End_Dttm"),
		csv.NillableStringWithMax("MachineNumber", "MachineNumber", 25),
		csv.NillableNonNegInt("Slot_Games_Played_Cnt", "Slot_Games_Played_Cnt"),
		csv.NillableFloat64Full("SlotCoinIn", "SlotCoinIn"),
		csv.NillableFloat64Full("SlotCoinOut", "SlotCoinOut"),
		csv.NillableFloat64Full("Slot_Jackpot_Amt", "Slot_Jackpot_Amt"),
		csv.NillableFloat64Full("Slot_Actl_Player_Win_Amt", "Slot_Actl_Player_Win_Amt"),
		csv.NillableFloat64Full("Slot_Theo_Player_Win_Amt", "Slot_Theo_Player_Win_Amt"),
		csv.NillableNonNegInt("Slot_Tm_Played_Second_Cnt", "Slot_Tm_Played_Second_Cnt"),
		csv.NillableFloat64Full("Slot_Xtra_Credit_Used_Amt", "Slot_Xtra_Credit_Used_Amt"),
		csv.NillableFloat64Full("Slot_Xtra_Credit_PTP_Earn_Amt", "Slot_Xtra_Credit_PTP_Earn_Amt"),
		csv.NillableFloat64Full("Slot_Avg_Bet_Amt", "Slot_Avg_Bet_Amt"),
		csv.NillableFloat64Full("Slot_Total_Comp_Earn_Amt", "Slot_Total_Comp_Earn_Amt"),
		csv.NillableFloat64Full("Slot_PTP_Slot_Play_Used_Amt", "Slot_PTP_Slot_Play_Used_Amt"),
		csv.NillableNonNegInt("Slot_Points_Multp_Cnt", "Slot_Points_Multp_Cnt"),
		csv.NillableNonNegInt("Slot_Points_Multpd_Cnt", "Slot_Points_Multpd_Cnt"),
		csv.NillableFloat64Full("Slot_Exp_Comp_Earn_Amt", "Slot_Exp_Comp_Earn_Amt"),
		csv.NillableNonNegInt("Slot_Ranked_Point_Multpd_Cnt", "Slot_Ranked_Point_Multpd_Cnt"),
		csv.NillableFlexBool("Abandoned_Card_Ind", "Abandoned_Card_Ind"),
		csv.NillableFlexBool("Manual_Edit_Ind", "Manual_Edit_Ind"),
		csv.NillableStringWithMax("Table_Game_Cd", "Table_Game_Cd", 256),
		csv.NillableNonNegInt("Table_Games_Played_Cnt", "Table_Games_Played_Cnt"),
		csv.NillableFloat64Full("Table_Chips_In_Amt", "Table_Chips_In_Amt"),
		csv.NillableFloat64Full("Table_Chips_Out_Amt", "Table_Chips_Out_Amt"),
		csv.NillableNonNegInt("Table_Tm_Played_Second_Cnt", "Table_Tm_Played_Second_Cnt"),
		csv.NillableFloat64Full("Table_Avg_Bet_Amt", "Table_Avg_Bet_Amt"),
		csv.NillableFloat64Full("Table_Actl_Player_Win_Amt", "Table_Actl_Player_Win_Amt"),
		csv.NillableFloat64Full("Table_Theo_Player_Win_Amt", "Table_Theo_Player_Win_Amt"),
		csv.NillableFloat64Full("Table_Total_Comp_Earn_Amt", "Table_Total_Comp_Earn_Amt"),
		csv.NillableFloat64Full("Table_Cash_Buy_In_Amt", "Table_Cash_Buy_In_Amt"),
		csv.NillableFloat64Full("Table_Non_Cash_Buy_In_Amt", "Table_Non_Cash_Buy_In_Amt"),
		csv.NillableFloat64Full("Table_Exp_Comp_Earn_Amt", "Table_Exp_Comp_Earn_Amt"),
		csv.NillableNonNegInt("Session_Base_Point_Cnt", "Session_Base_Point_Cnt"),
		csv.NonNillableNonNegInt("TransID", "TransID"),
		csv.NillableNonNegInt("TableDropNetMarkers", "TableDropNetMarkers"),
	})
}

// --- Casino Players (CSVCasinoPlayers = 4) ---

func buildCasinoPlayers(h hashers.Hashers) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVCasinoPlayers, []csv.InColumnProcessor{
		csv.UniqueID("Player_Last_Name", "Player_First_Name", "Player_Last_4SSN", "XXXX", "Player_DOB", "Player_Id", h.PlayerUniqueHasher),
		csv.MetaID("Casino_Player_Id", "MetaID", "Casino_Country", "Casino_State", "Casino_Country", "Casino_State", h.OrganizationPlayerIDHasher),
		csv.NonNillableNonNegInt("Casino_ID", "Casino_ID"),
		csv.NonEmptyStringWithMax("Gender", "Gender", 1),
		csv.NonEmptyStringWithMax("Zip_Cd", "Zip_Cd", 16),
		csv.NonEmptyStringWithMax("City_Cd", "City_Cd", 64),
		csv.NonEmptyStringWithMax("State_Cd", "State_Cd", 3),
		csv.NonEmptyStringWithMax("Country_Id", "Country_Id", 50),
		csv.NonNillableNonNegInt("Tier_ID", "Tier_ID"),
		csv.NonEmptyStringWithMax("Tier_Name", "Tier_Name", 50),
		csv.NonNillDate("Enrolled_Date", "Enrolled_Date"),
		csv.NillableNonNegInt("Address_Change", "Address_Change"),
	})
}

// --- Casino Par Sheet (CSVCasinoParSheet = 7) ---

func buildCasinoParSheet() csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVCasinoParSheet, []csv.InColumnProcessor{
		csv.NonEmptyStringWithMax("Machine_ID", "Machine_ID", 25),
		csv.NonNillableNonNegInt("MCH_Casino_ID", "MCH_Casino_ID"),
		csv.NonNillMMDDYYYY("MCH_Date", "MCH_Date"),
		csv.NillableFloat64Full("Number_ReelsLinesScatter", "Number_ReelsLinesScatter"),
		csv.NillableFloat64Full("Min_Wager", "Min_Wager"),
		csv.NillableFloat64Full("Max_Wager", "Max_Wager"),
		csv.NillableNonNegInt("Symbols_Per_Reel", "Symbols_Per_Reel"),
		csv.NillableFloat64Full("PaybackPCT", "PaybackPCT"),
		csv.NillableFloat64Full("Hit_FrequencyPCT", "Hit_FrequencyPCT"),
		csv.NillableFloat64Full("Plays_Per_Jackpot", "Plays_Per_Jackpot"),
		csv.NillableFloat64Full("Jackpot_Amount", "Jackpot_Amount"),
		csv.NillableFloat64Full("Plays_Per_Bonus", "Plays_Per_Bonus"),
		csv.NillableFloat64Full("Volatility_Index", "Volatility_Index"),
	})
}

// --- Complaints (CSVComplaints = 8) ---

func buildComplaints(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVComplaints, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.DateAndTimeNonZeroAndNotAfterNow("ComplaintDate", "Date"),
		csv.NonEmptyStringWithMax("Method", "Method", 256),
		csv.NonEmptyStringWithMax("Subject", "Subject", 256),
	})
}

// --- Demographic (CSVDemographic = 9) ---

func buildDemographic(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVDemographic, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.NonNilBirthYear("BirthYear", "BirthYear"),
		csv.NonEmptyStringWithMax("Gender", "Gender", 256),
		csv.CountryAndState("Country", "State", "Country", "State"),
		csv.DateAndTimeNonZeroAndNotAfterNow("AccountOpenDate", "Date"),
		csv.NonEmptyStringWithMax("Operator", "Operator", 256),
	})
}

// --- Deposits/Withdrawals (CSVDepositsWithdrawals = 10) ---

func buildDepositsWithdrawals(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVDepositsWithdrawals, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.NonEmptyStringWithMax("Type", "Type", 256),
		csv.DateAndTimeNonZeroAndNotAfterNow("Date", "Date"),
		csv.NonNilNonNegFloat64Full("Amount", "Amount"),
		csv.NonEmptyStringWithMax("Currency", "Currency", 256),
		csv.NonNilFlexBool("Success", "Success"),
		csv.NonEmptyStringWithMax("Method", "Method", 256),
	})
}

// --- Responsible Gaming (CSVResponsibleGaming = 11) ---

func buildResponsibleGaming(h hashers.Hashers, operatorID csv.CSVOutputString) csv.CSVMetadata {
	return csv.NewSimpleCSVMetadata(csv.CSVResponsibleGaming, []csv.InColumnProcessor{
		csv.MetaIDDefault(h.OrganizationPlayerIDHasher),
		csv.ConstantString("OrgID", operatorID),
		csv.DateAndTimeNonZeroAndNotAfterNow("LimitCreateDate", "LimitCreateDate"),
		csv.NonEmptyStringWithMax("Type", "Type", 256),
		csv.NonEmptyStringWithMax("Period", "Period", 256),
		csv.NonEmptyStringWithMax("Unit", "Unit", 256),
		csv.NonEmptyStringWithMax("Purpose", "Purpose", 256),
		csv.NonNilFloat64Full("Value", "Value"),
	})
}
