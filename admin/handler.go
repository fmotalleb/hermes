package admin

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	view "github.com/fmotalleb/hermes/templates"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

type handler struct {
	repo *repository
}

type createZoneRequest struct {
	Name             string `form:"name"`
	ForwardSelection string `form:"forward_selection"`
	CacheTTL         int    `form:"cache_ttl"`
}

type createRecordRequest struct {
	Name     string `form:"name"`
	Type     string `form:"type"`
	Value    string `form:"value"`
	TTL      int    `form:"ttl"`
	Priority int    `form:"priority"`
}

type updateZoneConfigRequest struct {
	ForwardSelection string `form:"forward_selection"`
	CacheTTL         int    `form:"cache_ttl"`
}

type createForwardZoneRequest struct {
	Name         string `form:"name"`
	AddressesCSV string `form:"addresses_csv"`
}

type updateFallbackRequest struct {
	FallbackForwardZoneID string `form:"fallback_forward_zone_id"`
}

type inboundEntrypointRequest struct {
	Type          string `form:"type"`
	ListenAddress string `form:"listen_address"`
	Port          int    `form:"port"`
	PublicKey     string `form:"public_key"`
	PrivateKey    string `form:"private_key"`
}

func newHandler(repo *repository) *handler {
	return &handler{repo: repo}
}

func (h *handler) dashboard(ctx *gofr.Context) (any, error) {
	viewZones, err := h.repo.listZones(ctx)
	if err != nil {
		return nil, err
	}
	entrypoints, err := h.repo.listInboundEntrypoints(ctx)
	if err != nil {
		return nil, err
	}
	forwardZones, err := h.repo.listForwardZones(ctx)
	if err != nil {
		return nil, err
	}
	fallbackID, err := h.repo.getFallbackForwardZoneID(ctx)
	if err != nil {
		return nil, err
	}

	return renderComponent(view.AdminPage(view.AdminPageData{
		Error:                 ctx.Request.Param("error"),
		Zones:                 viewZones,
		Entrypoints:           entrypoints,
		ForwardZones:          forwardZones,
		FallbackForwardZoneID: fallbackID,
	}))
}

func (h *handler) createZone(ctx *gofr.Context) (any, error) {
	var req createZoneRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+zone+payload"}, nil
	}

	mode, forwardID := parseForwardSelection(req.ForwardSelection)
	if err := h.repo.createZone(ctx, req.Name, mode, forwardID, req.CacheTTL); err != nil {
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

func (h *handler) updateZoneConfig(ctx *gofr.Context) (any, error) {
	var req updateZoneConfigRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+zone+config+payload"}, nil
	}
	mode, forwardID := parseForwardSelection(req.ForwardSelection)
	if err := h.repo.updateZoneConfig(ctx, ctx.Request.PathParam("zone"), mode, forwardID, req.CacheTTL); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) createInboundEntrypoint(ctx *gofr.Context) (any, error) {
	var req inboundEntrypointRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+entrypoint+payload"}, nil
	}
	if err := h.repo.createInboundEntrypoint(ctx, view.InboundEntrypoint{
		Type:          req.Type,
		ListenAddress: req.ListenAddress,
		Port:          req.Port,
		PublicKey:     req.PublicKey,
		PrivateKey:    req.PrivateKey,
	}); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) updateInboundEntrypoint(ctx *gofr.Context) (any, error) {
	var req inboundEntrypointRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+entrypoint+payload"}, nil
	}
	if err := h.repo.updateInboundEntrypoint(ctx, ctx.Request.PathParam("id"), view.InboundEntrypoint{
		Type:          req.Type,
		ListenAddress: req.ListenAddress,
		Port:          req.Port,
		PublicKey:     req.PublicKey,
		PrivateKey:    req.PrivateKey,
	}); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) deleteInboundEntrypoint(ctx *gofr.Context) (any, error) {
	if err := h.repo.deleteInboundEntrypoint(ctx, ctx.Request.PathParam("id")); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) createForwardZone(ctx *gofr.Context) (any, error) {
	var req createForwardZoneRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+forward+zone+payload"}, nil
	}
	if err := h.repo.createForwardZone(ctx, req.Name, req.AddressesCSV); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) deleteForwardZone(ctx *gofr.Context) (any, error) {
	if err := h.repo.deleteForwardZone(ctx, ctx.Request.PathParam("id")); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) updateForwardZone(ctx *gofr.Context) (any, error) {
	var req createForwardZoneRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+forward+zone+payload"}, nil
	}
	if err := h.repo.updateForwardZone(ctx, ctx.Request.PathParam("id"), req.Name, req.AddressesCSV); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) updateFallbackForwardZone(ctx *gofr.Context) (any, error) {
	var req updateFallbackRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+fallback+payload"}, nil
	}
	if err := h.repo.updateFallbackForwardZone(ctx, req.FallbackForwardZoneID); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func parseForwardSelection(selection string) (mode string, forwardZoneID string) {
	selection = strings.TrimSpace(selection)
	switch {
	case selection == "", selection == "default":
		return "default", ""
	case selection == "none":
		return "none", ""
	case strings.HasPrefix(selection, "id:"):
		id := strings.TrimSpace(strings.TrimPrefix(selection, "id:"))
		if id == "" {
			return "default", ""
		}
		return "custom", id
	default:
		return "default", ""
	}
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
