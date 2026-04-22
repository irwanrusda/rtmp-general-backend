package routes

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/controllers"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

// CORSMiddleware is the SOLE source of CORS headers (Nginx + Traefik have no CORS).
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "https://rtmp.nanobyte.web.id"
		}

		origin := r.Header.Get("Origin")
		if origin == frontendURL || strings.HasPrefix(origin, "http://localhost") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

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
	mux.HandleFunc("/api/admin/traffic-history", controllers.TrafficHistory)
	mux.HandleFunc("/api/admin/active-streams/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			controllers.DropActiveStream(w, r)
		} else {
			core.ErrorResponse(w, 405, "Method not allowed")
		}
	})
	mux.HandleFunc("/api/admin/clear-cache/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			controllers.ClearStreamCache(w, r)
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
