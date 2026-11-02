package webembed

import "embed"

//go:embed  web/index.html web/script.js
var EmbeddedFiles embed.FS
