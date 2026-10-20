package model

// DeployRequest adalah body JSON untuk POST /api/deploy (source = github).
// Untuk source zip gunakan multipart/form-data dengan field yang sama + file "file".
type DeployRequest struct {
	Name       string            `json:"name"`
	Slug       string            `json:"slug"`
	SourceType string            `json:"source_type"` // github | zip (default: github)
	RepoFull   string            `json:"repo_full_name"`
	Branch     string            `json:"branch"`
	Port       int               `json:"port"`
	Buildpack  string            `json:"buildpack"`
	AutoDeploy *bool             `json:"auto_deploy"`
	Env        map[string]string `json:"env"`
	Database   string            `json:"database"` // postgres | mysql | mongodb | redis (opsional)
}

// SetEnvRequest untuk PUT /api/apps/{id}/env.
type SetEnvRequest struct {
	Env map[string]string `json:"env"`
}
