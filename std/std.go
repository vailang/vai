// Package std provides the Vai standard library of embedded prompts.
package std

import (
	"embed"
)

//go:embed files/*.vai
var Standard embed.FS
