package main

import (
	"github.com/fmotalleb/helios/admin"
	"github.com/fmotalleb/helios/migrations"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	admin.Register(app)
	app.Run()
}
