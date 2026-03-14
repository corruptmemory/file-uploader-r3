package auth

import (
	"fmt"
	"net/http"
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
	Exp      int64  `json:"exp"`
}

// CreateToken creates a signed HS256 JWT with the given claims and signing key.
func CreateToken(claims JWTClaims, signingKey []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": claims.Username,
		"orgID":    claims.OrgID,
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
	exp, _ := mapClaims["exp"].(float64)

	claims := &JWTClaims{
		Username: username,
		OrgID:    orgID,
		Exp:      int64(exp),
	}

	// Check expiry
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// NewClaims creates a JWTClaims with a fresh expiry.
func NewClaims(username, orgID string) JWTClaims {
	return JWTClaims{
		Username: username,
		OrgID:    orgID,
		Exp:      time.Now().Add(TokenExpiry).Unix(),
	}
}

// SetSessionCookies sets the session and session-expires cookies on the response.
func SetSessionCookies(w http.ResponseWriter, tokenStr string, expiry time.Time, prefix string) {
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
		Expires:  expiry,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session-expires",
		Value:    expiry.UTC().Format(http.TimeFormat),
		Path:     cookiePath,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiry,
	})
}

// ClearSessionCookies clears the session cookies by setting them to expired.
func ClearSessionCookies(w http.ResponseWriter, prefix string) {
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
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session-expires",
		Value:    "",
		Path:     cookiePath,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
