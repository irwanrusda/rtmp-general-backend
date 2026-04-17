package config

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost" // Fail-safe default for local development
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "root"
	}
	pass := os.Getenv("DB_PASSWORD")
	if pass == "" && host != "localhost" {
		pass = os.Getenv("DB_PASS")
		if pass == "" {
			pass = "506cef66db0e2af91eac" // default Easypanel fallback
		}
	} else if host == "localhost" {
		// on local XAMPP, root usually has no password
		pass = ""
	}

	name := os.Getenv("DB_NAME")
	if name == "" {
		name = "rtmp"
	}

	// Create DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&multiStatements=true", user, pass, host, port, name)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to MySQL: ", err)
	}

	if err = DB.Ping(); err != nil {
		log.Println("Warning: Could not ping database: ", err)
	} else {
		log.Println("Database connection established!")
	}
}

func PatchSchema() {
	// Add missing columns if they don't exist
	// We handle errors gracefully because MySQL doesn't have 'ADD COLUMN IF NOT EXISTS' in older versions
	DB.Exec("ALTER TABLE users ADD COLUMN display_name VARCHAR(255) AFTER password_hash")
	// DB.Exec("ALTER TABLE users ADD COLUMN stream_key VARCHAR(100) AFTER display_name") // DEPRECATED
	DB.Exec("ALTER TABLE users ADD COLUMN role_id INT DEFAULT 3 AFTER password_hash")
	DB.Exec("ALTER TABLE users ADD COLUMN is_active TINYINT(1) DEFAULT 1 AFTER role_id")
	DB.Exec("ALTER TABLE users ADD COLUMN expires_at DATETIME DEFAULT NULL AFTER is_active")
	DB.Exec("ALTER TABLE `users` ADD COLUMN `max_stream_keys` int(11) DEFAULT 3 AFTER `expires_at`")
	DB.Exec("ALTER TABLE `users` ADD COLUMN `allowed_quality_id` int(11) DEFAULT 2 AFTER `is_active`")
	DB.Exec("ALTER TABLE `users` ADD COLUMN `must_change_password` TINYINT(1) DEFAULT 0 AFTER `password_hash`")

	DB.Exec("ALTER TABLE roles ADD COLUMN max_streams INT DEFAULT 1 AFTER can_manage_roles")

	// Create stream_keys table
	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS stream_keys (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id INT NOT NULL,
		stream_key VARCHAR(100) NOT NULL UNIQUE,
		label VARCHAR(100) DEFAULT 'Kamera',
		is_default TINYINT(1) DEFAULT 0,
		override_quality_id INT DEFAULT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)

	DB.Exec("ALTER TABLE stream_keys ADD COLUMN is_default TINYINT(1) DEFAULT 0 AFTER label")
	DB.Exec("ALTER TABLE stream_keys ADD COLUMN override_quality_id INT DEFAULT NULL AFTER is_default")

	// Inject 360p Default Keys for existing users who don't have one
	// Using LEFT(UUID(),16) instead of MD5() for maximum MySQL/MariaDB compatibility
	DB.Exec(`
		INSERT INTO stream_keys (user_id, stream_key, label, is_default, override_quality_id)
		SELECT u.id, REPLACE(LEFT(UUID(), 16), '-', ''), 'Default (360p)', 1, 1
		FROM users u
		LEFT JOIN stream_keys sk ON u.id = sk.user_id AND sk.is_default = 1
		WHERE sk.id IS NULL
	`)

	if err == nil {
		// Migrate old data if stream_key column still exists
		DB.Exec(`INSERT IGNORE INTO stream_keys (user_id, stream_key, label)
			SELECT id, stream_key, 'Kamera Utama' FROM users WHERE stream_key IS NOT NULL`)

		// Drop deprecated column
		DB.Exec("ALTER TABLE users DROP COLUMN stream_key")
	}

	// Create stream_logs table
	DB.Exec(`CREATE TABLE IF NOT EXISTS stream_logs (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id INT NOT NULL,
		stream_key VARCHAR(100) DEFAULT NULL,
		event_type ENUM('connected','rejected','disconnected') NOT NULL,
		message TEXT DEFAULT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
}

func SeedAdmin() string {
	PatchSchema()

	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}

	h := sha256.New()
	h.Write([]byte(password))
	hash := hex.EncodeToString(h.Sum(nil))

	// Ensure roles exists
	DB.Exec(`INSERT IGNORE INTO roles (id, name, can_publish, can_manage_users, can_manage_roles, max_streams) VALUES 
		(1, 'superadmin', 1, 1, 1, 99),
		(2, 'admin', 1, 1, 0, 5),
		(3, 'streamer', 1, 0, 0, 1),
		(4, 'viewer', 0, 0, 0, 0)
	`)

	// Upsert admin
	var adminID int
	DB.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&adminID)

	if adminID == 0 {
		res, err := DB.Exec(`
			INSERT INTO users (username, password_hash, display_name, role_id, is_active)
			VALUES (?, ?, 'Super Administrator', 1, 1)
		`, username, hash)
		if err != nil {
			log.Println("[Warning] Gagal seed Superadmin:", err)
		} else if res != nil {
			id, _ := res.LastInsertId()
			adminID = int(id)
		}
	}

	// Ensure admin has a stream key
	if adminID > 0 {
		DB.Exec("INSERT IGNORE INTO stream_keys (user_id, stream_key, label) VALUES (?, 'admin-stream-key', 'Utama')", adminID)
	}

	return fmt.Sprintf("Admin user '%s' seeded/updated successfully.", username)
}

func GetAuditInfo() map[string]interface{} {
	var userCount int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)

	var adminFound bool
	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}

	var dbHash string
	err := DB.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&dbHash)
	if err == nil {
		adminFound = true
	}

	// Calculate expected hash for current ADMIN_PASSWORD
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}
	h := sha256.New()
	h.Write([]byte(password))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	return map[string]interface{}{
		"total_users":       userCount,
		"admin_username":    username,
		"admin_found":       adminFound,
		"db_hash_match":     dbHash == expectedHash,
		"debug_hash_prefix": expectedHash[:8], // Only show prefix for security
	}
}
