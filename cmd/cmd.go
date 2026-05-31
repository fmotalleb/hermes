package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// This doesn't work due to how gofr manages its pubsub manager
	app := gofr.NewCMD()
	app.SubCommand("dns", func(c *gofr.Context) (any, error) {
		return nil, runDNS(c, app)
	})
	app.Run()
}
