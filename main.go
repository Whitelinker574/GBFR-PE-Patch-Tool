package main

import (
	"embed"

	"gbfrPlayerInfoEdit/internal/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	backend.Run(assets)
}
