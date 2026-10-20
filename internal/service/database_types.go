package service

import "errors"

// DBEngine mewakili tipe database yang didukung.
type DBEngine string

const (
	DBPostgres DBEngine = "postgres"
	DBMySQL    DBEngine = "mysql"
	DBMongo    DBEngine = "mongodb"
	DBRedis    DBEngine = "redis"
	DBSQLite   DBEngine = "sqlite" // volume-based, tanpa kontainer server
)

// DBProvisionRequest adalah input dari GUI/CLI untuk membuat database.
type DBProvisionRequest struct {
	AppID    uint     `json:"app_id"`
	AppSlug  string   `json:"app_slug"`
	Engine   DBEngine `json:"engine"`
	Version  string   `json:"version,omitempty"`
	DBName   string   `json:"db_name,omitempty"`
	Username string   `json:"username,omitempty"`
}

// DBProvisionResult dikembalikan setelah container berhasil dibuat.
type DBProvisionResult struct {
	ContainerID   string   `json:"container_id"`
	ContainerName string   `json:"container_name"`
	Engine        DBEngine `json:"engine"`
	Host          string   `json:"host"`
	Port          int      `json:"port"`
	Database      string   `json:"database"`
	Username      string   `json:"username"`
	Password      string   `json:"password"` // hanya dikembalikan sekali
	ConnectionURI string   `json:"connection_uri"`
	VolumeName    string   `json:"volume_name"`
}

// Sentinel error untuk caller (bisa dicek via errors.Is).
var (
	ErrEngineNotSupported = errors.New("db engine tidak didukung")
	ErrInvalidSlug        = errors.New("app_slug tidak valid")
	ErrAlreadyProvisioned = errors.New("database sudah pernah di-provision")
)
