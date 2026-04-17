package core

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/irwanrusda/rtmp-general-backend/app/models"
)

// In-memory session store (Session ID -> User struct)
var sessions = sync.Map{}

type SessionData struct {
	User      models.User
	ExpiresAt time.Time
}

// CreateSession generates a session cookie and stores user data
func CreateSession(w http.ResponseWriter, user models.User) {
	b := make([]byte, 32)
	rand.Read(b)
	sessionID := hex.EncodeToString(b)

	expires := time.Now().Add(24 * time.Hour)
	sessions.Store(sessionID, SessionData{
		User:      user,
		ExpiresAt: expires,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "RTMP_SESSION",
		Value:    sessionID,
		Expires:  expires,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,            // Required for SameSite=None
		SameSite: http.SameSiteNoneMode, // Allow cross-site (frontend <-> backend different domains)
	})
}

// GetSessionUser checks if user is logged in based on cookie
func GetSessionUser(r *http.Request) *models.User {
	cookie, err := r.Cookie("RTMP_SESSION")
	if err != nil {
		return nil
	}

	val, ok := sessions.Load(cookie.Value)
	if !ok {
		return nil
	}

	sessionData := val.(SessionData)
	if time.Now().After(sessionData.ExpiresAt) {
		sessions.Delete(cookie.Value)
		return nil
	}

	return &sessionData.User
}

// DestroySession removes session cookie and memory store
func DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("RTMP_SESSION")
	if err == nil {
		sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "RTMP_SESSION",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}
