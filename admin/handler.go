package admin

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	view "github.com/fmotalleb/helios/templates"
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
	Name    string `form:"name"`
	Address string `form:"address"`
}

type updateFallbackRequest struct {
	FallbackForwardZoneID string `form:"fallback_forward_zone_id"`
}

type inboundRequest struct {
	UDPListenAddress   string `form:"udp_listen_address"`
	UDPPort            int    `form:"udp_port"`
	TCPListenAddress   string `form:"tcp_listen_address"`
	TCPPort            int    `form:"tcp_port"`
	TLSListenAddress   string `form:"tls_listen_address"`
	TLSPort            int    `form:"tls_port"`
	TLSPublicKey       string `form:"tls_public_key"`
	TLSPrivateKey      string `form:"tls_private_key"`
	HTTPSListenAddress string `form:"https_listen_address"`
	HTTPSPort          int    `form:"https_port"`
	HTTPSPublicKey     string `form:"https_public_key"`
	HTTPSPrivateKey    string `form:"https_private_key"`
}

func newHandler(repo *repository) *handler {
	return &handler{repo: repo}
}

func (h *handler) dashboard(ctx *gofr.Context) (any, error) {
	viewZones, err := h.repo.listZones(ctx)
	if err != nil {
		return nil, err
	}
	inbound, err := h.repo.getInboundSettings(ctx)
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
		Inbound:               inbound,
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

func (h *handler) updateInbound(ctx *gofr.Context) (any, error) {
	var req inboundRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+inbound+payload"}, nil
	}
	if err := h.repo.updateInboundSettings(ctx, view.InboundSettings{
		UDPListenAddress:   req.UDPListenAddress,
		UDPPort:            req.UDPPort,
		TCPListenAddress:   req.TCPListenAddress,
		TCPPort:            req.TCPPort,
		TLSListenAddress:   req.TLSListenAddress,
		TLSPort:            req.TLSPort,
		TLSPublicKey:       req.TLSPublicKey,
		TLSPrivateKey:      req.TLSPrivateKey,
		HTTPSListenAddress: req.HTTPSListenAddress,
		HTTPSPort:          req.HTTPSPort,
		HTTPSPublicKey:     req.HTTPSPublicKey,
		HTTPSPrivateKey:    req.HTTPSPrivateKey,
	}); err != nil {
		return response.Redirect{URL: "/admin?error=" + url.QueryEscape(err.Error())}, nil
	}
	return response.Redirect{URL: "/admin"}, nil
}

func (h *handler) createForwardZone(ctx *gofr.Context) (any, error) {
	var req createForwardZoneRequest
	if err := ctx.Request.Bind(&req); err != nil {
		return response.Redirect{URL: "/admin?error=invalid+forward+zone+payload"}, nil
	}
	if err := h.repo.createForwardZone(ctx, req.Name, req.Address); err != nil {
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
