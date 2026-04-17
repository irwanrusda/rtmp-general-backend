package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/core"
)

func RolesHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || (!auth.CanManageUsers && !auth.CanManageRoles) {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	if r.Method == "GET" {
		rows, err := config.DB.Query("SELECT id, name, can_publish, can_manage_users, can_manage_roles, max_streams FROM roles ORDER BY id")
		if err != nil {
			core.ErrorResponse(w, 500, "DB Error")
			return
		}
		defer rows.Close()

		var roles []core.H
		for rows.Next() {
			var id int
			var pub, musr, mrol, mstr int
			var name string
			if err := rows.Scan(&id, &name, &pub, &musr, &mrol, &mstr); err == nil {
				roles = append(roles, core.H{
					"id":               id,
					"name":             name,
					"can_publish":      pub == 1,
					"can_manage_users": musr == 1,
					"can_manage_roles": mrol == 1,
					"max_streams":      mstr,
				})
			}
		}
		if roles == nil {
			roles = []core.H{}
		}
		core.JSONResponse(w, 200, roles)
		return
	}
	core.ErrorResponse(w, 405, "Method Not Allowed")
}

func RolesDetailHandler(w http.ResponseWriter, r *http.Request) {
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
			CanPublish     bool `json:"can_publish"`
			CanManageUsers bool `json:"can_manage_users"`
			CanManageRoles bool `json:"can_manage_roles"`
			MaxStreams     int  `json:"max_streams"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		config.DB.Exec("UPDATE roles SET can_publish=?, can_manage_users=?, can_manage_roles=?, max_streams=? WHERE id=?",
			in.CanPublish, in.CanManageUsers, in.CanManageRoles, in.MaxStreams, id)
		core.JSONResponse(w, 200, core.H{"message": "Role diperbarui"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}

func PermissionsHandler(w http.ResponseWriter, r *http.Request) {
	if core.GetSessionUser(r) == nil {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}
	
	rows, err := config.DB.Query("SELECT id, name, label, group_name FROM permissions ORDER BY sort_order, name")
	if err != nil {
		core.JSONResponse(w, 200, []core.H{}) // Empty fallback
		return
	}
	defer rows.Close()

	var perms []core.H
	for rows.Next() {
		var id int
		var name, label, gname string
		if err := rows.Scan(&id, &name, &label, &gname); err == nil {
			perms = append(perms, core.H{"id": id, "name": name, "label": label, "group_name": gname})
		}
	}
	if perms == nil {
		perms = []core.H{}
	}
	core.JSONResponse(w, 200, perms)
}

func RolePermissionsHandler(w http.ResponseWriter, r *http.Request) {
	auth := core.GetSessionUser(r)
	if auth == nil || !auth.CanManageRoles {
		core.ErrorResponse(w, 403, "Forbidden")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		core.ErrorResponse(w, 400, "Invalid ID")
		return
	}

	if r.Method == "GET" {
		rows, _ := config.DB.Query("SELECT permission_id FROM role_permissions WHERE role_id = ?", id)
		defer rows.Close()
		var ids []int
		for rows.Next() {
			var pid int
			if rows.Scan(&pid) == nil {
				ids = append(ids, pid)
			}
		}
		if ids == nil { ids = []int{} }
		core.JSONResponse(w, 200, core.H{"role_id": id, "permission_ids": ids})
		return
	}

	if r.Method == "PUT" {
		var in struct {
			PermissionIDs []int `json:"permission_ids"`
		}
		json.NewDecoder(r.Body).Decode(&in)

		config.DB.Exec("DELETE FROM role_permissions WHERE role_id = ?", id)
		for _, pid := range in.PermissionIDs {
			config.DB.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)", id, pid)
		}
		core.JSONResponse(w, 200, core.H{"message": "Permissions diperbarui"})
		return
	}

	core.ErrorResponse(w, 405, "Method Not Allowed")
}
