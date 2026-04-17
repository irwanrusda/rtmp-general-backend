package controllers

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

func MigrateDB(w http.ResponseWriter, r *http.Request) {
	// FIRST: Run PatchSchema to safely add any new columns (handles ALTER TABLE gracefully)
	config.PatchSchema()

	// Let's try multiple standard locations for the SQL file
	paths := []string{"./setup_local.sql", "./migrate.sql", "/www/migrate.sql"}
	
	var sqlBytes []byte
	var err error
	var loadedPath string

	for _, p := range paths {
		sqlBytes, err = ioutil.ReadFile(p)
		if err == nil {
			loadedPath = p
			break
		}
	}

	if err != nil || len(sqlBytes) == 0 {
		core.ErrorResponse(w, 500, "Migration file not found in paths")
		return
	}

	sqlQuery := string(sqlBytes)

	// Since multiStatements=true is active in DSN, we can just execute the whole file!
	_, err = config.DB.Exec(sqlQuery)
	if err != nil {
		core.ErrorResponse(w, 500, fmt.Sprintf("Migration failed: %v", err))
		return
	}

	// Fetch current table names
	rows, err := config.DB.Query("SHOW TABLES")
	var tables []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err == nil {
				tables = append(tables, tableName)
			}
		}
	}

	// Force Seed Admin (ensures superadmin is inserted/updated directly from this page)
	seedResult := config.SeedAdmin()

	// Audit helper with extra info
	audit := config.GetAuditInfo()
	audit["build_version"] = "2026-04-17-v6"
	
	core.JSONResponse(w, 200, core.H{
		"message":     "Database migrated successfully!",
		"file":        loadedPath,
		"tables":      tables,
		"seed_status": seedResult,
		"audit":       audit,
	})
}
