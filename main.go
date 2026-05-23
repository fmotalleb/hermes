package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	api.Register(app)
	app.OnStart(func(ctx *gofr.Context) error {
		return dns.Serve(ctx)
	})
	app.Run()
}
