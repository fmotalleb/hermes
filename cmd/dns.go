package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/dns"
)

func runDNS(ctx *gofr.Context, app *gofr.App) error {
	return dns.Serve(ctx, app, dns.WithProtocol(dns.ProtocolUDP))
}
