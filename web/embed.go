package web

import "embed"

// Assets contains the compiled React Single-Page Application assets.
//
//go:embed all:dist
var Assets embed.FS
