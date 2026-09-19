package web

import "embed"

// all: keeps SvelteKit's _app directory, which a plain directory pattern would skip.
//
//go:embed all:static
var Files embed.FS
