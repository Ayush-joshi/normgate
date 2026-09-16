// Package schemas embeds the versioned source contracts for offline validation.
package schemas

import "embed"

// Files contains all contract schemas. No network resolution is required.
//
//go:embed v1/*.schema.json
var Files embed.FS
