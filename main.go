package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	app.AddStaticFiles("/", "./static")
	app.GET("/", func(c *gofr.Context) (any, error) {
		return response.Redirect{
			URL: "/admin.html",
		}, nil
	})
	api.Register(app)
	app.OnStart(func(ctx *gofr.Context) error {
		go dns.Serve(ctx)
		return nil
	})
	app.Run()
}
