package main

import "fmt"

// Version is the current version of git-hunk.
const Version = "v1.0.2"

// VersionCmd prints the version number of git-hunk.
type VersionCmd struct{}

func (c *VersionCmd) Run(g *Globals) error {
	fmt.Fprintf(g.Out, "git-hunk %s\n", Version)

	return nil
}
