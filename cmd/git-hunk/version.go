package main

import (
	"fmt"
	"runtime/debug"
)

// Version is the build version, stamped by the Makefile from "git
// describe". It is deliberately left uninitialized: the linker's -X flag
// only writes a variable that is uninitialized or set to a constant, and a
// hardcoded default drifts from the tags it claims to name.
var Version string

// VersionCmd prints the version number of git-hunk.
type VersionCmd struct{}

func (c *VersionCmd) Run(g *Globals) error {
	fmt.Fprintf(g.Out, "git-hunk %s\n", version())

	return nil
}

// version reports the stamped version, falling back to the module version
// the go tool records in the binary. The fallback covers builds that never
// run the Makefile: "go install <pkg>@<version>" reports that version, and
// a plain local build reports "(devel)".
func version() string {
	if Version != "" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}

	return info.Main.Version
}
