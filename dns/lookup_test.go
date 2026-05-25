package dns

import (
	"testing"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/models"
)

func TestSelectBestRecordSetPrefersExactMatchOverGlob(t *testing.T) {
	records := []recordRow{
		{Name: "www", Type: models.A, Value: "192.0.2.10", TTL: 300},
		{Name: "*.example.com", Type: models.A, Value: "192.0.2.20", TTL: 300},
	}

	got := selectBestRecordSet("example.com", "www.example.com", dns.TypeA, records)
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].Name != "www" {
		t.Fatalf("expected exact record, got %q", got[0].Name)
	}
}

func TestSelectBestRecordSetPrefersMoreSpecificGlob(t *testing.T) {
	records := []recordRow{
		{Name: "*.example.com", Type: models.A, Value: "192.0.2.10", TTL: 300},
		{Name: "*.bar", Type: models.A, Value: "192.0.2.20", TTL: 300},
	}

	got := selectBestRecordSet("example.com", "foo.bar.example.com", dns.TypeA, records)
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].Name != "*.bar" {
		t.Fatalf("expected more specific glob, got %q", got[0].Name)
	}
}

func TestSelectBestRecordSetPrefersCNAMEWhenPresent(t *testing.T) {
	records := []recordRow{
		{Name: "www", Type: models.A, Value: "192.0.2.10", TTL: 300},
		{Name: "www", Type: models.CNAME, Value: "alias.example.net", TTL: 300},
	}

	got := buildAnswers("example.com", "www.example.com", dns.TypeA, records)
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if _, ok := got[0].(*dns.CNAME); !ok {
		t.Fatalf("expected CNAME record, got %T", got[0])
	}
}

func TestRecordToRRUsesSRVPriorityField(t *testing.T) {
	rr, ok := recordToRR(
		"example.com",
		"_sip._tcp.example.com",
		recordRow{
			Name:     "_sip._tcp",
			Type:     models.SRV,
			Value:    "10 5060 sip.example.com",
			TTL:      300,
			Priority: 42,
		},
	)
	if !ok {
		t.Fatal("expected SRV record to convert")
	}

	srv, ok := rr.(*dns.SRV)
	if !ok {
		t.Fatalf("expected *dns.SRV, got %T", rr)
	}

	if srv.Priority != 42 {
		t.Fatalf("expected priority 42, got %d", srv.Priority)
	}
	if srv.Weight != 10 {
		t.Fatalf("expected weight 10, got %d", srv.Weight)
	}
	if srv.Port != 5060 {
		t.Fatalf("expected port 5060, got %d", srv.Port)
	}
	if got := srv.Target; got != "sip.example.com." {
		t.Fatalf("expected target sip.example.com., got %q", got)
	}
}
