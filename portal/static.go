package portal

//go:generate pnpm --dir ui run build

import "embed"

//go:embed all:ui/dist
var EmbeddedFS embed.FS
