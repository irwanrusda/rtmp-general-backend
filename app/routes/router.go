package routes

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/controllers"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

// CORSMiddleware is removed because Nginx handles CORS directly.

// InitRouter maps URL paths to Go functions
func InitRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// ── RTMP Nginx Callbacks ─────────
	mux.HandleFunc("/auth", controllers.RtmpOnPublish)
	mux.HandleFunc("/auth_done", controllers.RtmpOnPublishDone)

	// ── API Routes ───────────────────
	mux.HandleFunc("/api/login", controllers.Login)
	mux.HandleFunc("/api/logout", controllers.Logout)
	mux.HandleFunc("/api/me", controllers.Me)
	mux.HandleFunc("/api/force-change-password", controllers.ForceChangePassword)
	mux.HandleFunc("/api/migrate", controllers.MigrateDB)

	mux.HandleFunc("/api/active-streams", controllers.ActiveStreams)
	mux.HandleFunc("/api/admin/active-streams/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			controllers.DropActiveStream(w, r)
		} else {
			core.ErrorResponse(w, 405, "Method not allowed")
		}
	})
	mux.HandleFunc("/api/profile", controllers.UpdateProfile)
	mux.HandleFunc("/api/my-stream-logs", controllers.StreamLogs)

	// Users
	mux.HandleFunc("/api/users", controllers.UsersHandler)
	mux.HandleFunc("/api/users/", controllers.UserDetailHandler)

	// Stream Keys
	mux.HandleFunc("/api/my-stream-keys", controllers.ListStreamKeysHandler)
	mux.HandleFunc("/api/my-stream-keys/", controllers.MyStreamKeyDetailHandler)
	mux.HandleFunc("/api/admin/stream-keys/", controllers.AdminStreamKeysHandler)
	mux.HandleFunc("/api/admin/stream-key/", controllers.AdminStreamKeyDeleteHandler)

	// Roles & Permissions
	mux.HandleFunc("/api/roles", controllers.RolesHandler)
	mux.HandleFunc("/api/roles/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/permissions") {
			controllers.RolePermissionsHandler(w, r)
		} else {
			controllers.RolesDetailHandler(w, r)
		}
	})
	mux.HandleFunc("/api/permissions", controllers.PermissionsHandler)

	// Master Data
	mux.HandleFunc("/api/master/stream-qualities", controllers.StreamQualitiesHandler)
	mux.HandleFunc("/api/master/stream-qualities/", controllers.StreamQualityDetailHandler)

	// ── 404 fallback (no static files — served by frontend container) ───
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		core.ErrorResponse(w, http.StatusNotFound, "Not found")
	})

	return mux
}

// Logger middleware
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[%s] %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
