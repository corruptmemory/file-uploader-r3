package hashers

import (
	"fmt"
	"strings"
)

// usStateCodes contains all 50 US states plus DC.
var usStateCodes = []string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
	"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
	"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
	"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
	"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
	"DC",
}

// GetCountrySubdivisions returns the valid subdivision codes for the given
// country. The country input is trimmed and uppercased. Returns an error for
// unrecognized countries.
func GetCountrySubdivisions(country string) ([]string, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	switch country {
	case "US", "USA":
		result := make([]string, len(usStateCodes))
		copy(result, usStateCodes)
		return result, nil
	default:
		return nil, fmt.Errorf("unrecognized country: %q", country)
	}
}
