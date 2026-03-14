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
}

func TestExpiredTokenRejected(t *testing.T) {
	signingKey := []byte("test-secret-key-for-jwt")

	claims := JWTClaims{
		Username: "bob",
		OrgID:    "org-456",
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

	SetSessionCookies(w, "my-jwt-token", expiry, "/")

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

	if sessionExpires == nil {
		t.Fatal("session-expires cookie not set")
	}
	if sessionExpires.HttpOnly {
		t.Error("session-expires cookie should NOT be HttpOnly")
	}
}

func TestClearSessionCookies(t *testing.T) {
	w := httptest.NewRecorder()

	ClearSessionCookies(w, "/")

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
