package models

type User struct {
	ID             int     `json:"id"`
	Username       string  `json:"username"`
	DisplayName    string  `json:"display_name"`
	RoleID         int     `json:"role_id"`
	RoleName       string  `json:"role"`
	IsActive       bool    `json:"is_active"`
	IsLive         bool    `json:"is_live"`
	MaxStreams      int     `json:"max_streams"`
	MaxStreamKeys  int     `json:"max_stream_keys"`
	AllowedQuality int     `json:"allowed_quality_id"`
	ExpiresAt      *string `json:"expires_at"` // nil = no expiry
	CanPublish     bool    `json:"can_publish"`
	CanManageUsers bool    `json:"can_manage_users"`
	CanManageRoles bool    `json:"can_manage_roles"`
	MustChangePassword bool `json:"must_change_password"`
}

type StreamKey struct {
	ID                int    `json:"id"`
	UserID            int    `json:"user_id"`
	StreamKey         string `json:"stream_key"`
	Label             string `json:"label"`
	IsDefault         bool   `json:"is_default"`
	OverrideQualityID *int   `json:"override_quality_id"`
	CreatedAt         string `json:"created_at"`
	IsLive            bool   `json:"is_live"`
}

type Role struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CanPublish     bool   `json:"can_publish"`
	CanManageUsers bool   `json:"can_manage_users"`
	CanManageRoles bool   `json:"can_manage_roles"`
	MaxStreams     int    `json:"max_streams"`
}

type StreamQuality struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Resolution  string `json:"resolution"`
	BitrateKbps int    `json:"bitrate_kbps"`
	FPS         int    `json:"fps"`
	IsActive    bool   `json:"is_active"`
	SortOrder   int    `json:"sort_order"`
}

type Permission struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Label     string `json:"label"`
	GroupName string `json:"group_name"`
}
