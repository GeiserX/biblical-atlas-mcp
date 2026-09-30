// Package version holds the build metadata that release builds set with -ldflags.
package version

var (
	// Version is the release version without the leading "v", or "dev".
	Version = "dev"
	// Commit is the short commit the binary was built from.
	Commit = "none"
	// Date is the build date.
	Date = "unknown"
)

// String returns the version, commit and build date on one line.
func String() string {
	return Version + " (" + Commit + ") " + Date
}
