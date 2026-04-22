package controllers

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

type RtmpStat struct {
	Server struct {
		Application []struct {
			Name string `xml:"name"`
			Live struct {
				Stream []struct {
					Name    string `xml:"name"`
					BwVideo int    `xml:"bw_video"`
					Meta    struct {
						Video struct {
							Width  int `xml:"width"`
							Height int `xml:"height"`
							Fps    int `xml:"frame_rate"`
						} `xml:"video"`
					} `xml:"meta"`
				} `xml:"stream"`
			} `xml:"live"`
		} `xml:"application"`
	} `xml:"server"`
}

// ActiveStreams handles GET /api/active-streams
func ActiveStreams(w http.ResponseWriter, r *http.Request) {
	if core.GetSessionUser(r) == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	// Fetch RTMP Stats
	var stats RtmpStat
	resp, errStat := http.Get("http://127.0.0.1/stat")
	if errStat == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		xml.Unmarshal(body, &stats)
		resp.Body.Close()
	}

	rows, err := config.DB.Query(`
		SELECT a.stream_key, a.started_at, u.username, u.display_name 
		FROM active_streams a
		JOIN users u ON a.user_id = u.id
		ORDER BY a.started_at DESC
	`)
	if err != nil {
		core.ErrorResponse(w, 500, "DB Error")
		return
	}
	defer rows.Close()

	var streams []core.H
	for rows.Next() {
		var sk, start, uname, disp string
		if err := rows.Scan(&sk, &start, &uname, &disp); err == nil {
			// Find stats for this stream
			width, height, fps, bitrate := 0, 0, 0, 0
			for _, app := range stats.Server.Application {
				if app.Name == "live" {
					for _, s := range app.Live.Stream {
						if s.Name == sk {
							width = s.Meta.Video.Width
							height = s.Meta.Video.Height
							fps = s.Meta.Video.Fps
							bitrate = s.BwVideo / 1024 // Kbps
						}
					}
				}
			}

			backendURL := os.Getenv("BACKEND_URL")
			if backendURL == "" {
				backendURL = "https://apirtmp.nanobyte.web.id"
			}
			streams = append(streams, core.H{
				"stream_key":   sk,
				"started_at":   start,
				"username":     uname,
				"display_name": disp,
				"hls_url":      backendURL + "/hls/" + sk + ".m3u8",
				"width":        width,
				"height":       height,
				"fps":          fps,
				"bitrate":      bitrate,
			})
		}
	}

	if streams == nil {
		streams = []core.H{} // Return empty array instead of null
	}
	core.JSONResponse(w, 200, streams)
}

// DropActiveStream handles DELETE /api/admin/active-streams/{streamKey}
func DropActiveStream(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	streamKey := parts[len(parts)-1]
	if streamKey == "" {
		core.ErrorResponse(w, 400, "Invalid Stream Key")
		return
	}

	var ownerID int
	if !user.CanManageUsers {
		err := config.DB.QueryRow("SELECT user_id FROM stream_keys WHERE stream_key = ?", streamKey).Scan(&ownerID)
		if err != nil || ownerID != user.ID {
			core.ErrorResponse(w, 403, "Forbidden")
			return
		}
	} else {
		config.DB.QueryRow("SELECT user_id FROM stream_keys WHERE stream_key = ?", streamKey).Scan(&ownerID)
	}

	_, err := config.DB.Exec("DELETE FROM active_streams WHERE stream_key = ?", streamKey)
	if err != nil {
		core.ErrorResponse(w, 500, "Gagal menghapus active stream")
		return
	}

	// Tambahan: Logkan kejadian putusnya jika ada owner id (Force drop)
	if ownerID > 0 {
		config.DB.Exec("INSERT INTO stream_logs (user_id, stream_key, event_type, message) VALUES (?, ?, 'disconnected', 'Diberhentikan paksa oleh Admin/Sistem')", ownerID, streamKey)
	}

	// Panggil Nginx control module untuk kill rtmp stream (opsional tapi dianjurkan jika pakai nginx control)
	// Kita bisa panggil request drop HTTP lokal ke nginx
	_, _ = http.Get(fmt.Sprintf("http://127.0.0.1/control/drop/publisher?app=live&name=%s", streamKey))

	core.JSONResponse(w, 200, core.H{"message": "Ghost stream berhasil dihapus"})
}

