package dns

import (
	"testing"

	"github.com/fmotalleb/hermes/models"
	"github.com/miekg/dns"
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
