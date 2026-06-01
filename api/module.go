package api

import (
	"context"
	"net/http"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/web"
	"github.com/fmotalleb/hermes/queries"
)

const apiBasePath = "/api/"

func apiRoute(name string) string {
	return apiBasePath + name
}

type Migrator interface {
	Run(context.Context) error
}

func Register(router *web.Router, db queries.DB, c cache.Cache, bus *pubsub.Bus, migrator Migrator, metricsHandler http.Handler) {
	handler := newHandler(newRepository(db, c, bus), migrator, metricsHandler, bus)

	router.GET(apiRoute("zones"), handler.getZones)
	router.GET(apiRoute("zones/{zone}"), handler.getZone)
	router.POST(apiRoute("zones"), handler.createZone)
	router.POST(apiRoute("zones/{zone}"), handler.updateZone)
	router.DELETE(apiRoute("zones/{zone}"), handler.deleteZone)
	router.GET(apiRoute("zones/{zone}/records"), handler.getZoneRecords)
	router.GET(apiRoute("zones/{zone}/records/{id}"), handler.getRecord)
	router.POST(apiRoute("zones/{zone}/records"), handler.createRecord)
	router.POST(apiRoute("zones/{zone}/records/{id}"), handler.updateRecord)
	router.DELETE(apiRoute("zones/{zone}/records/{id}"), handler.deleteRecord)
	router.GET(apiRoute("forward-zones"), handler.getForwardZones)
	router.GET(apiRoute("forward-zones/{id}"), handler.getForwardZone)
	router.POST(apiRoute("forward-zones"), handler.createForwardZone)
	router.POST(apiRoute("forward-zones/{id}"), handler.updateForwardZone)
	router.DELETE(apiRoute("forward-zones/{id}"), handler.deleteForwardZone)
	router.GET(apiRoute("settings"), handler.getSettings)
	router.POST(apiRoute("settings"), handler.updateSettings)
	router.POST(apiRoute("pubsub/{topic}"), handler.publishEvent)
	router.POST(apiRoute("migrations/run"), handler.runMigrations)
	router.GET(apiRoute("metrics"), handler.metrics)
}
