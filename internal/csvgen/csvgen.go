package csvgen

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand"
	"strings"
	"time"

	csvpkg "github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// Option configures the CSV generator.
type Option func(*generator)

// WithSeed sets a deterministic seed for random number generation.
func WithSeed(seed int64) Option {
	return func(g *generator) {
		g.seed = seed
		g.seedSet = true
	}
}

// WithErrorInjection enables error injection at the given rate (0.0-1.0).
func WithErrorInjection(rate float64) Option {
	return func(g *generator) {
		g.injectErrors = true
		g.errorRate = rate
	}
}

type generator struct {
	seed         int64
	seedSet      bool
	injectErrors bool
	errorRate    float64
}

// inputColumns defines the input columns for each CSV type, matching the spec 06 definitions exactly.
var inputColumns = map[csvpkg.CSVType][]string{
	csvpkg.CSVPlayers: {
		"LastName", "FirstName", "Last4SSN", "DOB",
		"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
	},
	csvpkg.CSVBets: {
		"OrganizationPlayerID", "OrganizationCountry", "OrganizationState",
		"StartDate", "CouponNumber", "Currency",
		"DateOfLastTransaction", "DateOfPlacementWager",
		"EventRawDescription", "EventSport", "EventLeague",
		"EventHomeTeam", "EventScoreHomeTeam", "EventAwayTeam", "EventScoreAwayTeam",
		"EventWinningTeam", "EventDate", "EventClosed",
		"Ticket", "TicketsCanceled", "TicketsFailed", "TicketsResettled",
		"TicketsSettled", "TicketsSold", "TicketsVoided",
		"WagerRawDescription", "WagerType", "WagerDescription",
		"WagerOdds", "WagerOddsDate", "WagerOddsBookmaker",
	},
	csvpkg.CSVBonus: {
		"OrganizationPlayerID", "BonusDate",
		"CashableAmount", "NonCashableAmount", "ForfeitedAmount",
		"OrganizationCountry", "OrganizationState",
	},
	csvpkg.CSVCasino: {
		"Casino_Player_Id", "Casino_Country", "Casino_State",
		"Casino_ID", "Accounting_Dt", "Gaming_Type_ID",
		"Start_Dttm", "End_Dttm", "MachineNumber",
		"Slot_Games_Played_Cnt", "SlotCoinIn", "SlotCoinOut",
		"Slot_Jackpot_Amt", "Slot_Actl_Player_Win_Amt", "Slot_Theo_Player_Win_Amt",
		"Slot_Tm_Played_Second_Cnt", "Slot_Xtra_Credit_Used_Amt", "Slot_Xtra_Credit_PTP_Earn_Amt",
		"Slot_Avg_Bet_Amt", "Slot_Total_Comp_Earn_Amt", "Slot_PTP_Slot_Play_Used_Amt",
		"Slot_Points_Multp_Cnt", "Slot_Points_Multpd_Cnt", "Slot_Exp_Comp_Earn_Amt",
		"Slot_Ranked_Point_Multpd_Cnt",
		"Abandoned_Card_Ind", "Manual_Edit_Ind",
		"Table_Game_Cd", "Table_Games_Played_Cnt",
		"Table_Chips_In_Amt", "Table_Chips_Out_Amt",
		"Table_Tm_Played_Second_Cnt", "Table_Avg_Bet_Amt",
		"Table_Actl_Player_Win_Amt", "Table_Theo_Player_Win_Amt",
		"Table_Total_Comp_Earn_Amt", "Table_Cash_Buy_In_Amt", "Table_Non_Cash_Buy_In_Amt",
		"Table_Exp_Comp_Earn_Amt", "Session_Base_Point_Cnt",
		"TransID", "TableDropNetMarkers",
	},
	csvpkg.CSVCasinoPlayers: {
		"Casino_Player_Id", "Player_First_Name", "Player_Last_Name",
		"Player_Last_4SSN", "Player_DOB",
		"Casino_Country", "Casino_State",
		"Casino_ID", "Gender", "Zip_Cd", "City_Cd", "State_Cd", "Country_Id",
		"Tier_ID", "Tier_Name", "Enrolled_Date", "Address_Change",
	},
	csvpkg.CSVCasinoParSheet: {
		"Machine_ID", "MCH_Casino_ID", "MCH_Date",
		"Number_ReelsLinesScatter", "Min_Wager", "Max_Wager",
		"Symbols_Per_Reel", "PaybackPCT", "Hit_FrequencyPCT",
		"Plays_Per_Jackpot", "Jackpot_Amount", "Plays_Per_Bonus", "Volatility_Index",
	},
	csvpkg.CSVComplaints: {
		"OrganizationPlayerID", "ComplaintDate", "Method", "Subject",
		"OrganizationCountry", "OrganizationState",
	},
	csvpkg.CSVDemographic: {
		"OrganizationPlayerID", "BirthYear", "Gender",
		"Country", "State", "AccountOpenDate", "Operator",
		"OrganizationCountry", "OrganizationState",
	},
	csvpkg.CSVDepositsWithdrawals: {
		"OrganizationPlayerID", "Type", "Date", "Amount", "Currency",
		"Success", "Method",
		"OrganizationCountry", "OrganizationState",
	},
	csvpkg.CSVResponsibleGaming: {
		"OrganizationPlayerID", "LimitCreateDate", "Type", "Period",
		"Unit", "Purpose", "Value",
		"OrganizationCountry", "OrganizationState",
	},
}

