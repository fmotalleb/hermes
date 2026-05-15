package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/admin"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	admin.Register(app)
	app.Run()
}
