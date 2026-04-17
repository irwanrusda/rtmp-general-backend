package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
	"github.com/irwanrusda/rtmp-general-backend/app/models"
)

// autoDeactivateExpired deactivates users whose expires_at has passed
func autoDeactivateExpired() {
	config.DB.Exec(`
		UPDATE users 
		SET is_active = 0 
		WHERE expires_at IS NOT NULL 
		  AND expires_at <= NOW() 
		  AND is_active = 1
	`)
}

// Implementation of UsersHandler inside user_controller.go
func UsersHandler(w http.ResponseWriter, r *http.Request) {
	me := core.GetSessionUser(r)
	if me == nil || !me.CanManageUsers {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	if r.Method == "GET" {
		// Auto-deactivate expired accounts before listing
		autoDeactivateExpired()

		rows, err := config.DB.Query(`
			SELECT u.id, u.username, u.display_name, u.is_active, u.allowed_quality_id, u.max_stream_keys, r.name as role_name,
			       u.expires_at
			FROM users u JOIN roles r ON u.role_id = r.id
			ORDER BY u.id DESC
		`)
		if err != nil {
			core.ErrorResponse(w, 500, "Database error")
			return
		}
		defer rows.Close()

		var users []core.H
		for rows.Next() {
			var id int
			var isAct, qid, maxKeys int
			var uname, disp, rn string
			var expiresAt *string
			if err := rows.Scan(&id, &uname, &disp, &isAct, &qid, &maxKeys, &rn, &expiresAt); err == nil {
				users = append(users, core.H{
					"id":                 id,
					"username":           uname,
					"display_name":       disp,
					"is_active":          isAct == 1,
					"allowed_quality_id": qid,
					"max_stream_keys":    maxKeys,
					"role_name":          rn,
					"expires_at":         expiresAt,
				})
			}
		}
		if users == nil {
			users = []core.H{}
		}
		core.JSONResponse(w, 200, users)
		return
	}

	if r.Method == http.MethodPost {
		var in struct {
			Username         string  `json:"username"`
			DisplayName      string  `json:"display_name"`
			AllowedQualityID int     `json:"allowed_quality_id"`
			MaxStreamKeys    int     `json:"max_stream_keys"`
			RoleID           int     `json:"role_id"`
			IsActive         int     `json:"is_active"`
			ExpiresAt        *string `json:"expires_at"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		// simple hash for "123456" default password
		h := sha256.New()
		h.Write([]byte("123456"))
		pw := hex.EncodeToString(h.Sum(nil))

		// Parse expires_at if provided (frontend sends datetime-local "2006-01-02T15:04")
		var expiresAt interface{} = nil
		if in.ExpiresAt != nil && *in.ExpiresAt != "" {
			t, err := time.Parse("2006-01-02T15:04", *in.ExpiresAt)
			if err == nil {
				expiresAt = t.Format("2006-01-02 15:04:05")
			}
		}

		res, err := config.DB.Exec(`
			INSERT INTO users (username, display_name, password_hash, role_id, is_active, allowed_quality_id, max_stream_keys, expires_at, must_change_password) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			in.Username, in.DisplayName, pw, in.RoleID, in.IsActive, in.AllowedQualityID, in.MaxStreamKeys, expiresAt,
		)
		if err != nil {
			core.ErrorResponse(w, 400, "Gagal/Username sudah ada")
			return
		}
		id, _ := res.LastInsertId()

		// Generate and insert the default 360p stream key for this new user
		randStr := hex.EncodeToString(h.Sum(nil)[:8]) // Reuse hasher for random look
		config.DB.Exec(`
			INSERT INTO stream_keys (user_id, stream_key, label, is_default, override_quality_id) 
			VALUES (?, ?, 'Default (360p)', 1, 1)`,
			id, randStr,
		)

		core.JSONResponse(w, 201, core.H{"message": "User berhasil ditambahkan", "id": id})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

func UserDetailHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageUsers {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid ID")
		return
	}

	if r.Method == "GET" {
		var u models.User
		var isAct int
		err := config.DB.QueryRow(
			"SELECT id, username, display_name, role_id, is_active, allowed_quality_id, max_stream_keys, expires_at FROM users WHERE id = ?", id,
		).Scan(&u.ID, &u.Username, &u.DisplayName, &u.RoleID, &isAct, &u.AllowedQuality, &u.MaxStreamKeys, &u.ExpiresAt)
		if err != nil {
			core.ErrorResponse(w, 404, "User not found")
			return
		}
		u.IsActive = (isAct == 1)
		core.JSONResponse(w, 200, u)
		return
	}

	if r.Method == "PUT" {
		var in struct {
			Username         string  `json:"username"`
			DisplayName      string  `json:"display_name"`
			Password         string  `json:"password"`
			AllowedQualityID int     `json:"allowed_quality_id"`
			MaxStreamKeys    int     `json:"max_stream_keys"`
			RoleID           int     `json:"role_id"`
			IsActive         int     `json:"is_active"`
			ExpiresAt        *string `json:"expires_at"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		// Parse expires_at
		var expiresAt interface{} = nil
		if in.ExpiresAt != nil && *in.ExpiresAt != "" {
			t, err := time.Parse("2006-01-02T15:04", *in.ExpiresAt)
			if err == nil {
				expiresAt = t.Format("2006-01-02 15:04:05")
			}
		}

		if in.Password != "" {
			h := sha256.New()
			h.Write([]byte(in.Password))
			pw := hex.EncodeToString(h.Sum(nil))
			config.DB.Exec(
				"UPDATE users SET username=?, display_name=?, role_id=?, is_active=?, allowed_quality_id=?, max_stream_keys=?, expires_at=?, password_hash=? WHERE id=?",
				in.Username, in.DisplayName, in.RoleID, in.IsActive, in.AllowedQualityID, in.MaxStreamKeys, expiresAt, pw, id,
			)
		} else {
			config.DB.Exec(
				"UPDATE users SET username=?, display_name=?, role_id=?, is_active=?, allowed_quality_id=?, max_stream_keys=?, expires_at=? WHERE id=?",
				in.Username, in.DisplayName, in.RoleID, in.IsActive, in.AllowedQualityID, in.MaxStreamKeys, expiresAt, id,
			)
		}
		core.JSONResponse(w, 200, core.H{"message": "User diperbarui"})
		return
	}

	if r.Method == "DELETE" {
		_, err := config.DB.Exec("DELETE FROM users WHERE id = ?", id)
		if err != nil {
			core.ErrorResponse(w, 500, "Gagal menghapus user")
			return
		}
		core.JSONResponse(w, 200, core.H{"message": "User berhasil dihapus"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}
