package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

func StreamQualitiesHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageRoles {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	if r.Method == "GET" {
		rows, err := config.DB.Query("SELECT id, name, resolution, bitrate_kbps, fps, is_active, sort_order FROM stream_qualities ORDER BY sort_order, id")
		if err != nil {
			core.JSONResponse(w, 200, []core.H{})
			return
		}
		defer rows.Close()

		var list []core.H
		for rows.Next() {
			var id, kbps, fps, isAct, sort int
			var name, res string
			if err := rows.Scan(&id, &name, &res, &kbps, &fps, &isAct, &sort); err == nil {
				list = append(list, core.H{
					"id":           id,
					"name":         name,
					"resolution":   res,
					"bitrate_kbps": kbps,
					"fps":          fps,
					"is_active":    isAct == 1,
					"sort_order":   sort,
				})
			}
		}
		if list == nil { list = []core.H{} }
		core.JSONResponse(w, 200, list)
		return
	}

	if r.Method == "POST" {
		var in struct {
			Name        string `json:"name"`
			Resolution  string `json:"resolution"`
			BitrateKbps int    `json:"bitrate_kbps"`
			Fps         int    `json:"fps"`
			IsActive    int    `json:"is_active"`
			SortOrder   int    `json:"sort_order"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		res, err := config.DB.Exec("INSERT INTO stream_qualities (name, resolution, bitrate_kbps, fps, is_active, sort_order) VALUES (?,?,?,?,?,?)",
			in.Name, in.Resolution, in.BitrateKbps, in.Fps, in.IsActive, in.SortOrder)
		if err != nil {
			core.ErrorResponse(w, 400, "Gagal menambah kualitas")
			return
		}
		id, _ := res.LastInsertId()
		core.JSONResponse(w, 201, core.H{"message": "Kualitas ditambahkan", "id": id})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

func StreamQualityDetailHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageRoles {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid ID")
		return
	}

	if r.Method == "PUT" {
		var in struct {
			Name        string `json:"name"`
			Resolution  string `json:"resolution"`
			BitrateKbps int    `json:"bitrate_kbps"`
			Fps         int    `json:"fps"`
			IsActive    int    `json:"is_active"`
			SortOrder   int    `json:"sort_order"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		config.DB.Exec("UPDATE stream_qualities SET name=?, resolution=?, bitrate_kbps=?, fps=?, is_active=?, sort_order=? WHERE id=?",
			in.Name, in.Resolution, in.BitrateKbps, in.Fps, in.IsActive, in.SortOrder, id)
		core.JSONResponse(w, 200, core.H{"message": "Diperbarui"})
		return
	}

	if r.Method == "DELETE" {
		config.DB.Exec("DELETE FROM stream_qualities WHERE id = ?", id)
		core.JSONResponse(w, 200, core.H{"message": "Dihapus"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}