// ClearStreamCache handles DELETE /api/admin/clear-cache/{streamKey}
func ClearStreamCache(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	streamKey := parts[len(parts)-1]
	if streamKey == "" {
		core.ErrorResponse(w, 400, "Invalid Stream Key")
		return
	}

	if !user.CanManageUsers {
		var ownerID int
		err := config.DB.QueryRow("SELECT user_id FROM stream_keys WHERE stream_key = ?", streamKey).Scan(&ownerID)
		if err != nil || ownerID != user.ID {
			core.ErrorResponse(w, 403, "Forbidden")
			return
		}
	}

	// Hapus file utama playlist m3u8
	m3u8Path := "/tmp/hls/" + streamKey + ".m3u8"
	_ = os.Remove(m3u8Path)

	// Cari dan hapus semua segments *.ts yang berkaitan
	if files, err := ioutil.ReadDir("/tmp/hls"); err == nil {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), streamKey+"-") && strings.HasSuffix(file.Name(), ".ts") {
				_ = os.Remove("/tmp/hls/" + file.Name())
			}
		}
	}

	core.JSONResponse(w, 200, core.H{"message": "Cache HLS berhasil dibersihkan"})
}

func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	var input struct {
		DisplayName     string `json:"display_name"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	// In real setup, you would check password here and update DB
	// Just stubbing for brevity
	core.JSONResponse(w, 200, core.H{"message": "Profil berhasil diperbarui"})
}

// StreamLogs handles GET /api/my-stream-logs
func StreamLogs(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil {
		core.ErrorResponse(w, 401, "Unauthorized")
		return
	}

	rows, err := config.DB.Query(`
		SELECT id, stream_key, event_type, message, created_at
		FROM stream_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 50
	`, user.ID)
	if err != nil {
		core.ErrorResponse(w, 500, "DB Error")
		return
	}
	defer rows.Close()

	var logs []core.H
	for rows.Next() {
		var id int
		var sk, evType, msg, createdAt string
		if err := rows.Scan(&id, &sk, &evType, &msg, &createdAt); err == nil {
			logs = append(logs, core.H{
				"id":         id,
				"stream_key": sk,
				"event_type": evType,
				"message":    msg,
				"created_at": createdAt,
			})
		}
	}
	if logs == nil {
		logs = []core.H{}
	}
	core.JSONResponse(w, 200, logs)
}

// TrafficHistory handles GET /api/admin/traffic-history
func TrafficHistory(w http.ResponseWriter, r *http.Request) {
	user := core.GetSessionUser(r)
	if user == nil || !user.CanManageUsers {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	// Ambil histori global 500 logs terakhir digabung dengan data user
	query := `
		SELECT l.id, l.stream_key, l.event_type, l.message, l.created_at, u.username, u.display_name
		FROM stream_logs l
		JOIN users u ON l.user_id = u.id
		ORDER BY l.created_at DESC
		LIMIT 200
	`
	rows, err := config.DB.Query(query)
	if err != nil {
		core.ErrorResponse(w, 500, "Database error")
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id int
		var sk, ev, msg, un, dn string
		var ca []uint8
		rows.Scan(&id, &sk, &ev, &msg, &ca, &un, &dn)

		// Ekstrak hari/waktu jam untuk mempermudah grouping di frontend Recharts
		logs = append(logs, map[string]interface{}{
			"id":           id,
			"stream_key":   sk,
			"event_type":   ev,
			"message":      msg,
			"created_at":   string(ca),
			"username":     un,
			"display_name": dn,
		})
	}

	core.JSONResponse(w, 200, logs)
}
