package dns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func effectiveForwardZoneID(zone zoneRow, defaultForwardZoneID string) string {
	switch zone.ForwardPolicy {
	case "custom":
		return zone.ForwardZoneID
	case "default":
		return defaultForwardZoneID
	default:
		return ""
	}
}

func (h *handler) forward(
	ctx context.Context,
	req *dns.Msg,
	forwardZoneID string,
) (*dns.Msg, error) {
	ctx, span := otel.Tracer("dns").Start(ctx, "dns.forward")
	defer span.End()

	span.SetAttributes(attribute.String("forward_zone_id", forwardZoneID))

	fz, err := h.store.findForwardZone(
		ctx,
		forwardZoneID,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load forward zone")
		return nil, err
	}
	span.SetAttributes(
		attribute.String("forward_zone.name", fz.Name),
		attribute.Int("forward_zone.address_count", len(fz.Addresses)),
	)

	for i, addr := range fz.Addresses {
		if addr.Address == "" || addr.Port == 0 {
			span.AddEvent("skipped invalid forward address", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("address_obj", fmt.Sprintf("%+v", addr)),
			))
			continue
		}

		span.AddEvent("forward attempt", trace.WithAttributes(
			attribute.Int("index", i),
			attribute.String("protocol", addr.Protocol),
			attribute.String("address", addr.Address),
			attribute.Int("port", addr.Port),
		))

		target := net.JoinHostPort(addr.Address, strconv.Itoa(addr.Port))
		var resp *dns.Msg
		var exchangeErr error

		switch addr.Protocol {
		case "udp", "tcp", "tls":
			client := &dns.Client{
				Net:     addr.Protocol,
				Timeout: 3 * time.Second,
			}
			if addr.Protocol == "tls" && addr.TLSServerName != "" {
				client.TLSConfig.ServerName = addr.TLSServerName
			}
			resp, _, exchangeErr = client.ExchangeContext(
				ctx,
				req.Copy(),
				target,
			)

			// Retry truncated UDP over TCP
			if exchangeErr == nil && resp != nil && resp.Truncated && addr.Protocol == "udp" {
				span.AddEvent("retrying truncated response over tcp", trace.WithAttributes(
					attribute.Int("index", i),
					attribute.String("target", target),
				))
				tcpClient := &dns.Client{
					Net:     "tcp",
					Timeout: 3 * time.Second,
				}
				tcpResp, _, tcpErr := tcpClient.ExchangeContext(
					ctx,
					req.Copy(),
					target,
				)
				if tcpErr == nil && tcpResp != nil {
					resp = tcpResp
					exchangeErr = nil
				}
			}

		case "https": // DNS over HTTPS
			httpClient := &http.Client{Timeout: 3 * time.Second}
			queryURL := url.URL{
				Scheme: "https",
				Host:   target,
				Path:   addr.DOHPath,
			}
			if queryURL.Path == "" {
				queryURL.Path = "/dns-query"
			}

			msgBytes, mErr := req.Pack()
			if mErr != nil {
				exchangeErr = mErr
				break
			}

			httpReq, hErr := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				queryURL.String(),
				bytes.NewReader(msgBytes),
			)
			if hErr != nil {
				exchangeErr = hErr
				break
			}
			httpReq.Header.Set("Content-Type", "application/dns-message")
			httpReq.Header.Set("Accept", "application/dns-message")

			httpResp, hErr := httpClient.Do(httpReq)
			if hErr != nil {
				exchangeErr = hErr
				break
			}
			defer httpResp.Body.Close()

			if httpResp.StatusCode != http.StatusOK {
				exchangeErr = fmt.Errorf("DoH query failed with status: %s", httpResp.Status)
				break
			}

			respBytes, rErr := io.ReadAll(httpResp.Body)
			if rErr != nil {
				exchangeErr = rErr
				break
			}

			dohResp := new(dns.Msg)
			if uErr := dohResp.Unpack(respBytes); uErr != nil {
				exchangeErr = uErr
				break
			}
			resp = dohResp

		default:
			span.AddEvent("unsupported protocol", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("protocol", addr.Protocol),
			))
			continue
		}

		if exchangeErr != nil {
			span.AddEvent("forward attempt failed", trace.WithAttributes(
				attribute.Int("index", i),
				attribute.String("error", exchangeErr.Error()),
			))
			continue
		}

		if resp == nil {
			span.AddEvent("forward returned empty response", trace.WithAttributes(
				attribute.Int("index", i),
			))
			continue
		}

		span.AddEvent("forward succeeded", trace.WithAttributes(
			attribute.Int("index", i),
		))
		span.SetStatus(codes.Ok, "forwarded")
		return resp, nil
	}

	span.SetStatus(codes.Error, "all forwarders failed")
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Rcode = dns.RcodeServerFailure

	return msg, nil
}
