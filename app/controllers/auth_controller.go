package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
	"github.com/irwanrusda/rtmp-general-backend/app/models"
)

func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		core.ErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		core.ErrorResponse(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	hasher := sha256.New()
	hasher.Write([]byte(input.Password))
	pwHash := hex.EncodeToString(hasher.Sum(nil))

	var u models.User
	var isAct int
	var mustChange int
	err := config.DB.QueryRow(`
		SELECT u.id, u.username, u.display_name, u.is_active, u.allowed_quality_id, u.must_change_password,
		       r.id, r.name, r.can_publish, r.can_manage_users, r.can_manage_roles, r.max_streams
		FROM users u 
		JOIN roles r ON u.role_id = r.id 
		WHERE u.username = ? AND u.password_hash = ?
	`, input.Username, pwHash).Scan(
		&u.ID, &u.Username, &u.DisplayName, &isAct, &u.AllowedQuality, &mustChange,
		&u.RoleID, &u.RoleName, &u.CanPublish, &u.CanManageUsers, &u.CanManageRoles, &u.MaxStreams,
	)

	if err != nil {
		core.ErrorResponse(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}

	if isAct == 0 {
		core.ErrorResponse(w, http.StatusForbidden, "Akun Anda dinonaktifkan")
		return
	}
	u.IsActive = (isAct == 1)
	u.MustChangePassword = (mustChange == 1)

	// Create session
	core.CreateSession(w, u)

	core.JSONResponse(w, 200, core.H{
		"message": "Login berhasil",
		"user":    u,
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	core.DestroySession(w, r)
	core.JSONResponse(w, 200, core.H{"message": "Logout berhasil"})
}

func Me(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Cek status live
	var cnt int
	_ = config.DB.QueryRow("SELECT COUNT(*) FROM active_streams WHERE user_id = ?", user.ID).Scan(&cnt)
	user.IsLive = (cnt > 0)

	core.JSONResponse(w, 200, user)
}

func ForceChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		core.ErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Password == "" {
		core.ErrorResponse(w, http.StatusBadRequest, "Password tidak boleh kosong")
		return
	}

	hasher := sha256.New()
	hasher.Write([]byte(input.Password))
	pwHash := hex.EncodeToString(hasher.Sum(nil))

	_, err := config.DB.Exec("UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?", pwHash, user.ID)
	if err != nil {
		core.ErrorResponse(w, http.StatusInternalServerError, "Gagal mengupdate password")
		return
	}

	// Password changed successfully, Destroy session to force re-login
	core.DestroySession(w, r)
	core.JSONResponse(w, 200, core.H{"message": "Password berhasil diubah. Silakan login kembali."})
}
