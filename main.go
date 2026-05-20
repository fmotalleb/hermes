package main

import (
	"context"

	"gofr.dev/pkg/gofr"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/api"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/migrations"
)

func main() {
	app := gofr.New()
	app.Migrate(migrations.All())
	api.Register(app)
	ctx := context.TODO()
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return dns.Serve(ctx, app.Config, app.Logger(), app.Metrics())
	})
	errGroup.Go(func() error {
		app.Run()
		return nil
	})
	errGroup.Wait()
}
