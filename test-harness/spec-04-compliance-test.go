//go:build ignore
// +build ignore

// This is a standalone compliance test for spec 04. Run with:
//   go run test-harness/spec-04-compliance-test.go

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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

// Reference implementation of ProcessName from spec
func refProcessName(in string) string {
	current := in
	for {
		next := refProcessNameOnce(current)
		if next == current {
			return next
		}
		current = next
	}
}

func refProcessNameOnce(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch {
		case r == '-':
			buf.WriteRune(r)
		case r == '\u2014':
			buf.WriteRune('-')
		case r == '\u2013':
			buf.WriteRune('-')
		case unicode.IsPunct(r):
			buf.WriteRune(' ')
		default:
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err == nil {
		s = result
	}

	buf.Reset()
	for _, r := range s {
		if r > 127 {
			buf.WriteRune(' ')
		} else {
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	// Collapse whitespace
	fields := strings.Fields(s)
	s = strings.Join(fields, " ")

	// Normalize hyphens
	s = strings.TrimLeft(s, "-")
	s = strings.TrimRight(s, "-")
	// Remove whitespace adjacent to hyphens
	for strings.Contains(s, " -") || strings.Contains(s, "- ") {
		s = strings.ReplaceAll(s, " -", "-")
		s = strings.ReplaceAll(s, "- ", "-")
	}
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	return s
}

// Reference argon2 hash
func refArgon2Hash(cleartext, pepper string) string {
	hash := argon2.IDKey([]byte(cleartext), []byte(pepper), 3, 32*1024, 4, 32)
	return base64.StdEncoding.EncodeToString(hash)
}

// Reference PlayerUniqueHasher (spec section 3.1)
func refPlayerUniqueHasher(useFirstLetter bool, uniqueIDPepper string, last4SSN, firstName, lastName, dob string) string {
	// Step 1: if flag true and len > 1, truncate to first character
	if useFirstLetter && len(firstName) > 1 {
		firstName = firstName[:1]
	}
	// Step 2: apply name processor to firstName
	firstName = refProcessName(firstName)
	// Step 3: apply name processor to lastName
	lastName = refProcessName(lastName)
	// Step 4: take only first character of normalized firstName
	if len(firstName) > 0 {
		firstName = firstName[:1]
	}
	// Step 5: construct cleartext (no delimiters)
	cleartext := uniqueIDPepper + last4SSN + firstName + lastName + dob
	// Step 6-7: hash and encode
	return refArgon2Hash(cleartext, uniqueIDPepper)
}

// Reference OrganizationPlayerIDHasher (spec section 3.2)
func refOrgPlayerIDHasher(orgPlayerIDPepper, playerID, country, state string) string {
	cleartext := orgPlayerIDPepper + ":" + playerID + ":" + country + ":" + state
	return refArgon2Hash(cleartext, orgPlayerIDPepper)
}

func main() {
	fmt.Println("=== Spec 04: Compliance Verification ===")

	fmt.Println("\n--- Argon2 Reference Values ---")
	// Verify our reference matches known output from the test file
	known := refArgon2Hash("test-cleartext", "test-pepper")
	check("Known argon2 output matches",
		known == "sdVwUjRzr12y+w4B1mhx9bEJ/Zb6T+Cqx8XDrZI8Ang=",
		fmt.Sprintf("got %q", known))

	fmt.Println("\n--- ProcessName Compliance ---")
	cases := []struct{ in, out string }{
		{"Zoe-Andre O'Brien", "zoe-andre o brien"},
		{"  MARY--ANN  ", "mary-ann"},
		{"Jean---Pierre", "jean-pierre"},
		{"Müller", "muller"},
		{"François", "francois"},
		{"Björk", "bjork"},
		{"Dvořák", "dvorak"},
		{"", ""},
		{"SMITH", "smith"},
		{"Muñoz", "munoz"},
		{"O'Brien", "o brien"},
		{"Smith-Jones", "smith-jones"},
		{"John  Smith", "john smith"},
		{"  Smith  ", "smith"},
		{"Ñoño", "nono"},
	}
	for _, c := range cases {
		got := refProcessName(c.in)
		check(fmt.Sprintf("ProcessName(%q) == %q", c.in, c.out),
			got == c.out,
			fmt.Sprintf("got %q", got))
	}

	fmt.Println("\n--- PlayerUniqueHasher Cleartext Compliance ---")
	// Verify the cleartext construction matches spec:
	// pepper + last4SSN + firstName[:1] + lastName + dob
	pepper := "test-unique-pepper"
	h1 := refPlayerUniqueHasher(false, pepper, "1234", "John", "Smith", "19900101")
	// Manual construction: pepper + "1234" + "j" + "smith" + "19900101"
	manualCleartext := pepper + "1234" + "j" + "smith" + "19900101"
	h1Manual := refArgon2Hash(manualCleartext, pepper)
	check("PlayerUniqueHasher cleartext matches manual construction",
		h1 == h1Manual,
		fmt.Sprintf("ref=%q manual=%q", h1, h1Manual))

	fmt.Println("\n--- OrgPlayerIDHasher Cleartext Compliance ---")
	orgPepper := "test-org-pepper"
	h2 := refOrgPlayerIDHasher(orgPepper, "P001", "US", "NJ")
	// Manual construction: pepper + ":" + playerID + ":" + country + ":" + state
	orgManual := refArgon2Hash(orgPepper+":"+"P001"+":"+"US"+":"+"NJ", orgPepper)
	check("OrgPlayerIDHasher cleartext matches manual construction",
		h2 == orgManual,
		fmt.Sprintf("ref=%q manual=%q", h2, orgManual))

	// Verify different state produces different hash
	h3 := refOrgPlayerIDHasher(orgPepper, "P001", "US", "CA")
	check("Different state produces different hash", h2 != h3, "hashes should differ")

	fmt.Println("\n--- M-dash and N-dash Handling ---")
	// M-dash (—) should become hyphen
	check("M-dash becomes hyphen", refProcessName("Jean\u2014Pierre") == "jean-pierre",
		fmt.Sprintf("got %q", refProcessName("Jean\u2014Pierre")))
	// N-dash (–) should become hyphen
	check("N-dash becomes hyphen", refProcessName("Jean\u2013Pierre") == "jean-pierre",
		fmt.Sprintf("got %q", refProcessName("Jean\u2013Pierre")))

	fmt.Println()
	fmt.Printf("=== Results: PASS=%d FAIL=%d ===\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
