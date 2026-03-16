package portal

import "embed"

//go:embed all:ui/dist
var EmbeddedFS embed.FS
