package core

import (
	"encoding/json"
	"net/http"
)

type H map[string]interface{}

// JSONResponse sends a standard JSON response and automatically adds success: true if code < 300
func JSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(statusCode)

	// Inject success key if it's a map and code is success
	if statusCode >= 200 && statusCode < 300 {
		if dict, ok := data.(map[string]interface{}); ok {
			if _, exists := dict["success"]; !exists {
				dict["success"] = true
			}
			data = dict
		}
	}

	json.NewEncoder(w).Encode(data)
}

// ErrorResponse sends a JSON error
func ErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	JSONResponse(w, statusCode, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
