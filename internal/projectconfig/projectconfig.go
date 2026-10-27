package projectconfig

// ProjectType adalah kategori proyek yang didukung MVP.
type ProjectType string

const (
	TypeStatic ProjectType = "static" // HTML/CSS/JS tanpa build step
	TypeSPA    ProjectType = "spa"    // Vite/React/Vue dengan npm run build
	TypeNode   ProjectType = "node"   // Backend Node.js (Express dsb)
	TypeCustom ProjectType = "custom" // Didefinisikan manual via wdeploy.json
)

func (t ProjectType) Valid() bool {
	switch t {
	case TypeStatic, TypeSPA, TypeNode, TypeCustom:
		return true
	default:
		return false
	}
}

// IsValidProjectType menggantikan InvalidProjectType agar penamaan lebih intuitif.
func IsValidProjectType(s string) bool {
	return ProjectType(s).Valid()
}

// ConfigSource mendefinisikan sumber konfigurasi yang digunakan.
type ConfigSource string

const (
	SourceWDeployJSON ConfigSource = "wdeploy.json" // file di repo
	SourceFE          ConfigSource = "fe"           // project_type di request
	SourceAuto        ConfigSource = "auto"         // deteksi filesystem
	SourceDefault     ConfigSource = "default"      // fallback (node)
)

// BuildConfig menyimpan konfigurasi proses build.
type BuildConfig struct {
	Command string `json:"command,omitempty"`
	Output  string `json:"output,omitempty"`
}

// RunConfig menyimpan konfigurasi port atau jalannya server.
type RunConfig struct {
	Port int `json:"port,omitempty"`
}

// ResolveConfig hasil resolusi akhir tipe proyek dan sumbernya.
type ResolveConfig struct {
	ProjectType  ProjectType  `json:"project_type"`
	ConfigSource ConfigSource `json:"config_source"`
	Build        *BuildConfig `json:"build,omitempty"`
	Run          *RunConfig   `json:"run,omitempty"`
}
