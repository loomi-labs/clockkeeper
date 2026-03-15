package clockkeeper

import "embed"

//go:embed all:web/build
var StaticFiles embed.FS
