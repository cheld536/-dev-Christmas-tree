package buildinfo

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

var override string

func Version() string {
	if override != "" {
		return override
	}
	return strings.TrimSpace(version)
}
