package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the version string set by ldflags.
	Version = "dev"

	// Commit is the git commit hash set by ldflags.
	Commit = "unknown"

	// Date is the build date set by ldflags.
	Date = "unknown"
)

// String returns the formatted version string.
func String() string {
	return fmt.Sprintf("yutemal %s (%s) built on %s", Version, Commit, Date)
}

// Info returns detailed version information.
func Info() string {
	return fmt.Sprintf(`yutemal - YouTube Music AT Terminal
Version: %s
Commit: %s
Built: %s
Go: %s
OS/Arch: %s/%s`, Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
