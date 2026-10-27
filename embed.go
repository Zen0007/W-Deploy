package webembed

import "embed"

//go:embed  web/index.html
var EmbeddedFiles embed.FS