// InputColumnsForType returns the input columns for a CSV type.
func InputColumnsForType(t csvpkg.CSVType) ([]string, error) {
	cols, ok := inputColumns[t]
	if !ok {
		return nil, fmt.Errorf("unknown CSV type: %v", t)
	}
	result := make([]string, len(cols))
	copy(result, cols)
	return result, nil
}

// GenerateCSV generates synthetic CSV data for the given type and row count.
func GenerateCSV(csvType csvpkg.CSVType, rows int, opts ...Option) ([]byte, error) {
	g := &generator{}
	for _, opt := range opts {
		opt(g)
	}

	cols, ok := inputColumns[csvType]
	if !ok {
		return nil, fmt.Errorf("unknown CSV type: %v", csvType)
	}

	var rng *rand.Rand
	if g.seedSet {
		rng = rand.New(rand.NewSource(g.seed))
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header
	if err := w.Write(cols); err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}

	for i := range rows {
		row := generateRow(rng, csvType, cols, i+1)

		if g.injectErrors && rng.Float64() < g.errorRate {
			injectError(rng, cols, row)
		}

		record := make([]string, len(cols))
		for j, col := range cols {
			record[j] = row[col]
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("writing row %d: %w", i+1, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flushing CSV: %w", err)
	}

	return buf.Bytes(), nil
}

// --- Name pools ---

var firstNames = []string{
	"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
	"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Christopher", "Karen", "José", "María", "François", "Björk",
	"Müller", "Søren", "André", "Hélène", "Yüksel", "Renée", "O'Brien", "Naïve",
	"Chloé", "Zoë", "Léa", "Noël", "Jürgen", "Günther", "Ólafur", "Ástríður",
	"Siobhán", "Seán", "Maël", "Anaïs", "Raphaël", "Gaël", "Loïc", "Benoît",
	"Cédric", "Élodie",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "García", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Thompson", "White",
	"Harris", "Clark", "Smith-Jones", "O'Connor", "van der Berg", "De la Cruz",
	"MacDonald", "St. Claire", "Müller-Schmidt", "Al-Rashid", "Di Stefano",
	"FitzGerald", "McAllister", "O'Sullivan", "Van Dyke", "De Groot", "Le Blanc",
	"Fernández", "González", "Hernández", "López", "Martínez", "Rodríguez",
	"Ramírez", "Sánchez", "Pérez", "Gutiérrez",
}

// US state codes (50 states + DC)
var usStates = []string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
	"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
	"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
	"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
	"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
	"DC",
}

var boolValues = []string{"true", "false", "yes", "no", "t", "f"}

var complaintMethods = []string{"Phone", "Email", "Chat", "Mail", "In-Person"}
var complaintSubjects = []string{
	"Account Issue", "Bonus Dispute", "Technical Problem",
	"Payment Delay", "Verification", "Game Malfunction",
}
var genders = []string{"M", "F", "O"}
var depTypes = []string{"Deposit", "Withdrawal"}
var currencies = []string{"USD", "EUR", "GBP"}
var paymentMethods = []string{"Credit Card", "Wire Transfer", "ACH", "PayPal", "Check"}
var rgTypes = []string{"Deposit Limit", "Wager Limit", "Loss Limit", "Time Limit", "Session Limit"}
var rgPeriods = []string{"Daily", "Weekly", "Monthly", "Yearly"}
var rgUnits = []string{"USD", "Hours", "Minutes", "Count"}
var rgPurposes = []string{"Self-Exclusion", "Responsible Gaming", "Cooling Off", "Budget Control"}
var operators = []string{"OperatorA", "OperatorB", "OperatorC"}
var sports = []string{"Football", "Basketball", "Baseball", "Hockey", "Soccer", "Tennis"}
var leagues = []string{"NFL", "NBA", "MLB", "NHL", "MLS", "ATP"}
var teams = []string{
	"Eagles", "Cowboys", "Giants", "Commanders",
	"Lakers", "Celtics", "Warriors", "Heat",
	"Yankees", "Red Sox", "Dodgers", "Cubs",
}
var wagerTypes = []string{"Moneyline", "Spread", "Over/Under", "Parlay", "Prop", "Futures"}
var tierNames = []string{"Bronze", "Silver", "Gold", "Platinum", "Diamond"}
var cities = []string{
	"New York", "Los Angeles", "Chicago", "Houston", "Phoenix",
	"Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose",
}
var tableGames = []string{"Blackjack", "Roulette", "Poker", "Baccarat", "Craps"}

// --- Value generation helpers ---

func playerID(rowNum int) string {
	return fmt.Sprintf("PLAYER-%06d", rowNum)
}

func randomDate(rng *rand.Rand) time.Time {
	now := time.Now()
	fiveYearsAgo := now.AddDate(-5, 0, 0)
	delta := now.Unix() - fiveYearsAgo.Unix()
	return time.Unix(fiveYearsAgo.Unix()+rng.Int63n(delta), 0).UTC()
}

func randomDOB(rng *rand.Rand) time.Time {
	// Between 18 and 80 years ago
	now := time.Now()
	minAge := now.AddDate(-80, 0, 0)
	maxAge := now.AddDate(-18, 0, 0)
	delta := maxAge.Unix() - minAge.Unix()
	return time.Unix(minAge.Unix()+rng.Int63n(delta), 0).UTC()
}

func formatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func formatDateOnly(t time.Time) string {
	return t.Format("2006-01-02")
}

func formatMMDDYYYY(t time.Time) string {
	return t.Format("01022006")
}

func formatHHMMSS(rng *rand.Rand) string {
	h := rng.Intn(24)
	m := rng.Intn(60)
	s := rng.Intn(60)
	return fmt.Sprintf("%02d%02d%02d", h, m, s)
}

func randomLast4SSN(rng *rand.Rand) string {
	return fmt.Sprintf("%04d", rng.Intn(10000))
}

func randomDollarAmount(rng *rand.Rand, min, max float64) string {
	v := min + rng.Float64()*(max-min)
	return fmt.Sprintf("%.2f", v)
}

func randomSmallInt(rng *rand.Rand, max int) string {
	return fmt.Sprintf("%d", rng.Intn(max)+1)
}

func randomPercentage(rng *rand.Rand) string {
	return fmt.Sprintf("%.2f", rng.Float64()*100)
}

func randomBool(rng *rand.Rand) string {
	return boolValues[rng.Intn(len(boolValues))]
}

func randomState(rng *rand.Rand) string {
	return usStates[rng.Intn(len(usStates))]
}

func nullable(rng *rand.Rand, value string) string {
	if rng.Float64() < 0.1 {
		return ""
	}
	return value
}

func pick(rng *rand.Rand, pool []string) string {
	return pool[rng.Intn(len(pool))]
}

// generateRow generates a single valid data row for the given CSV type.
func generateRow(rng *rand.Rand, csvType csvpkg.CSVType, cols []string, rowNum int) map[string]string {
	row := make(map[string]string, len(cols))

	switch csvType {
	case csvpkg.CSVPlayers:
		row["LastName"] = pick(rng, lastNames)
		row["FirstName"] = pick(rng, firstNames)
		row["Last4SSN"] = randomLast4SSN(rng)
		row["DOB"] = formatDateOnly(randomDOB(rng))
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)

	case csvpkg.CSVBets:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)
		row["StartDate"] = formatDateTime(randomDate(rng))
		row["CouponNumber"] = fmt.Sprintf("CPN-%06d", rng.Intn(999999))
		row["Currency"] = "USD"
		row["DateOfLastTransaction"] = formatDateTime(randomDate(rng))
		row["DateOfPlacementWager"] = formatDateTime(randomDate(rng))
		row["EventRawDescription"] = nullable(rng, fmt.Sprintf("Event %d", rng.Intn(1000)))
		row["EventSport"] = nullable(rng, pick(rng, sports))
		row["EventLeague"] = nullable(rng, pick(rng, leagues))
		homeTeam := pick(rng, teams)
		awayTeam := pick(rng, teams)
		row["EventHomeTeam"] = nullable(rng, homeTeam)
		row["EventScoreHomeTeam"] = nullable(rng, randomSmallInt(rng, 10))
		row["EventAwayTeam"] = nullable(rng, awayTeam)
		row["EventScoreAwayTeam"] = nullable(rng, randomSmallInt(rng, 10))
		row["EventWinningTeam"] = nullable(rng, homeTeam)
		row["EventDate"] = nullable(rng, formatDateTime(randomDate(rng)))
		row["EventClosed"] = nullable(rng, randomBool(rng))
		row["Ticket"] = fmt.Sprintf("TKT-%08d", rng.Intn(99999999))
		row["TicketsCanceled"] = randomDollarAmount(rng, 0, 100)
		row["TicketsFailed"] = randomDollarAmount(rng, 0, 100)
		row["TicketsResettled"] = randomDollarAmount(rng, 0, 100)
		row["TicketsSettled"] = randomDollarAmount(rng, 1, 10000)
		row["TicketsSold"] = randomDollarAmount(rng, 1, 10000)
		row["TicketsVoided"] = randomDollarAmount(rng, 0, 100)
		row["WagerRawDescription"] = nullable(rng, fmt.Sprintf("Wager on %s", homeTeam))
		row["WagerType"] = nullable(rng, pick(rng, wagerTypes))
		row["WagerDescription"] = nullable(rng, fmt.Sprintf("%s vs %s", homeTeam, awayTeam))
		row["WagerOdds"] = nullable(rng, randomDollarAmount(rng, 1, 500))
		row["WagerOddsDate"] = nullable(rng, formatDateTime(randomDate(rng)))
		row["WagerOddsBookmaker"] = nullable(rng, fmt.Sprintf("Bookmaker-%d", rng.Intn(20)))

	case csvpkg.CSVBonus:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["BonusDate"] = formatDateTime(randomDate(rng))
		row["CashableAmount"] = nullable(rng, randomDollarAmount(rng, 5, 500))
		row["NonCashableAmount"] = nullable(rng, randomDollarAmount(rng, 5, 500))
		row["ForfeitedAmount"] = nullable(rng, randomDollarAmount(rng, 0, 200))
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)

	case csvpkg.CSVCasino:
		row["Casino_Player_Id"] = playerID(rowNum)
		row["Casino_Country"] = "US"
		row["Casino_State"] = randomState(rng)
		row["Casino_ID"] = randomSmallInt(rng, 100)
		row["Accounting_Dt"] = formatDateOnly(randomDate(rng))
		row["Gaming_Type_ID"] = randomSmallInt(rng, 10)
		row["Start_Dttm"] = formatHHMMSS(rng)
		row["End_Dttm"] = formatHHMMSS(rng)
		row["MachineNumber"] = nullable(rng, fmt.Sprintf("MCH-%04d", rng.Intn(9999)))
		row["Slot_Games_Played_Cnt"] = nullable(rng, randomSmallInt(rng, 100))
		row["SlotCoinIn"] = nullable(rng, randomDollarAmount(rng, 1, 10000))
		row["SlotCoinOut"] = nullable(rng, randomDollarAmount(rng, 1, 10000))
		row["Slot_Jackpot_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 50000))
		row["Slot_Actl_Player_Win_Amt"] = nullable(rng, randomDollarAmount(rng, -5000, 10000))
		row["Slot_Theo_Player_Win_Amt"] = nullable(rng, randomDollarAmount(rng, -5000, 10000))
		row["Slot_Tm_Played_Second_Cnt"] = nullable(rng, randomSmallInt(rng, 3600))
		row["Slot_Xtra_Credit_Used_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 500))
		row["Slot_Xtra_Credit_PTP_Earn_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 500))
		row["Slot_Avg_Bet_Amt"] = nullable(rng, randomDollarAmount(rng, 1, 100))
		row["Slot_Total_Comp_Earn_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 200))
		row["Slot_PTP_Slot_Play_Used_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 1000))
		row["Slot_Points_Multp_Cnt"] = nullable(rng, randomSmallInt(rng, 10))
		row["Slot_Points_Multpd_Cnt"] = nullable(rng, randomSmallInt(rng, 10))
		row["Slot_Exp_Comp_Earn_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 200))
		row["Slot_Ranked_Point_Multpd_Cnt"] = nullable(rng, randomSmallInt(rng, 10))
		row["Abandoned_Card_Ind"] = nullable(rng, randomBool(rng))
		row["Manual_Edit_Ind"] = nullable(rng, randomBool(rng))
		row["Table_Game_Cd"] = nullable(rng, pick(rng, tableGames))
		row["Table_Games_Played_Cnt"] = nullable(rng, randomSmallInt(rng, 50))
		row["Table_Chips_In_Amt"] = nullable(rng, randomDollarAmount(rng, 1, 10000))
		row["Table_Chips_Out_Amt"] = nullable(rng, randomDollarAmount(rng, 1, 10000))
		row["Table_Tm_Played_Second_Cnt"] = nullable(rng, randomSmallInt(rng, 3600))
		row["Table_Avg_Bet_Amt"] = nullable(rng, randomDollarAmount(rng, 5, 500))
		row["Table_Actl_Player_Win_Amt"] = nullable(rng, randomDollarAmount(rng, -5000, 10000))
		row["Table_Theo_Player_Win_Amt"] = nullable(rng, randomDollarAmount(rng, -5000, 10000))
		row["Table_Total_Comp_Earn_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 200))
		row["Table_Cash_Buy_In_Amt"] = nullable(rng, randomDollarAmount(rng, 100, 50000))
		row["Table_Non_Cash_Buy_In_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 10000))
		row["Table_Exp_Comp_Earn_Amt"] = nullable(rng, randomDollarAmount(rng, 0, 200))
		row["Session_Base_Point_Cnt"] = nullable(rng, randomSmallInt(rng, 100))
		row["TransID"] = fmt.Sprintf("%d", rng.Intn(999999)+1)
		row["TableDropNetMarkers"] = nullable(rng, randomSmallInt(rng, 50))

	case csvpkg.CSVCasinoPlayers:
		row["Casino_Player_Id"] = playerID(rowNum)
		row["Player_First_Name"] = pick(rng, firstNames)
		row["Player_Last_Name"] = pick(rng, lastNames)
		row["Player_Last_4SSN"] = randomLast4SSN(rng)
		row["Player_DOB"] = formatDateOnly(randomDOB(rng))
		row["Casino_Country"] = "US"
		row["Casino_State"] = randomState(rng)
		row["Casino_ID"] = randomSmallInt(rng, 100)
		row["Gender"] = pick(rng, genders)[:1]
		row["Zip_Cd"] = fmt.Sprintf("%05d", rng.Intn(99999))
		row["City_Cd"] = pick(rng, cities)
		row["State_Cd"] = randomState(rng)
		row["Country_Id"] = "US"
		row["Tier_ID"] = randomSmallInt(rng, 5)
		row["Tier_Name"] = pick(rng, tierNames)
		row["Enrolled_Date"] = formatDateOnly(randomDate(rng))
		row["Address_Change"] = nullable(rng, randomSmallInt(rng, 5))

	case csvpkg.CSVCasinoParSheet:
		row["Machine_ID"] = fmt.Sprintf("MCH-%04d", rng.Intn(9999))
		row["MCH_Casino_ID"] = randomSmallInt(rng, 100)
		row["MCH_Date"] = formatMMDDYYYY(randomDate(rng))
		row["Number_ReelsLinesScatter"] = nullable(rng, randomPercentage(rng))
		row["Min_Wager"] = nullable(rng, randomDollarAmount(rng, 0.01, 5))
		row["Max_Wager"] = nullable(rng, randomDollarAmount(rng, 5, 500))
		row["Symbols_Per_Reel"] = nullable(rng, randomSmallInt(rng, 30))
		row["PaybackPCT"] = nullable(rng, randomPercentage(rng))
		row["Hit_FrequencyPCT"] = nullable(rng, randomPercentage(rng))
		row["Plays_Per_Jackpot"] = nullable(rng, randomDollarAmount(rng, 1000, 100000))
		row["Jackpot_Amount"] = nullable(rng, randomDollarAmount(rng, 100, 1000000))
		row["Plays_Per_Bonus"] = nullable(rng, randomDollarAmount(rng, 10, 1000))
		row["Volatility_Index"] = nullable(rng, randomPercentage(rng))

	case csvpkg.CSVComplaints:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["ComplaintDate"] = formatDateTime(randomDate(rng))
		row["Method"] = pick(rng, complaintMethods)
		row["Subject"] = pick(rng, complaintSubjects)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)

	case csvpkg.CSVDemographic:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["BirthYear"] = formatDateOnly(randomDOB(rng))
		row["Gender"] = pick(rng, genders)
		row["Country"] = "US"
		row["State"] = randomState(rng)
		row["AccountOpenDate"] = formatDateTime(randomDate(rng))
		row["Operator"] = pick(rng, operators)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)

	case csvpkg.CSVDepositsWithdrawals:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["Type"] = pick(rng, depTypes)
		row["Date"] = formatDateTime(randomDate(rng))
		row["Amount"] = randomDollarAmount(rng, 10, 50000)
		row["Currency"] = pick(rng, currencies)
		row["Success"] = randomBool(rng)
		row["Method"] = pick(rng, paymentMethods)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)

	case csvpkg.CSVResponsibleGaming:
		row["OrganizationPlayerID"] = playerID(rowNum)
		row["LimitCreateDate"] = formatDateTime(randomDate(rng))
		row["Type"] = pick(rng, rgTypes)
		row["Period"] = pick(rng, rgPeriods)
		row["Unit"] = pick(rng, rgUnits)
		row["Purpose"] = pick(rng, rgPurposes)
		row["Value"] = randomDollarAmount(rng, 1, 10000)
		row["OrganizationCountry"] = "US"
		row["OrganizationState"] = randomState(rng)
	}

	return row
}

// injectError randomly corrupts one field in a row.
func injectError(rng *rand.Rand, cols []string, row map[string]string) {
	errorType := rng.Intn(6)
	// Pick a random column to corrupt (skip cols that might not exist)
	col := cols[rng.Intn(len(cols))]

	switch errorType {
	case 0: // Missing required field
		row[col] = ""
	case 1: // Invalid type (string where number expected)
		row[col] = "not_a_number"
	case 2: // Future date
		future := time.Now().AddDate(5, 0, 0)
		row[col] = formatDateTime(future)
	case 3: // Invalid state code
		invalidStates := []string{"XX", "ZZ", "QQ"}
		row[col] = invalidStates[rng.Intn(len(invalidStates))]
	case 4: // Negative amount
		row[col] = "-999.99"
	case 5: // Overlong string
		row[col] = strings.Repeat("X", 2000)
	}
}
