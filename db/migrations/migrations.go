// Package migrations embeds the versioned SQL migrations into the binaries,
// so a migration run always uses exactly the files the image was built with.
//
// Add a migration with `make migration-new NAME=create_something`. Files are
// named NNNNN_description.sql and contain `-- +goose Up` and `-- +goose Down`
// sections. Never edit a migration that has reached stage or production;
// add a new one instead.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
