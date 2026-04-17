package controllers

import (
	"log"
	"net/http"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
)

// RtmpOnPublish handles Nginx-RTMP on_publish callback
// It receives standard application/x-www-form-urlencoded POST data
func RtmpOnPublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse error", http.StatusBadRequest)
		return
	}

	streamKey := r.FormValue("name")

	log.Printf("[RTMP] Publish attempt with key: %s\n", streamKey)

	// Admin test key shortcut
	if streamKey == "testkey123" {
		w.WriteHeader(200)
		return
	}

	var userID int
	var canPublish int
	var isActive int
	var maxStreams int

	err := config.DB.QueryRow(`
		SELECT u.id, r.can_publish, u.is_active, r.max_streams
		FROM stream_keys sk
		JOIN users u ON sk.user_id = u.id
		JOIN roles r ON u.role_id = r.id 
		WHERE sk.stream_key = ?
	`, streamKey).Scan(&userID, &canPublish, &isActive, &maxStreams)

	if err != nil {
		log.Printf("[RTMP] Invalid stream key: %s\n", streamKey)
		http.Error(w, "Invalid stream key", http.StatusUnauthorized)
		return
	}

	if isActive == 0 {
		log.Printf("[RTMP] Inactive user ID: %d\n", userID)
		http.Error(w, "User is inactive", http.StatusForbidden)
		return
	}

	if canPublish == 0 {
		log.Printf("[RTMP] User lacks publish permission ID: %d\n", userID)
		http.Error(w, "Role cannot publish", http.StatusForbidden)
		return
	}

	// Check concurrent stream limit (REMOVED: all users now have unlimited concurrent streams)
	// No limit check applied anymore.

	// Insert into active_streams
	_, err = config.DB.Exec("INSERT IGNORE INTO active_streams (user_id, stream_key) VALUES (?, ?)", userID, streamKey)
	if err != nil {
		log.Println("[RTMP] Warning, failed to track active stream:", err)
	}

	log.Printf("[RTMP] Publish GRANTED for User ID: %d\n", userID)
	w.WriteHeader(200)
}

// RtmpOnPublishDone handles Nginx-RTMP on_publish_done callback
func RtmpOnPublishDone(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(200) // Nginx doesn't care much about error here, but we should always return 200
		return
	}

	streamKey := r.FormValue("name")
	log.Printf("[RTMP] Publish DONE for key: %s\n", streamKey)

	_, _ = config.DB.Exec("DELETE FROM active_streams WHERE stream_key = ?", streamKey)
	w.WriteHeader(200)
}
