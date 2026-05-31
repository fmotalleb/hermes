package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/auth"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	auth.Register(app)
	api.Register(app)
	app.GET("/", serveStatic)
	app.GET("/{path:.*}", serveStatic)
	app.Run()
}
