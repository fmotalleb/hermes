package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/auth"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	auth.Register(app)
	api.Register(app)
	app.GET("/", serveStatic)
	app.GET("/{path:.*}", serveStatic)
	app.OnStart(func(ctx *gofr.Context) error {
		go dns.Serve(ctx)
		return nil
	})
	app.Run()
}
