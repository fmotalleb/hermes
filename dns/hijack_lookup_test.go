package dns

import (
	"context"
	"testing"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/models"
)

type mockHijackStore struct {
	hijacks map[models.DNSRecordType][]hijackRow
}

func (m *mockHijackStore) findZone(ctx context.Context, qname string) (zoneRow, error) {
	return zoneRow{}, nil
}

func (m *mockHijackStore) loadSettings(ctx context.Context) (settingsRow, error) {
	return settingsRow{}, nil
}

func (m *mockHijackStore) findForwardZone(ctx context.Context, id string) (forwardZoneRow, error) {
	return forwardZoneRow{}, nil
}

func (m *mockHijackStore) recordsForZone(ctx context.Context, zoneID string) ([]record, error) {
	return nil, nil
}

func (m *mockHijackStore) lookupHijacks(_ context.Context, qtype models.DNSRecordType) ([]hijackRow, bool) {
	hrs, ok := m.hijacks[qtype]
	return hrs, ok
}

func TestHijackGlobMatching(t *testing.T) {
	mockStore := &mockHijackStore{
		hijacks: map[models.DNSRecordType][]hijackRow{
			models.A: {
				{Name: "*.example.com", Type: models.A, Value: "1.1.1.1", Policy: models.HijackPolicyRaw, TTL: 60},
				{Name: "foo.example.com", Type: models.A, Value: "2.2.2.2", Policy: models.HijackPolicyRaw, TTL: 60},
				{Name: "*.bar.example.com", Type: models.A, Value: "3.3.3.3", Policy: models.HijackPolicyRaw, TTL: 60},
			},
		},
	}

	h := &handler{
		dnsStore: mockStore,
	}

	tests := []struct {
		name        string
		qname       string
		qtype       uint16
		expectedVal string
		expectedOK  bool
	}{
		{
			name:        "exact match",
			qname:       "foo.example.com.",
			qtype:       dns.TypeA,
			expectedVal: "2.2.2.2",
			expectedOK:  true,
		},
		{
			name:        "glob match",
			qname:       "www.example.com.",
			qtype:       dns.TypeA,
			expectedVal: "1.1.1.1",
			expectedOK:  true,
		},
		{
			name:        "more specific glob match",
			qname:       "test.bar.example.com.",
			qtype:       dns.TypeA,
			expectedVal: "3.3.3.3",
			expectedOK:  true,
		},
		{
			name:        "no match",
			qname:       "nomatch.com.",
			qtype:       dns.TypeA,
			expectedVal: "",
			expectedOK:  false,
		},
		{
			name:        "wrong type",
			qname:       "foo.example.com.",
			qtype:       dns.TypeMX,
			expectedVal: "",
			expectedOK:  false,
		},
	}

	for _, tt := range tests {
		t := tt
		t.Run(t.Name(), func(t *testing.T) {
			msg, ok := h.hijack(context.Background(), t.qname, t.qtype, new(dns.Msg))
			if ok != t.expectedOK {
				t.Fatalf("expected ok %v, got %v", t.expectedOK, ok)
			}

			if t.expectedOK {
				if len(msg.Answer) == 0 {
					t.Fatalf("expected an answer, got none")
				}
				a, ok := msg.Answer[0].(*dns.A)
				if !ok {
					t.Fatalf("expected A record, got %T", msg.Answer[0])
				}
				if a.A.String() != t.expectedVal {
					t.Fatalf("expected value %s, got %s", t.expectedVal, a.A.String())
				}
			}
		})
	}
}
