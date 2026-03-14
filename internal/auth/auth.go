package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenExpiry is the JWT lifetime.
const TokenExpiry = 5 * time.Minute

// ExtensionWindow is the time before expiry within which a session can be extended.
const ExtensionWindow = 40 * time.Second

// JWTClaims holds the custom claims for session JWTs.
type JWTClaims struct {
	Username string `json:"username"`
	OrgID    string `json:"orgID"`
	JTI      string `json:"jti"`
	Exp      int64  `json:"exp"`
}

// TokenBlacklist tracks revoked JTIs with automatic expiry-based eviction.
type TokenBlacklist struct {
	mu      sync.Mutex
	entries map[string]time.Time // JTI -> expiry time
}

// NewTokenBlacklist creates a new empty blacklist.
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		entries: make(map[string]time.Time),
	}
}

// Revoke adds a JTI to the blacklist. The entry is kept until its expiry time.
func (b *TokenBlacklist) Revoke(jti string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = expiry
	// Evict expired entries while we're here
	now := time.Now()
	for k, exp := range b.entries {
		if now.After(exp) {
			delete(b.entries, k)
		}
	}
}

// IsRevoked returns true if the JTI is in the blacklist.
func (b *TokenBlacklist) IsRevoked(jti string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.entries[jti]
	return ok
}

// generateJTI creates a random 16-byte hex-encoded JWT ID.
func generateJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// CreateToken creates a signed HS256 JWT with the given claims and signing key.
func CreateToken(claims JWTClaims, signingKey []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": claims.Username,
		"orgID":    claims.OrgID,
		"jti":      claims.JTI,
		"exp":      claims.Exp,
	})
	return token.SignedString(signingKey)
}

// ParseToken validates and parses a JWT string, returning claims or an error.
func ParseToken(tokenStr string, signingKey []byte) (*JWTClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	username, _ := mapClaims["username"].(string)
	orgID, _ := mapClaims["orgID"].(string)
	jti, _ := mapClaims["jti"].(string)
	exp, _ := mapClaims["exp"].(float64)

	claims := &JWTClaims{
		Username: username,
		OrgID:    orgID,
		JTI:      jti,
		Exp:      int64(exp),
	}

	// Check expiry
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// NewClaims creates a JWTClaims with a fresh expiry and a unique JTI.
func NewClaims(username, orgID string) JWTClaims {
	return JWTClaims{
		Username: username,
		OrgID:    orgID,
		JTI:      generateJTI(),
		Exp:      time.Now().Add(TokenExpiry).Unix(),
	}
}

// SetSessionCookies sets the session and session-expires cookies on the response.
// When secure is true, the Secure flag is set on cookies (for TLS connections).
func SetSessionCookies(w http.ResponseWriter, tokenStr string, expiry time.Time, prefix string, secure bool) {
	cookiePath := "/"
	if prefix != "" {
		cookiePath = prefix
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    tokenStr,
		Path:     cookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		Expires:  expiry,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session-expires",
		Value:    expiry.UTC().Format(http.TimeFormat),
		Path:     cookiePath,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		Expires:  expiry,
	})
}

// ClearSessionCookies clears the session cookies by setting them to expired.
// When secure is true, the Secure flag is set on cookies (for TLS connections).
func ClearSessionCookies(w http.ResponseWriter, prefix string, secure bool) {
	cookiePath := "/"
	if prefix != "" {
		cookiePath = prefix
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     cookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session-expires",
		Value:    "",
		Path:     cookiePath,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}
