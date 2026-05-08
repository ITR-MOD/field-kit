package gui

import "embed"

// FrontendFS holds the embedded frontend web assets.
// It is exported so cmd/gui.go can pass it to Wails after sub-pathing.
//
//go:embed all:frontend
var FrontendFS embed.FS
