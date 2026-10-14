package main

import "embed"

// embeddedAssets bundles the project's static assets (sprite sheets, fonts, the music track, default settings JSON) into the binary so a wasm build has everything it needs without separate asset hosting.
//
//go:embed all:resources/images all:resources/fonts all:resources/audio settings/default.json
var embeddedAssets embed.FS
