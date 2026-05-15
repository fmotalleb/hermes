package admin

import "gofr.dev/pkg/gofr"

func Register(app *gofr.App) {
	handler := newHandler(newRepository())

	app.AddStaticFiles("/static", "./static")
	app.GET("/admin", handler.dashboard)
	app.POST("/admin/zones", handler.createZone)
	app.POST("/admin/zones/{zone}/delete", handler.deleteZone)
	app.POST("/admin/zones/{zone}/records", handler.createRecord)
	app.POST("/admin/zones/{zone}/records/{id}/delete", handler.deleteRecord)
	app.POST("/admin/zones/{zone}/config", handler.updateZoneConfig)
	app.POST("/admin/inbound", handler.createInboundEntrypoint)
	app.POST("/admin/inbound/{id}", handler.updateInboundEntrypoint)
	app.POST("/admin/inbound/{id}/delete", handler.deleteInboundEntrypoint)
	app.POST("/admin/forward-zones", handler.createForwardZone)
	app.POST("/admin/forward-zones/fallback", handler.updateFallbackForwardZone)
	app.POST("/admin/forward-zones/{id}", handler.updateForwardZone)
	app.POST("/admin/forward-zones/{id}/delete", handler.deleteForwardZone)
}
