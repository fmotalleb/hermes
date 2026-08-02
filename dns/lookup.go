package dns

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/fmotalleb/hermes/models"
)

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	ctx, span := h.tracer.Start(context.Background(), "dns.serve")
	defer func() {
		err := recover()
		if err != nil {
			span.RecordError(fmt.Errorf("dns panic: %s", err))
			span.SetStatus(codes.Error, "dns panic recovered")
			h.log().Error("fatal error recovered", zap.Any("error", err))
		}
	}()

	defer span.End(trace.WithStackTrace(true))
	if len(r.Question) == 0 {
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(msg)
		span.SetStatus(codes.Error, "no question in request")
		return
	}

	q := r.Question[0]
	span.AddEvent("lookup", trace.WithAttributes(
		attribute.String("name", q.Name),
		attribute.Int("class", int(q.Qclass)),
		attribute.Int("type", int(q.Qtype)),
	))

	if resp, ok := h.cachedResponse(ctx, r); ok {
		span.AddEvent("cache hit", trace.WithAttributes(
			attribute.String("name", q.Name),
			attribute.Int("class", int(q.Qclass)),
			attribute.Int("type", int(q.Qtype)),
		))
		writeAnswer(w, resp, span)
		return
	}
	span.AddEvent("cache miss")
	resp, err := h.lookup(ctx, q.Name, q.Qtype, r)
	if err != nil {
		span.AddEvent("failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		span.SetStatus(codes.Error, "failed to lookup the domain")
		h.log().Error("dns lookup failed", zap.Error(err))
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		writeAnswer(w, msg, span)
		return
	}
	h.cacheResponse(ctx, r, resp)
	writeAnswer(w, resp, span)
}

func writeAnswer(w dns.ResponseWriter, resp *dns.Msg, span trace.Span) {
	if err := w.WriteMsg(resp); err != nil {
		span.SetStatus(codes.Error, "failed to send answer")
		span.RecordError(err)
	} else {
		span.AddEvent("returned answer")
		span.SetStatus(codes.Ok, "returned answer")
	}
}

func (h *handler) lookup(ctx context.Context, qname string, qtype uint16, req *dns.Msg) (*dns.Msg, error) {
	ctx, span := otel.Tracer("dns").Start(ctx, "dns.lookup")
	defer span.End()

	span.SetAttributes(
		attribute.String("query.name", qname),
		attribute.Int("query.type", int(qtype)),
	)

	// TODO Work in progress left open, need to handle proxy mode, and forward mode separately
	result, ok := h.hijack(ctx, qname, qtype, req)
	if ok {
		span.SetAttributes(attribute.Bool("query.hijacked", true))
		span.AddEvent("query hijacked")
		return result, nil
	}

	qname = normalizeDNSName(qname)
	span.SetAttributes(attribute.String("query.normalized_name", qname))

	zone, err := h.findZone(ctx, qname)
	if err != nil {
		if isNoRows(err) {
			span.AddEvent("zone not found")

			settings, settingsErr := h.loadSettings(ctx)
			if settingsErr != nil {
				span.RecordError(settingsErr)
				span.SetStatus(codes.Error, "failed to load settings")
				return nil, settingsErr
			}

			if settings.DefaultForwardZoneID != "" {
				span.AddEvent("forwarding using default forward zone", trace.WithAttributes(
					attribute.String("forward_zone_id", settings.DefaultForwardZoneID),
				))

				return h.forward(ctx, req, settings.DefaultForwardZoneID)
			}

			span.SetStatus(codes.Ok, "nxdomain")
			return nxdomain(req), nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to find zone")
		return nil, err
	}
	span.SetAttributes(
		attribute.String("zone.id", zone.ID),
		attribute.String("zone.name", zone.Name),
		attribute.String("zone.forward_policy", zone.ForwardPolicy),
		attribute.String("zone.forward_zone_id", zone.ForwardZoneID),
	)

	records, err := h.recordsForZone(ctx, zone.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load records")
		return nil, err
	}
	span.SetAttributes(attribute.Int("zone.record_count", len(records)))

	answers := buildAnswers(zone.Name, qname, qtype, records)
	if len(answers) > 0 {
		span.AddEvent("answer selected", trace.WithAttributes(
			attribute.Int("answer_count", len(answers)),
		))
		span.SetStatus(codes.Ok, "answered from zone")
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.Answer = answers
		msg.RecursionAvailable = false
		return msg, nil
	}

	defaultForwardZoneID := ""
	if zone.ForwardPolicy == models.ForwardPolicyDefault {
		settings, err := h.loadSettings(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to load settings")
			return nil, err
		}
		defaultForwardZoneID = settings.DefaultForwardZoneID
	}

	forwardZoneID := effectiveForwardZoneID(zone, defaultForwardZoneID)

	if forwardZoneID != "" {
		span.AddEvent("forwarding query", trace.WithAttributes(
			attribute.String("forward_zone_id", forwardZoneID),
		))
		return h.forward(ctx, req, forwardZoneID)
	}

	span.AddEvent("no matching records")
	span.SetStatus(codes.Ok, "no answer")
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = true
	msg.RecursionAvailable = false
	return msg, nil
}
