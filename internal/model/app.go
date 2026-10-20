package model

import "W-dev/internal/repository"

// AppView adalah representasi app untuk API (menambahkan URL publik).
type AppView struct {
	repository.App
	URL string `json:"url"`
}

// DeployResponse dikembalikan setelah deploy diantrekan.
type DeployResponse struct {
	App          AppView  `json:"app"`
	DeploymentID uint     `json:"deployment_id"`
	WebSocket    string   `json:"websocket"`
	Warnings     []string `json:"warnings,omitempty"`
}

// UserView adalah profil ringkas yang dikirim ke frontend.
type UserView struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}
