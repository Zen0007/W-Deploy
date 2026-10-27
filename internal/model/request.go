package model

// DeployRequest adalah body JSON untuk POST /api/deploy.
// JSON source "local" menerima path server; upload folder memakai multipart/form-data.
type DeployRequest struct {
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Source      string            `json:"source"`
	SourceType  string            `json:"source_type"`
	RepoFull    string            `json:"repo_full_name"`
	Path        string            `json:"path"`
	Branch      string            `json:"branch"`
	Port        int               `json:"port"`
	Buildpack   string            `json:"buildpack"`
	AutoDeploy  *bool             `json:"auto_deploy"`
	Env         map[string]string `json:"env"`
	Database    string            `json:"database"`

	// ProjectType adalah fallback hint dari FE.
	// Nilai valid: static | spa | node | custom. Kosong = auto-detect.
	ProjectType string `json:"project_type,omitempty"`
}

// SetEnvRequest untuk PUT /api/apps/{id}/env.
type SetEnvRequest struct {
	Env map[string]string `json:"env"`
}
