package api

import "gofr.dev/pkg/gofr"

const apiBasePath = "/api/"

func apiRoute(name string) string {
	return apiBasePath + name
}

func Register(app *gofr.App) {
	handler := newHandler(newRepository())
	app.GET(apiRoute("zones"), handler.getZones)
	app.GET(apiRoute("zones/{zone}"), handler.getZone)
	app.POST(apiRoute("zones"), handler.createZone)
	app.POST(apiRoute("zones/{zone}"), handler.updateZone)
	app.DELETE(apiRoute("zones/{zone}"), handler.deleteZone)
	app.GET(apiRoute("zones/{zone}/records"), handler.getZoneRecords)
	app.GET(apiRoute("zones/{zone}/records/{id}"), handler.getRecord)
	app.POST(apiRoute("zones/{zone}/records"), handler.createRecord)
	app.POST(apiRoute("zones/{zone}/records/{id}"), handler.updateRecord)
	app.DELETE(apiRoute("zones/{zone}/records/{id}"), handler.deleteRecord)
	app.GET(apiRoute("forward-zones"), handler.getForwardZones)
	app.GET(apiRoute("forward-zones/{id}"), handler.getForwardZone)
	app.POST(apiRoute("forward-zones"), handler.createForwardZone)
	app.POST(apiRoute("forward-zones/{id}"), handler.updateForwardZone)
	app.DELETE(apiRoute("forward-zones/{id}"), handler.deleteForwardZone)
}
