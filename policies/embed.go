// Package policies contains the generic, reproducible policy authoring template.
package policies

import "embed"

//go:embed baseline/*.json baseline/*.rego baseline/tests/*.yaml
var Baseline embed.FS
