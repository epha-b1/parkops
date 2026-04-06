package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"parkops/internal/auth"
)

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

// secureCookie is set to true when APP_ENV=production. Initialized in NewRouter.
var secureCookie bool

// sessionSecret is used to HMAC-sign session IDs. Set in NewRouter from config.
var sessionSecret string

// signSessionID produces an HMAC-SHA256 signature of the session ID using sessionSecret.
func signSessionID(sessionID string) string {
	if sessionSecret == "" {
		return sessionID
	}
	mac := hmac.New(sha256.New, []byte(sessionSecret))
	mac.Write([]byte(sessionID))
	return sessionID + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifySessionID validates and strips the HMAC signature from a signed session cookie value.
func verifySessionID(signed string) (string, bool) {
	if sessionSecret == "" {
		return signed, true
	}
	idx := len(signed) - 65 // 1 dot + 64 hex chars
	if idx <= 0 || signed[idx] != '.' {
		return "", false
	}
	sessionID := signed[:idx]
	expected := signSessionID(sessionID)
	if !hmac.Equal([]byte(signed), []byte(expected)) {
		return "", false
	}
	return sessionID, true
}

func setSessionCookie(c *gin.Context, sessionID string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    signSessionID(sessionID),
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.SessionTimeout.Seconds()),
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
