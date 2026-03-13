# 06 — CSV Type Definitions

**Dependencies:** 05-csv-framework.md (all processor constructors), 04-hashing-and-normalization.md (hashers).

**Produces:** Registration of all 10 CSV type handlers in `internal/csv/columnmapping/`.

---

This spec is pure reference data. Each section defines the exact processor list for one CSV type. Construct a `simpleCSVMetadata` for each type using exactly the processors listed, in the order shown.

The `hashers` and `operatorID` parameters come from the application at runtime and are passed to the constructors.

---

## 1. Players (CSVPlayers = 3, slug: "players")

**Input:** LastName, FirstName, Last4SSN, DOB, OrganizationPlayerID, OrganizationCountry, OrganizationState

**Output:** UniquePlayerID, MetaID, OrganizationCountry, OrganizationState, OrgID

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `UniqueIDDefault(hashers.PlayerUniqueHasher)` | LastName, FirstName, Last4SSN, DOB | UniquePlayerID |
| 2 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 3 | `ConstantString("OrgID", operatorID)` | — | OrgID |

---

## 2. Bets (CSVBets = 2, slug: "bets")

**Input:** OrganizationPlayerID, OrganizationCountry, OrganizationState, StartDate, CouponNumber, Currency, DateOfLastTransaction, DateOfPlacementWager, EventRawDescription, EventSport, EventLeague, EventHomeTeam, EventScoreHomeTeam, EventAwayTeam, EventScoreAwayTeam, EventWinningTeam, EventDate, EventClosed, Ticket, TicketsCanceled, TicketsFailed, TicketsResettled, TicketsSettled, TicketsSold, TicketsVoided, WagerRawDescription, WagerType, WagerDescription, WagerOdds, WagerOddsDate, WagerOddsBookmaker

**Output:** MetaID, OrganizationCountry, OrganizationState, OrgID, StartDate, CouponNumber, Currency, DateOfLastTransaction, DateOfPlacementWager, EventRawDescription, EventSport, EventLeague, EventHomeTeam, EventScoreHomeTeam, EventAwayTeam, EventScoreAwayTeam, EventWinningTeam, EventDate, EventClosed, Ticket, TicketsCanceled, TicketsFailed, TicketsResettled, TicketsSettled, TicketsSold, TicketsVoided, WagerRawDescription, WagerType, WagerDescription, WagerOdds, WagerOddsDate, WagerOddsBookmaker

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `DateAndTimeNonZeroAndNotAfterNow("StartDate", "StartDate")` | StartDate | StartDate |
| 4 | `NonEmptyStringWithMax("CouponNumber", "CouponNumber", 256)` | CouponNumber | CouponNumber |
| 5 | `NonEmptyStringWithMax("Currency", "Currency", 3)` | Currency | Currency |
| 6 | `DateAndTimeNonZeroAndNotAfterNow("DateOfLastTransaction", "DateOfLastTransaction")` | DateOfLastTransaction | DateOfLastTransaction |
| 7 | `DateAndTimeNonZeroAndNotAfterNow("DateOfPlacementWager", "DateOfPlacementWager")` | DateOfPlacementWager | DateOfPlacementWager |
| 8 | `NillableStringWithMax("EventRawDescription", "EventRawDescription", 1024)` | EventRawDescription | EventRawDescription |
| 9 | `NillableStringWithMax("EventSport", "EventSport", 256)` | EventSport | EventSport |
| 10 | `NillableStringWithMax("EventLeague", "EventLeague", 256)` | EventLeague | EventLeague |
| 11 | `NillableStringWithMax("EventHomeTeam", "EventHomeTeam", 256)` | EventHomeTeam | EventHomeTeam |
| 12 | `NillableNonNegInt("EventScoreHomeTeam", "EventScoreHomeTeam")` | EventScoreHomeTeam | EventScoreHomeTeam |
| 13 | `NillableStringWithMax("EventAwayTeam", "EventAwayTeam", 256)` | EventAwayTeam | EventAwayTeam |
| 14 | `NillableNonNegInt("EventScoreAwayTeam", "EventScoreAwayTeam")` | EventScoreAwayTeam | EventScoreAwayTeam |
| 15 | `NillableStringWithMax("EventWinningTeam", "EventWinningTeam", 256)` | EventWinningTeam | EventWinningTeam |
| 16 | `NillableDateAndTimeNotAfterNow("EventDate", "EventDate")` | EventDate | EventDate |
| 17 | `NillableFlexBool("EventClosed", "EventClosed")` | EventClosed | EventClosed |
| 18 | `NonEmptyStringWithMax("Ticket", "Ticket", 256)` | Ticket | Ticket |
| 19 | `NonNilFloat64Full("TicketsCanceled", "TicketsCanceled")` | TicketsCanceled | TicketsCanceled |
| 20 | `NonNilFloat64Full("TicketsFailed", "TicketsFailed")` | TicketsFailed | TicketsFailed |
| 21 | `NonNilFloat64Full("TicketsResettled", "TicketsResettled")` | TicketsResettled | TicketsResettled |
| 22 | `NonNilFloat64Full("TicketsSettled", "TicketsSettled")` | TicketsSettled | TicketsSettled |
| 23 | `NonNilFloat64Full("TicketsSold", "TicketsSold")` | TicketsSold | TicketsSold |
| 24 | `NonNilFloat64Full("TicketsVoided", "TicketsVoided")` | TicketsVoided | TicketsVoided |
| 25 | `NillableStringWithMax("WagerRawDescription", "WagerRawDescription", 1024)` | WagerRawDescription | WagerRawDescription |
| 26 | `NillableStringWithMax("WagerType", "WagerType", 256)` | WagerType | WagerType |
| 27 | `NillableStringWithMax("WagerDescription", "WagerDescription", 256)` | WagerDescription | WagerDescription |
| 28 | `NillableNonNegFloat64Full("WagerOdds", "WagerOdds")` | WagerOdds | WagerOdds |
| 29 | `NillableDateAndTimeNotAfterNow("WagerOddsDate", "WagerOddsDate")` | WagerOddsDate | WagerOddsDate |
| 30 | `NillableStringWithMax("WagerOddsBookmaker", "WagerOddsBookmaker", 256)` | WagerOddsBookmaker | WagerOddsBookmaker |

