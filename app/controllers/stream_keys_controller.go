package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
	"github.com/irwanrusda/rtmp-general-backend/app/models"
)

// ListStreamKeysHandler is for the User Dashboard to view their own keys.
func ListStreamKeysHandler(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	if r.Method == "GET" {
		rows, err := config.DB.Query(`
			SELECT sk.id, sk.user_id, sk.stream_key, sk.label, sk.is_default, sk.override_quality_id, sk.created_at, 
			       IF(ast.id IS NOT NULL, 1, 0) as is_live
			FROM stream_keys sk 
			LEFT JOIN active_streams ast ON sk.stream_key = ast.stream_key 
			WHERE sk.user_id = ? ORDER BY sk.is_default DESC, sk.id
		`, user.ID)
		if err != nil {
			core.ErrorResponse(w, 500, "DB Error")
			return
		}
		defer rows.Close()

		var keys []models.StreamKey
		for rows.Next() {
			var k models.StreamKey
			var isLiveInt int
			var isDefaultInt int
			var override sql.NullInt32
			if err := rows.Scan(&k.ID, &k.UserID, &k.StreamKey, &k.Label, &isDefaultInt, &override, &k.CreatedAt, &isLiveInt); err == nil {
				k.IsLive = (isLiveInt == 1)
				k.IsDefault = (isDefaultInt == 1)
				if override.Valid {
					val := int(override.Int32)
					k.OverrideQualityID = &val
				}
				keys = append(keys, k)
			}
		}
		if keys == nil {
			keys = []models.StreamKey{}
		}

		core.JSONResponse(w, 200, keys)
		return
	}

	if r.Method == "POST" {
		var in struct {
			Label     string `json:"label"`
			StreamKey string `json:"stream_key"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		// Check limit (exclude default keys from quota)
		var currentKeys int
		config.DB.QueryRow("SELECT COUNT(*) FROM stream_keys WHERE user_id = ? AND is_default = 0", user.ID).Scan(&currentKeys)

		if currentKeys >= user.MaxStreamKeys {
			core.ErrorResponse(w, 400, "Limit reached. You can only hold "+strconv.Itoa(user.MaxStreamKeys)+" keys.")
			return
		}

		if in.Label == "" {
			in.Label = "Main Camera"
		}

		sk := in.StreamKey
		if sk == "" {
			b := make([]byte, 8)
			rand.Read(b)
			sk = hex.EncodeToString(b)
		} else {
			// Basic Validation for custom string
			for _, char := range sk {
				if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' && char != '-' {
					core.ErrorResponse(w, 400, "Stream Key can only contain letters, numbers, dash (-) and underscore (_)")
					return
				}
			}
		}

		res, err := config.DB.Exec("INSERT INTO stream_keys (user_id, stream_key, label) VALUES (?, ?, ?)", user.ID, sk, in.Label)
		if err != nil {
			core.ErrorResponse(w, 500, "Failed to create stream key. Identifier might be taken already.")
			return
		}

		id, _ := res.LastInsertId()
		core.JSONResponse(w, 201, core.H{"message": "Stream Key generated", "id": id, "stream_key": sk})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

func MyStreamKeyDetailHandler(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	idStr := parts[len(parts)-1]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid ID")
		return
	}

	if r.Method == "DELETE" {
		res, err := config.DB.Exec("DELETE FROM stream_keys WHERE id = ? AND user_id = ? AND is_default = 0", id, user.ID)
		if err != nil {
			core.ErrorResponse(w, 500, "DB Error")
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			core.ErrorResponse(w, 404, "Stream key not found or you don't have permission")
			return
		}

		core.JSONResponse(w, 200, core.H{"message": "Stream Key deleted"})
		return
	}

	if r.Method == "PUT" {
		var in struct {
			Label     string `json:"label"`
			StreamKey string `json:"stream_key"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		if in.Label == "" || in.StreamKey == "" {
			core.ErrorResponse(w, 400, "Label and Stream Key cannot be empty")
			return
		}

		for _, char := range in.StreamKey {
			if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' && char != '-' {
				core.ErrorResponse(w, 400, "Stream Key can only contain letters, numbers, dash (-) and underscore (_)")
				return
			}
		}

		res, err := config.DB.Exec("UPDATE stream_keys SET label = ?, stream_key = ? WHERE id = ? AND user_id = ? AND is_default = 0", in.Label, in.StreamKey, id, user.ID)
		if err != nil {
			core.ErrorResponse(w, 500, "Failed to update. Stream Key identifier might already be used.")
			return
		}

		aff, _ := res.RowsAffected()
		if aff == 0 {
			core.ErrorResponse(w, 404, "No matching stream key found or no changes made")
			return
		}

		core.JSONResponse(w, 200, core.H{"message": "Stream Key updated successfully"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

// AdminStreamKeysHandler is for Superadmin to manage keys for ANY user.
// GET /api/admin/stream-keys/{user_id}
// POST /api/admin/stream-keys/{user_id}
func AdminStreamKeysHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageUsers {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	// Path should be /api/admin/stream-keys/1
	userID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid User ID")
		return
	}

	if r.Method == "GET" {
		rows, err := config.DB.Query(`
			SELECT sk.id, sk.user_id, sk.stream_key, sk.label, sk.created_at,
			       IF(ast.id IS NOT NULL, 1, 0) as is_live
			FROM stream_keys sk 
			LEFT JOIN active_streams ast ON sk.stream_key = ast.stream_key 
			WHERE sk.user_id = ? ORDER BY sk.id
		`, userID)
		if err != nil {
			core.ErrorResponse(w, 500, "DB Error")
			return
		}
		defer rows.Close()

		var keys []models.StreamKey
		for rows.Next() {
			var k models.StreamKey
			var isLiveInt int
			if err := rows.Scan(&k.ID, &k.UserID, &k.StreamKey, &k.Label, &k.CreatedAt, &isLiveInt); err == nil {
				k.IsLive = (isLiveInt == 1)
				keys = append(keys, k)
			}
		}
		if keys == nil {
			keys = []models.StreamKey{}
		}
		core.JSONResponse(w, 200, keys)
		return
	}

	if r.Method == "POST" {
		var in struct {
			StreamKey string `json:"stream_key"`
			Label     string `json:"label"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		sk := in.StreamKey
		if sk == "" {
			b := make([]byte, 8) // 16 char string
			rand.Read(b)
			sk = hex.EncodeToString(b)
		}

		var maxKeys, currentKeys int
		config.DB.QueryRow("SELECT max_stream_keys FROM users WHERE id = ?", userID).Scan(&maxKeys)
		config.DB.QueryRow("SELECT COUNT(*) FROM stream_keys WHERE user_id = ?", userID).Scan(&currentKeys)

		if currentKeys >= maxKeys {
			core.ErrorResponse(w, 400, "Batas maksimal stream key untuk user ini tercapai")
			return
		}

		if in.Label == "" {
			in.Label = "Kamera"
		}

		res, err := config.DB.Exec("INSERT INTO stream_keys (user_id, stream_key, label) VALUES (?, ?, ?)", userID, sk, in.Label)
		if err != nil {
			core.ErrorResponse(w, 400, "Gagal membuat Stream Key (mungkin duplikat)")
			return
		}
		
		id, _ := res.LastInsertId()
		core.JSONResponse(w, 201, core.H{"message": "Stream Key ditambahkan", "id": id, "stream_key": sk})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

// AdminStreamKeyDeleteHandler
// DELETE /api/admin/stream-key/{id}
func AdminStreamKeyDeleteHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageUsers {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	keyID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid Key ID")
		return
	}

	if r.Method == "DELETE" {
		_, err := config.DB.Exec("DELETE FROM stream_keys WHERE id = ?", keyID)
		if err != nil {
			core.ErrorResponse(w, 500, "Gagal menghapus stream key")
			return
		}
		core.JSONResponse(w, 200, core.H{"message": "Stream Key berhasil dihapus"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}
