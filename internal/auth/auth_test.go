package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateAndParseToken(t *testing.T) {
	signingKey := []byte("test-secret-key-for-jwt")

	claims := NewClaims("alice", "org-123")
	tokenStr, err := CreateToken(claims, signingKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	parsed, err := ParseToken(tokenStr, signingKey)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	if parsed.Username != "alice" {
		t.Errorf("Username = %q, want %q", parsed.Username, "alice")
	}
	if parsed.OrgID != "org-123" {
		t.Errorf("OrgID = %q, want %q", parsed.OrgID, "org-123")
	}
	if parsed.Exp != claims.Exp {
		t.Errorf("Exp = %d, want %d", parsed.Exp, claims.Exp)
	}
	if parsed.JTI == "" {
		t.Error("JTI should not be empty")
	}
	if parsed.JTI != claims.JTI {
		t.Errorf("JTI = %q, want %q", parsed.JTI, claims.JTI)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	signingKey := []byte("test-secret-key-for-jwt")

	claims := JWTClaims{
		Username: "bob",
		OrgID:    "org-456",
		JTI:      "test-jti",
		Exp:      time.Now().Add(-1 * time.Minute).Unix(),
	}
	tokenStr, err := CreateToken(claims, signingKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	_, err = ParseToken(tokenStr, signingKey)
	if err == nil {
		t.Fatal("ParseToken should reject expired token")
	}
}

func TestWrongSigningKeyRejected(t *testing.T) {
	key1 := []byte("key-one")
	key2 := []byte("key-two")

	claims := NewClaims("charlie", "org-789")
	tokenStr, err := CreateToken(claims, key1)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	_, err = ParseToken(tokenStr, key2)
	if err == nil {
		t.Fatal("ParseToken should reject token signed with different key")
	}
}

func TestSessionExtensionWithinWindow(t *testing.T) {
	signingKey := []byte("test-secret-key-for-jwt")

	// Token expiring in 30 seconds (within 40s window)
	claims := JWTClaims{
		Username: "alice",
		OrgID:    "org-123",
		JTI:      "test-jti-ext",
		Exp:      time.Now().Add(30 * time.Second).Unix(),
	}
	tokenStr, err := CreateToken(claims, signingKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	parsed, err := ParseToken(tokenStr, signingKey)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	expiresIn := time.Until(time.Unix(parsed.Exp, 0))
	if expiresIn > ExtensionWindow {
		t.Fatalf("token should be within extension window, expires in %v", expiresIn)
	}

	// Issue new token
	newClaims := NewClaims(parsed.Username, parsed.OrgID)
	newTokenStr, err := CreateToken(newClaims, signingKey)
	if err != nil {
		t.Fatalf("CreateToken for extension: %v", err)
	}

	newParsed, err := ParseToken(newTokenStr, signingKey)
	if err != nil {
		t.Fatalf("ParseToken for extension: %v", err)
	}

	// New token should expire ~5 minutes from now
	newExpiresIn := time.Until(time.Unix(newParsed.Exp, 0))
	if newExpiresIn < 4*time.Minute || newExpiresIn > 6*time.Minute {
		t.Errorf("extended token should expire in ~5 minutes, got %v", newExpiresIn)
	}
}

func TestSessionExtensionOutsideWindow(t *testing.T) {
	signingKey := []byte("test-secret-key-for-jwt")

	// Token expiring in 60 seconds (outside 40s window)
	claims := JWTClaims{
		Username: "alice",
		OrgID:    "org-123",
		JTI:      "test-jti-ext2",
		Exp:      time.Now().Add(60 * time.Second).Unix(),
	}
	tokenStr, err := CreateToken(claims, signingKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	parsed, err := ParseToken(tokenStr, signingKey)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	expiresIn := time.Until(time.Unix(parsed.Exp, 0))
	if expiresIn <= ExtensionWindow {
		t.Fatalf("token should be outside extension window, expires in %v", expiresIn)
	}

	// No action should be taken — token is still valid and not within window
}

func TestSetSessionCookies(t *testing.T) {
	w := httptest.NewRecorder()
	expiry := time.Now().Add(5 * time.Minute)

	SetSessionCookies(w, "my-jwt-token", expiry, "/", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	var session, sessionExpires *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case "session":
			session = c
		case "session-expires":
			sessionExpires = c
		}
	}

	if session == nil {
		t.Fatal("session cookie not set")
	}
	if !session.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
	if session.Value != "my-jwt-token" {
		t.Errorf("session Value = %q, want %q", session.Value, "my-jwt-token")
	}
	if session.Secure {
		t.Error("session cookie should NOT have Secure flag when secure=false")
	}

	if sessionExpires == nil {
		t.Fatal("session-expires cookie not set")
	}
	if sessionExpires.HttpOnly {
		t.Error("session-expires cookie should NOT be HttpOnly")
	}
}

func TestClearSessionCookies(t *testing.T) {
	w := httptest.NewRecorder()

	ClearSessionCookies(w, "/", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	for _, c := range cookies {
		if c.MaxAge != -1 {
			t.Errorf("cookie %q MaxAge = %d, want -1", c.Name, c.MaxAge)
		}
	}
}

func TestTokenBlacklistRevokeAndCheck(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Close()

	jti := "test-jti-revoke"
	expiry := time.Now().Add(5 * time.Minute)

	if bl.IsRevoked(jti) {
		t.Fatal("JTI should not be revoked before calling Revoke")
	}

	bl.Revoke(jti, expiry)

	if !bl.IsRevoked(jti) {
		t.Fatal("JTI should be revoked after calling Revoke")
	}

	// A different JTI should not be revoked
	if bl.IsRevoked("other-jti") {
		t.Fatal("unrelated JTI should not be revoked")
	}
}

func TestTokenBlacklistAutoEvictsExpired(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Close()

	// Add an already-expired entry via Revoke
	bl.Revoke("expired-jti", time.Now().Add(-1*time.Minute))

	// Revoke a new JTI — this triggers eviction of the expired one
	bl.Revoke("new-jti", time.Now().Add(5*time.Minute))

	// The expired JTI should have been evicted
	if bl.IsRevoked("expired-jti") {
		t.Error("expired JTI should have been evicted")
	}

	// The new JTI should still be there
	if !bl.IsRevoked("new-jti") {
		t.Error("new JTI should still be revoked")
	}
}

func TestNewClaimsHasUniqueJTI(t *testing.T) {
	c1 := NewClaims("user1", "org1")
	c2 := NewClaims("user1", "org1")

	if c1.JTI == "" {
		t.Error("JTI should not be empty")
	}
	if c1.JTI == c2.JTI {
		t.Error("two calls to NewClaims should produce different JTIs")
	}
}

func TestSetSessionCookiesSecureFlag(t *testing.T) {
	tests := []struct {
		name   string
		secure bool
	}{
		{"secure true", true},
		{"secure false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			expiry := time.Now().Add(5 * time.Minute)
			SetSessionCookies(w, "token", expiry, "/", tt.secure)

			for _, c := range w.Result().Cookies() {
				if c.Secure != tt.secure {
					t.Errorf("cookie %q Secure = %v, want %v", c.Name, c.Secure, tt.secure)
				}
			}
		})
	}
}

func TestClearSessionCookiesSecureFlag(t *testing.T) {
	tests := []struct {
		name   string
		secure bool
	}{
		{"secure true", true},
		{"secure false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ClearSessionCookies(w, "/", tt.secure)

			for _, c := range w.Result().Cookies() {
				if c.Secure != tt.secure {
					t.Errorf("cookie %q Secure = %v, want %v", c.Name, c.Secure, tt.secure)
				}
			}
		})
	}
}
