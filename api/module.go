package api

import "gofr.dev/pkg/gofr"

const apiBasePath = "/api/"

func apiRoute(name string) string {
	return apiBasePath + name
}

func Register(app *gofr.App) {
	handler := newHandler(newRepository())
	app.GET(apiRoute("zones"), handler.getZones)
	// app.POST("/api/zones", handler.createZone)
	// app.POST("/api/zones/{zone}/delete", handler.deleteZone)
	// app.POST("/api/zones/{zone}/records", handler.createRecord)
	// app.POST("/api/zones/{zone}/records/{id}/delete", handler.deleteRecord)
	// app.POST("/api/zones/{zone}/config", handler.updateZoneConfig)
	// app.POST("/api/inbound", handler.createInboundEntrypoint)
	// app.POST("/api/inbound/{id}", handler.updateInboundEntrypoint)
	// app.POST("/api/inbound/{id}/delete", handler.deleteInboundEntrypoint)
	// app.POST("/api/forward-zones", handler.createForwardZone)
	// app.POST("/api/forward-zones/fallback", handler.updateFallbackForwardZone)
	// app.POST("/api/forward-zones/{id}", handler.updateForwardZone)
	// app.POST("/api/forward-zones/{id}/delete", handler.deleteForwardZone)
}
