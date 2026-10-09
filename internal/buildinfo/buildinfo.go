// Package buildinfo carries the version stamped into the binary at build time.
//
// The Dockerfile and Makefile set these with:
//
//	-ldflags "-X <module>/internal/buildinfo.Version=... -X <module>/internal/buildinfo.Commit=..."
//
// A binary built with plain `go build` reports "dev".
package buildinfo

var (
	Version = "dev"
	Commit  = "unknown"
)