---

## 3. Bonus (CSVBonus = 5, slug: "bonus")

**Input:** OrganizationPlayerID, BonusDate, CashableAmount, NonCashableAmount, ForfeitedAmount, OrganizationCountry, OrganizationState

**Output:** MetaID, OrganizationCountry, OrganizationState, OrgID, Date, CashableAmount, NonCashableAmount, ForfeitedAmount

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `DateAndTimeNonZeroAndNotAfterNow("BonusDate", "Date")` | BonusDate | Date |
| 4 | `NillableNonNegFloat64Full("CashableAmount", "CashableAmount")` | CashableAmount | CashableAmount |
| 5 | `NillableNonNegFloat64Full("NonCashableAmount", "NonCashableAmount")` | NonCashableAmount | NonCashableAmount |
| 6 | `NillableNonNegFloat64Full("ForfeitedAmount", "ForfeitedAmount")` | ForfeitedAmount | ForfeitedAmount |

---

## 4. Casino (CSVCasino = 6, slug: "casino")

**Input:** Casino_Player_Id, Casino_Country, Casino_State, Casino_ID, Accounting_Dt, Gaming_Type_ID, Start_Dttm, End_Dttm, MachineNumber, Slot_Games_Played_Cnt, SlotCoinIn, SlotCoinOut, Slot_Jackpot_Amt, Slot_Actl_Player_Win_Amt, Slot_Theo_Player_Win_Amt, Slot_Tm_Played_Second_Cnt, Slot_Xtra_Credit_Used_Amt, Slot_Xtra_Credit_PTP_Earn_Amt, Slot_Avg_Bet_Amt, Slot_Total_Comp_Earn_Amt, Slot_PTP_Slot_Play_Used_Amt, Slot_Points_Multp_Cnt, Slot_Points_Multpd_Cnt, Slot_Exp_Comp_Earn_Amt, Slot_Ranked_Point_Multpd_Cnt, Abandoned_Card_Ind, Manual_Edit_Ind, Table_Game_Cd, Table_Games_Played_Cnt, Table_Chips_In_Amt, Table_Chips_Out_Amt, Table_Tm_Played_Second_Cnt, Table_Avg_Bet_Amt, Table_Actl_Player_Win_Amt, Table_Theo_Player_Win_Amt, Table_Total_Comp_Earn_Amt, Table_Cash_Buy_In_Amt, Table_Non_Cash_Buy_In_Amt, Table_Exp_Comp_Earn_Amt, Session_Base_Point_Cnt, TransID, TableDropNetMarkers

