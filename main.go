package main

import (
	"github.com/fmotalleb/hermes/admin"
	"github.com/fmotalleb/hermes/migrations"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	admin.Register(app)
	app.Run()
}
