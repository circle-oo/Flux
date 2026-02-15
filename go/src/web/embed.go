package web

import "embed"

// DistFS embeds the built frontend assets.
// The dist/ directory is populated by the Makefile build step
// which copies typescript/dist/ here before compiling.
//
//go:embed all:dist
var DistFS embed.FS
