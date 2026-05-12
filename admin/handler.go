package admin

import (
	"bytes"
	"context"
	"net/url"

	"github.com/a-h/templ"
	view "github.com/fmotalleb/helios/templates"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

type handler struct {
	repo *repository
}

type createZoneRequest struct {
	Name string `form:"name"`
}

type createRecordRequest struct {
	Name     string `form:"name"`
	Type     string `form:"type"`
	Value    string `form:"value"`
	TTL      int    `form:"ttl"`
	Priority int    `form:"priority"`
}

func newHandler(repo *repository) *handler {
	return &handler{repo: repo}
}

func (h *handler) dashboard(ctx *gofr.Context) (any, error) {
	viewZones, err := h.repo.listZones(ctx)
	if err != nil {
		return nil, err
	}

	return renderComponent(view.AdminPage(view.AdminPageData{
		Error: ctx.Request.Param("error"),
		Zones: viewZones,
	}))
}

func (h *handler) createZone(ctx *gofr.Context) (any, error) {
	var req createZoneRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+zone+payload"}, nil
	}

	if err := h.repo.createZone(ctx, req.Name); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}

	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) deleteZone(ctx *gofr.Context) (any, error) {
	if err := h.repo.deleteZone(ctx, ctx.Request.PathParam("zone")); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}

	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) createRecord(ctx *gofr.Context) (any, error) {
	var req createRecordRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+record+payload"}, nil
	}

	err := h.repo.createRecord(ctx, ctx.Request.PathParam("zone"), view.DNSRecord{
		Name:     req.Name,
		Type:     req.Type,
		Value:    req.Value,
		TTL:      req.TTL,
		Priority: req.Priority,
	})
	if err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}

	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) deleteRecord(ctx *gofr.Context) (any, error) {
	err := h.repo.deleteRecord(ctx, ctx.Request.PathParam("zone"), ctx.Request.PathParam("id"))
	if err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}

	return response.Redirect{URL: "/admin"}, nil
}

func renderComponent(component templ.Component) (any, error) {
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		return nil, err
	}

	return response.File{
		ContentType: "text/html; charset=utf-8",
		Content:     buf.Bytes(),
	}, nil
}
