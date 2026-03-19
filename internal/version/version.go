package version

import (
	"runtime/debug"
	"strings"
)

// Build-time parameters set via -ldflags

var Version = "devel"

// cleanVersion removes the +dirty suffix and other unwanted parts from version strings.
func cleanVersion(v string) string {
	// Remove +dirty suffix
	v = strings.TrimSuffix(v, "+dirty")
	// Remove any other + suffixes (like +build metadata)
	if idx := strings.Index(v, "+"); idx != -1 {
		v = v[:idx]
	}
	return v
}

// A user may install swarmy using `go install github.com/cloudwithax/swarmy@latest`.
// without -ldflags, in which case the version above is unset. As a workaround
// we use the embedded build version that *is* set when using `go install` (and
// is only set for `go install` and not for `go build`).
func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	mainVersion := info.Main.Version
	if mainVersion != "" && mainVersion != "(devel)" {
		Version = cleanVersion(mainVersion)
	}
}
