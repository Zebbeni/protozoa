package main

import "embed"

// embeddedAssets bundles the project's static assets (sprite sheets,
// fonts, default settings JSON) into the binary so a wasm build has
// everything it needs without separate asset hosting. main.go hands
// this FS to the config and resources packages at startup; both fall
// back to file paths only if the FS is unset (currently never).
//
// Embedding the entire grid sprite tree adds ~1–2 MB to the binary —
// negligible for a desktop build, acceptable for a wasm download.
//
//go:embed all:resources/images all:resources/fonts settings/default.json
var embeddedAssets embed.FS
