package hashers

import (
	"encoding/base64"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Hashers defines the interface for player data hashing operations.
type Hashers interface {
	PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string
	OrganizationPlayerIDHasher(playerID, country, state string) string
	SaveDB() error
}

// PlayerDB defines the interface for the player deduplication database.
// Implemented in spec 08; used here for dependency inversion.
type PlayerDB interface {
	GetPlayerByOrgPlayerID(playerID, country, state string) (metaID string, found bool)
	AddEntry(metaID, playerID, country, state string)
}

// PlayerDataHasher implements the Hashers interface using Argon2id hashing.
type PlayerDataHasher struct {
	useOnlyFirstLetterOfFirstName bool
	dbPath                        string
	uniqueIDPepper                string
	orgPlayerIDPepper             string
	nameProcessor                 func(string) string
	playersdb                     PlayerDB
}

// NewPlayerDataHasher creates a new PlayerDataHasher with the given configuration.
func NewPlayerDataHasher(
	useOnlyFirstLetterOfFirstName bool,
	dbPath string,
	uniqueIDPepper, orgPlayerIDPepper string,
	nameProcessor func(string) string,
	playersdb PlayerDB,
) *PlayerDataHasher {
	return &PlayerDataHasher{
		useOnlyFirstLetterOfFirstName: useOnlyFirstLetterOfFirstName,
		dbPath:                        dbPath,
		uniqueIDPepper:                uniqueIDPepper,
		orgPlayerIDPepper:             orgPlayerIDPepper,
		nameProcessor:                 nameProcessor,
		playersdb:                     playersdb,
	}
}

// argon2Hash computes an Argon2id hash with the spec parameters and returns
// the result as a standard base64-encoded string.
func argon2Hash(cleartext, pepper string) string {
	hash := argon2.IDKey([]byte(cleartext), []byte(pepper), 3, 32*1024, 4, 32)
	return base64.StdEncoding.EncodeToString(hash)
}

// PlayerUniqueHasher computes a unique player hash from PII fields.
func (h *PlayerDataHasher) PlayerUniqueHasher(last4SSN, firstName, lastName, dob string) string {
	if h.useOnlyFirstLetterOfFirstName && len(firstName) > 1 {
		firstName = firstName[:1]
	}
	firstName = h.nameProcessor(firstName)
	lastName = h.nameProcessor(lastName)
	if len(firstName) > 0 {
		firstName = firstName[:1]
	}
	cleartext := h.uniqueIDPepper + last4SSN + firstName + lastName + dob
	return argon2Hash(cleartext, h.uniqueIDPepper)
}

// OrganizationPlayerIDHasher computes a hash for an organization player ID,
// using the PlayerDB cache for deduplication.
func (h *PlayerDataHasher) OrganizationPlayerIDHasher(playerID, country, state string) string {
	if metaID, found := h.playersdb.GetPlayerByOrgPlayerID(playerID, country, state); found {
		return metaID
	}
	cleartext := h.orgPlayerIDPepper + ":" + playerID + ":" + country + ":" + state
	metaID := argon2Hash(cleartext, h.orgPlayerIDPepper)
	h.playersdb.AddEntry(metaID, playerID, country, state)
	return metaID
}

// SaveDB persists the PlayerDB to disk at the configured path.
// Uses a type assertion since Save is an implementation detail, not part of PlayerDB.
func (h *PlayerDataHasher) SaveDB() error {
	type saver interface{ Save(path string) error }
	if s, ok := h.playersdb.(saver); ok {
		return s.Save(h.dbPath)
	}
	return nil
}

// Compile-time check that PlayerDataHasher implements Hashers.
var _ Hashers = (*PlayerDataHasher)(nil)

// --- Name Normalization ---

var (
	multiSpaceRe  = regexp.MustCompile(`\s{2,}`)
	multiHyphenRe = regexp.MustCompile(`-{2,}`)
	spaceHyphenRe = regexp.MustCompile(`\s*-\s*`)
)

// ProcessName normalizes a name string by removing diacriticals, normalizing
// punctuation and whitespace, and lowercasing. It iterates until stable.
func ProcessName(in string) string {
	current := in
	for {
		next := processNameOnce(current)
		if next == current {
			return next
		}
		current = next
	}
}

func processNameOnce(s string) string {
	// 1. Process punctuation
	var buf strings.Builder
	for _, r := range s {
		switch {
		case r == '-': // keep hyphens
			buf.WriteRune(r)
		case r == '\u2014': // m-dash to hyphen
			buf.WriteRune('-')
		case r == '\u2013': // n-dash to hyphen
			buf.WriteRune('-')
		case unicode.IsPunct(r): // all other punct to space
			buf.WriteRune(' ')
		default:
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	// 2. Remove diacritical marks
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err == nil {
		s = result
	}

	// 3. Replace non-ASCII with space
	buf.Reset()
	for _, r := range s {
		if r > 127 {
			buf.WriteRune(' ')
		} else {
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	// 4. Collapse whitespace
	s = multiSpaceRe.ReplaceAllString(s, " ")

	// 5. Normalize hyphens
	s = strings.TrimLeft(s, "-")
	s = strings.TrimRight(s, "-")
	s = spaceHyphenRe.ReplaceAllString(s, "-")
	s = multiHyphenRe.ReplaceAllString(s, "-")

	// 6. Trim whitespace
	s = strings.TrimSpace(s)

	// 7. Lowercase
	s = strings.ToLower(s)

	return s
}
