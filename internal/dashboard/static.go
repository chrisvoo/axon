package dashboard

import "embed"

// FS holds the built React app (copied from web/dist by `make web`).
//
//go:embed all:dist
var FS embed.FS
