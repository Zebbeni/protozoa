package main

import "embed"

// embeddedAssets bundles the project's static assets (sprite sheets, fonts, default settings JSON) into the binary so a wasm build has everything it needs without separate asset hosting.
//
//go:embed all:resources/images all:resources/fonts settings/default.json
var embeddedAssets embed.FS
