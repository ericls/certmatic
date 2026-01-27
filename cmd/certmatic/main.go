package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// Include standard Caddy modules
	_ "github.com/caddyserver/caddy/v2/modules/standard"

	// Include certmatic modules
	_ "github.com/ericls/certmatic/internal/caddy"
)

func main() {
	caddycmd.Main()
}
