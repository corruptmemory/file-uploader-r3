package hashers

import (
	"testing"
)

// mockPlayerDB is a simple in-memory PlayerDB for testing.
type mockPlayerDB struct {
	entries map[string]string // key: playerID+":"+country+":"+state -> metaID
}

func newMockPlayerDB() *mockPlayerDB {
	return &mockPlayerDB{entries: make(map[string]string)}
}

func (m *mockPlayerDB) GetPlayerByOrgPlayerID(playerID, country, state string) (string, bool) {
	key := playerID + ":" + country + ":" + state
	metaID, found := m.entries[key]
	return metaID, found
}

func (m *mockPlayerDB) AddEntry(metaID, playerID, country, state string) {
	key := playerID + ":" + country + ":" + state
	m.entries[key] = metaID
}

func (m *mockPlayerDB) Save(path string) error {
	return nil
}

// --- Argon2 Hashing Tests ---

func TestArgon2KnownOutput(t *testing.T) {
	// Hardcoded regression test: precomputed with pepper="test-pepper", cleartext="test-cleartext",
	// argon2id time=3, mem=32*1024, par=4, keyLen=32.
	expected := "sdVwUjRzr12y+w4B1mhx9bEJ/Zb6T+Cqx8XDrZI8Ang="

	got := argon2Hash("test-cleartext", "test-pepper")
	if got != expected {
		t.Errorf("argon2Hash mismatch: got %q, want %q", got, expected)
	}
}

func TestArgon2Determinism(t *testing.T) {
	pepper := "determinism-pepper"
	cleartext := "determinism-input"
	h1 := argon2Hash(cleartext, pepper)
	h2 := argon2Hash(cleartext, pepper)
	if h1 != h2 {
		t.Errorf("argon2Hash is not deterministic: %q != %q", h1, h2)
	}
}

func TestArgon2DifferentInputs(t *testing.T) {
	pepper := "diff-pepper"
	h1 := argon2Hash("input-a", pepper)
	h2 := argon2Hash("input-b", pepper)
	if h1 == h2 {
		t.Errorf("different inputs should produce different hashes")
	}
}

func TestArgon2Parameters(t *testing.T) {
	// Verify output length: base64 of 32 bytes = 44 chars (with padding).
	got := argon2Hash("param-test", "param-pepper")
	if len(got) != 44 {
		t.Errorf("expected 44-char base64 output (32 bytes), got %d chars", len(got))
	}
}

// --- PlayerDataHasher Tests ---

func TestPlayerUniqueHasherDeterministic(t *testing.T) {
	db := newMockPlayerDB()
	h := NewPlayerDataHasher(false, "", "pepper1", "pepper2", ProcessName, db)
	defer h.Close()
	r1 := h.PlayerUniqueHasher("1234", "John", "Smith", "19900101")
	r2 := h.PlayerUniqueHasher("1234", "John", "Smith", "19900101")
	if r1 != r2 {
		t.Errorf("PlayerUniqueHasher not deterministic: %q != %q", r1, r2)
	}
}

func TestOrgPlayerIDHasherUsesCache(t *testing.T) {
	db := newMockPlayerDB()
	h := NewPlayerDataHasher(false, "", "pepper1", "pepper2", ProcessName, db)
	defer h.Close()

	// First call should compute and cache.
	r1 := h.OrganizationPlayerIDHasher("P001", "US", "NJ")
	if len(db.entries) != 1 {
		t.Errorf("expected 1 cache entry after first call, got %d", len(db.entries))
	}

	// Second call should return cached value.
	r2 := h.OrganizationPlayerIDHasher("P001", "US", "NJ")
	if r1 != r2 {
		t.Errorf("cached result mismatch: %q != %q", r1, r2)
	}

	// Pre-seed with a known value to verify cache is actually used.
	db.AddEntry("fake-meta-id", "P002", "US", "CA")
	r3 := h.OrganizationPlayerIDHasher("P002", "US", "CA")
	if r3 != "fake-meta-id" {
		t.Errorf("expected cached 'fake-meta-id', got %q", r3)
	}
}

func TestOrgPlayerIDHasherIncludesCountryState(t *testing.T) {
	db := newMockPlayerDB()
	h := NewPlayerDataHasher(false, "", "pepper1", "pepper2", ProcessName, db)
	defer h.Close()

	r1 := h.OrganizationPlayerIDHasher("P001", "US", "NJ")
	r2 := h.OrganizationPlayerIDHasher("P001", "US", "CA")
	if r1 == r2 {
		t.Errorf("same playerID with different state should produce different hashes")
	}
}

