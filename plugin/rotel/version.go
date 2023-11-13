// Copyright JAMF Software, LLC

package rotel

import (
	"runtime/debug"
)

// version is the current release version of the rotel instrumentation.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, dep := range info.Deps {
			if dep.Path == instrumentationName {
				return dep.Version
			}
		}
	}
	return "unknown"
}

// semVersion is the semantic version to be supplied to tracer creation.
func semVersion() string {
	return "semver:" + version()
}