**Note:** Uses custom column names (`Casino_Player_Id`, `Casino_Country`, `Casino_State`). No OrgID constant.

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaID("Casino_Player_Id", "MetaID", "Casino_Country", "Casino_State", "Casino_Country", "Casino_State", hashers.OrganizationPlayerIDHasher)` | Casino_Player_Id, Casino_Country, Casino_State | MetaID, Casino_Country, Casino_State |
| 2 | `NonNillableNonNegInt("Casino_ID", "Casino_ID")` | Casino_ID | Casino_ID |
| 3 | `NonNillDate("Accounting_Dt", "Accounting_Dt")` | Accounting_Dt | Accounting_Dt |
| 4 | `NonNillableNonNegInt("Gaming_Type_ID", "Gaming_Type_ID")` | Gaming_Type_ID | Gaming_Type_ID |
| 5 | `NonNillableHHMMSSTime("Start_Dttm", "Start_Dttm")` | Start_Dttm | Start_Dttm |
| 6 | `NonNillableHHMMSSTime("End_Dttm", "End_Dttm")` | End_Dttm | End_Dttm |
| 7 | `NillableStringWithMax("MachineNumber", "MachineNumber", 25)` | MachineNumber | MachineNumber |
| 8–40 | *(remaining columns — see full processor list below)* | | |

**Full remaining processors (8–40):**

| # | Constructor | Column |
|---|---|---|
| 8 | `NillableNonNegInt` | Slot_Games_Played_Cnt |
| 9 | `NillableFloat64Full` | SlotCoinIn |
| 10 | `NillableFloat64Full` | SlotCoinOut |
| 11 | `NillableFloat64Full` | Slot_Jackpot_Amt |
| 12 | `NillableFloat64Full` | Slot_Actl_Player_Win_Amt |
| 13 | `NillableFloat64Full` | Slot_Theo_Player_Win_Amt |
| 14 | `NillableNonNegInt` | Slot_Tm_Played_Second_Cnt |
| 15 | `NillableFloat64Full` | Slot_Xtra_Credit_Used_Amt |
| 16 | `NillableFloat64Full` | Slot_Xtra_Credit_PTP_Earn_Amt |
| 17 | `NillableFloat64Full` | Slot_Avg_Bet_Amt |
| 18 | `NillableFloat64Full` | Slot_Total_Comp_Earn_Amt |
| 19 | `NillableFloat64Full` | Slot_PTP_Slot_Play_Used_Amt |
| 20 | `NillableNonNegInt` | Slot_Points_Multp_Cnt |
| 21 | `NillableNonNegInt` | Slot_Points_Multpd_Cnt |
| 22 | `NillableFloat64Full` | Slot_Exp_Comp_Earn_Amt |
| 23 | `NillableNonNegInt` | Slot_Ranked_Point_Multpd_Cnt |
| 24 | `NillableFlexBool` | Abandoned_Card_Ind |
| 25 | `NillableFlexBool` | Manual_Edit_Ind |
| 26 | `NillableStringWithMax(_, _, 256)` | Table_Game_Cd |
| 27 | `NillableNonNegInt` | Table_Games_Played_Cnt |
| 28 | `NillableFloat64Full` | Table_Chips_In_Amt |
| 29 | `NillableFloat64Full` | Table_Chips_Out_Amt |
| 30 | `NillableNonNegInt` | Table_Tm_Played_Second_Cnt |
| 31 | `NillableFloat64Full` | Table_Avg_Bet_Amt |
| 32 | `NillableFloat64Full` | Table_Actl_Player_Win_Amt |
| 33 | `NillableFloat64Full` | Table_Theo_Player_Win_Amt |
| 34 | `NillableFloat64Full` | Table_Total_Comp_Earn_Amt |
| 35 | `NillableFloat64Full` | Table_Cash_Buy_In_Amt |
| 36 | `NillableFloat64Full` | Table_Non_Cash_Buy_In_Amt |
| 37 | `NillableFloat64Full` | Table_Exp_Comp_Earn_Amt |
| 38 | `NillableNonNegInt` | Session_Base_Point_Cnt |
| 39 | `NonNillableNonNegInt` | TransID |
| 40 | `NillableNonNegInt` | TableDropNetMarkers |

For processors 8–40 where both input and output column names are identical, use the same name for both parameters.

---

## 5. Casino Players (CSVCasinoPlayers = 4, slug: "casino-players")

**Input:** Casino_Player_Id, Player_First_Name, Player_Last_Name, Player_Last_4SSN, Player_DOB, Casino_Country, Casino_State, Casino_ID, Gender, Zip_Cd, City_Cd, State_Cd, Country_Id, Tier_ID, Tier_Name, Enrolled_Date, Address_Change

**Note:** Uses `"XXXX"` as SSN fallback. No OrgID constant.

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `UniqueID("Player_Last_Name", "Player_First_Name", "Player_Last_4SSN", "XXXX", "Player_DOB", "Player_Id", hashers.PlayerUniqueHasher)` | Player_Last_Name, Player_First_Name, Player_Last_4SSN, Player_DOB | Player_Id |
| 2 | `MetaID("Casino_Player_Id", "MetaID", "Casino_Country", "Casino_State", "Casino_Country", "Casino_State", hashers.OrganizationPlayerIDHasher)` | Casino_Player_Id, Casino_Country, Casino_State | MetaID, Casino_Country, Casino_State |
| 3 | `NonNillableNonNegInt("Casino_ID", "Casino_ID")` | Casino_ID | Casino_ID |
| 4 | `NonEmptyStringWithMax("Gender", "Gender", 1)` | Gender | Gender |
| 5 | `NonEmptyStringWithMax("Zip_Cd", "Zip_Cd", 16)` | Zip_Cd | Zip_Cd |
| 6 | `NonEmptyStringWithMax("City_Cd", "City_Cd", 64)` | City_Cd | City_Cd |
| 7 | `NonEmptyStringWithMax("State_Cd", "State_Cd", 3)` | State_Cd | State_Cd |
| 8 | `NonEmptyStringWithMax("Country_Id", "Country_Id", 50)` | Country_Id | Country_Id |
| 9 | `NonNillableNonNegInt("Tier_ID", "Tier_ID")` | Tier_ID | Tier_ID |
| 10 | `NonEmptyStringWithMax("Tier_Name", "Tier_Name", 50)` | Tier_Name | Tier_Name |
| 11 | `NonNillDate("Enrolled_Date", "Enrolled_Date")` | Enrolled_Date | Enrolled_Date |
| 12 | `NillableNonNegInt("Address_Change", "Address_Change")` | Address_Change | Address_Change |

---

## 6. Casino Par Sheet (CSVCasinoParSheet = 7, slug: "casino-par-sheet")

**The only CSV type with no player identification.** No MetaID, no UniqueID, no OrgID.

**Input/Output:** Machine_ID, MCH_Casino_ID, MCH_Date, Number_ReelsLinesScatter, Min_Wager, Max_Wager, Symbols_Per_Reel, PaybackPCT, Hit_FrequencyPCT, Plays_Per_Jackpot, Jackpot_Amount, Plays_Per_Bonus, Volatility_Index

| # | Constructor | Column |
|---|---|---|
| 1 | `NonEmptyStringWithMax(_, _, 25)` | Machine_ID |
| 2 | `NonNillableNonNegInt` | MCH_Casino_ID |
| 3 | `NonNillMMDDYYYY` | MCH_Date |
| 4 | `NillableFloat64Full` | Number_ReelsLinesScatter |
| 5 | `NillableFloat64Full` | Min_Wager |
| 6 | `NillableFloat64Full` | Max_Wager |
| 7 | `NillableNonNegInt` | Symbols_Per_Reel |
| 8 | `NillableFloat64Full` | PaybackPCT |
| 9 | `NillableFloat64Full` | Hit_FrequencyPCT |
| 10 | `NillableFloat64Full` | Plays_Per_Jackpot |
| 11 | `NillableFloat64Full` | Jackpot_Amount |
| 12 | `NillableFloat64Full` | Plays_Per_Bonus |
| 13 | `NillableFloat64Full` | Volatility_Index |

---

## 7. Complaints (CSVComplaints = 8, slug: "complaints")

**Input:** OrganizationPlayerID, ComplaintDate, Method, Subject, OrganizationCountry, OrganizationState

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `DateAndTimeNonZeroAndNotAfterNow("ComplaintDate", "Date")` | ComplaintDate | Date |
| 4 | `NonEmptyStringWithMax("Method", "Method", 256)` | Method | Method |
| 5 | `NonEmptyStringWithMax("Subject", "Subject", 256)` | Subject | Subject |

---

## 8. Demographic (CSVDemographic = 9, slug: "demographic")

**Input:** OrganizationPlayerID, BirthYear, Gender, Country, State, AccountOpenDate, Operator, OrganizationCountry, OrganizationState

**Note:** Two separate country/state pairs — Organization (for hashing) and demographic (validated, not hashed).

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `NonNilBirthYear("BirthYear", "BirthYear")` | BirthYear | BirthYear |
| 4 | `NonEmptyStringWithMax("Gender", "Gender", 256)` | Gender | Gender |
| 5 | `CountryAndState("Country", "State", "Country", "State")` | Country, State | Country, State |
| 6 | `DateAndTimeNonZeroAndNotAfterNow("AccountOpenDate", "Date")` | AccountOpenDate | Date |
| 7 | `NonEmptyStringWithMax("Operator", "Operator", 256)` | Operator | Operator |

---

## 9. Deposits/Withdrawals (CSVDepositsWithdrawals = 10, slug: "deposits-withdrawals")

**Input:** OrganizationPlayerID, Type, Date, Amount, Currency, Success, Method, OrganizationCountry, OrganizationState

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `NonEmptyStringWithMax("Type", "Type", 256)` | Type | Type |
| 4 | `DateAndTimeNonZeroAndNotAfterNow("Date", "Date")` | Date | Date |
| 5 | `NonNilNonNegFloat64Full("Amount", "Amount")` | Amount | Amount |
| 6 | `NonEmptyStringWithMax("Currency", "Currency", 256)` | Currency | Currency |
| 7 | `NonNilFlexBool("Success", "Success")` | Success | Success |
| 8 | `NonEmptyStringWithMax("Method", "Method", 256)` | Method | Method |

---

## 10. Responsible Gaming (CSVResponsibleGaming = 11, slug: "responsible-gaming")

**Input:** OrganizationPlayerID, LimitCreateDate, Type, Period, Unit, Purpose, Value, OrganizationCountry, OrganizationState

| # | Constructor | Input | Output |
|---|---|---|---|
| 1 | `MetaIDDefault(hashers.OrganizationPlayerIDHasher)` | OrganizationPlayerID, OrganizationCountry, OrganizationState | MetaID, OrganizationCountry, OrganizationState |
| 2 | `ConstantString("OrgID", operatorID)` | — | OrgID |
| 3 | `DateAndTimeNonZeroAndNotAfterNow("LimitCreateDate", "LimitCreateDate")` | LimitCreateDate | LimitCreateDate |
| 4 | `NonEmptyStringWithMax("Type", "Type", 256)` | Type | Type |
| 5 | `NonEmptyStringWithMax("Period", "Period", 256)` | Period | Period |
| 6 | `NonEmptyStringWithMax("Unit", "Unit", 256)` | Unit | Unit |
| 7 | `NonEmptyStringWithMax("Purpose", "Purpose", 256)` | Purpose | Purpose |
| 8 | `NonNilFloat64Full("Value", "Value")` | Value | Value |

---

## Auto-Detection

All 10 types have sufficiently distinct required column sets. Casino types use `Casino_Player_Id`/`Casino_Country`/`Casino_State` while standard types use `OrganizationPlayerID`/`OrganizationCountry`/`OrganizationState`. Casino Par Sheet is uniquely identified by `Machine_ID` and `MCH_Casino_ID`.

## Tests

### Auto-Detection Tests

For each CSV type, construct a header row with exactly the required columns, run detection, verify the correct type is returned. Also test:
- Extra columns still match
- Missing one required column → no match
- No matching type → error
- Ambiguous match → error (should not happen with these definitions)

## Acceptance Criteria

- [ ] All 10 types registered with exact processor lists
- [ ] Auto-detection matches exactly one handler for valid files
- [ ] Casino types use custom column names
- [ ] Casino Players uses `"XXXX"` SSN fallback
- [ ] Casino Par Sheet has no player identification
- [ ] Demographic validates two separate country/state pairs
- [ ] All tests pass
