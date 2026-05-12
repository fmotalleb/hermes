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
}