func TestPlayerUniqueHasherFirstLetterFlag(t *testing.T) {
	db := newMockPlayerDB()
	hFull := NewPlayerDataHasher(false, "", "pepper1", "pepper2", ProcessName, db)
	defer hFull.Close()
	hFirst := NewPlayerDataHasher(true, "", "pepper1", "pepper2", ProcessName, db)
	defer hFirst.Close()

	// For ASCII names, the flag doesn't change the first char (both paths yield "j").
	rFull := hFull.PlayerUniqueHasher("1234", "Jonathan", "Smith", "19900101")
	rFirst := hFirst.PlayerUniqueHasher("1234", "Jonathan", "Smith", "19900101")
	if rFull != rFirst {
		t.Errorf("ASCII name should give same hash regardless of flag: full=%q first=%q", rFull, rFirst)
	}

	// For a non-ASCII first character (e.g. "Élodie"), the flag truncates to the
	// first byte before normalization, producing a different first-name component.
	// Flag=true: "É"[:1] = broken UTF-8 byte → ProcessName strips it → empty firstName.
	// Flag=false: ProcessName("Élodie") → "elodie" → first char "e".
	rFullAccent := hFull.PlayerUniqueHasher("1234", "\u00c9lodie", "Smith", "19900101")
	rFirstAccent := hFirst.PlayerUniqueHasher("1234", "\u00c9lodie", "Smith", "19900101")
	if rFullAccent == rFirstAccent {
		t.Errorf("non-ASCII firstName should produce different hashes with/without flag")
	}

	// Single-char firstName: the flag has no effect (len <= 1, guard skips truncation).
	rFull1 := hFull.PlayerUniqueHasher("1234", "J", "Smith", "19900101")
	rFirst1 := hFirst.PlayerUniqueHasher("1234", "J", "Smith", "19900101")
	if rFull1 != rFirst1 {
		t.Errorf("single-char firstName should give same result regardless of flag")
	}
}

// --- Name Normalization Tests ---

func TestProcessName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SMITH", "smith"},
		{"Muller", "muller"},
		{"M\u00fcller", "muller"},      // Müller
		{"Fran\u00e7ois", "francois"},  // François
		{"Mu\u00f1oz", "munoz"},        // Muñoz
		{"O'Brien", "o brien"},         // apostrophe -> space
		{"Smith-Jones", "smith-jones"}, // hyphen preserved
		{"John  Smith", "john smith"},  // collapse whitespace
		{"  Smith  ", "smith"},         // trim
		{"\u00d1o\u00f1o", "nono"},     // Ñoño
		{"", ""},                       // empty
		{"Bj\u00f6rk", "bjork"},        // Björk
		{"Dvo\u0159\u00e1k", "dvorak"}, // Dvořák
		{"Zoe-Andre O'Brien", "zoe-andre o brien"},
		{"  MARY--ANN  ", "mary-ann"},
		{"Jean---Pierre", "jean-pierre"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ProcessName(tt.input)
			if got != tt.expected {
				t.Errorf("ProcessName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- Country/State Tests ---

func TestGetCountrySubdivisionsUS(t *testing.T) {
	codes, err := GetCountrySubdivisions("US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(codes) != 51 {
		t.Errorf("expected 51 codes, got %d", len(codes))
	}
}

func TestGetCountrySubdivisionsUSA(t *testing.T) {
	codesUS, _ := GetCountrySubdivisions("US")
	codesUSA, err := GetCountrySubdivisions("USA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(codesUS) != len(codesUSA) {
		t.Errorf("US and USA should return same codes")
	}
	for i := range codesUS {
		if codesUS[i] != codesUSA[i] {
			t.Errorf("mismatch at index %d: %q != %q", i, codesUS[i], codesUSA[i])
		}
	}
}

func TestGetCountrySubdivisionsCaseInsensitive(t *testing.T) {
	_, err := GetCountrySubdivisions("us")
	if err != nil {
		t.Errorf("lowercase 'us' should be accepted: %v", err)
	}
	_, err = GetCountrySubdivisions("  Usa  ")
	if err != nil {
		t.Errorf("padded 'Usa' should be accepted: %v", err)
	}
}

func TestGetCountrySubdivisionsUnknown(t *testing.T) {
	_, err := GetCountrySubdivisions("XX")
	if err == nil {
		t.Errorf("expected error for unknown country 'XX'")
	}
}

func TestGetCountrySubdivisionsAll51Codes(t *testing.T) {
	expected := []string{
		"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
		"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
		"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
		"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
		"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
		"DC",
	}
	codes, err := GetCountrySubdivisions("US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	codeSet := make(map[string]bool)
	for _, c := range codes {
		codeSet[c] = true
	}
	for _, e := range expected {
		if !codeSet[e] {
			t.Errorf("missing state code: %q", e)
		}
	}
}
