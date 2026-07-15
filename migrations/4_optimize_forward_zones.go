package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/lib/pq"
)

type forwardAddress struct {
	Protocol      string `json:"protocol"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	TLSServerName string `json:"tls_servername,omitempty"`
	DOHPath       string `json:"doh_path,omitempty"`
}

func optimizeForwardZones() Migration {
	return Migration{
		Version: 4,
		Name:    "optimize_forward_zones",
		Up: func(ctx context.Context, db DBTX) error {
			// 1. Add addresses_jsonb column
			_, err := db.ExecContext(ctx, "ALTER TABLE forward_zones ADD COLUMN addresses_jsonb JSONB DEFAULT '[]'::jsonb;")
			if err != nil {
				return err
			}

			// 2. Migrate data
			rows, err := db.QueryContext(ctx, "SELECT id, addresses FROM forward_zones;")
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var id string
				var addresses pq.StringArray
				if err = rows.Scan(&id, &addresses); err != nil {
					return err
				}

				var structured []forwardAddress
				for _, addr := range addresses {
					structured = append(structured, parseLegacyAddress(addr))
				}

				buf, err := json.Marshal(structured)
				if err != nil {
					return err
				}

				_, err = db.ExecContext(ctx, "UPDATE forward_zones SET addresses_jsonb = $1 WHERE id = $2;", buf, id)
				if err != nil {
					return err
				}
			}

			if err := rows.Err(); err != nil {
				return err
			}

			// 3. Drop old addresses and rename
			_, err = db.ExecContext(ctx, "ALTER TABLE forward_zones DROP COLUMN addresses;")
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, "ALTER TABLE forward_zones RENAME COLUMN addresses_jsonb TO addresses;")
			if err != nil {
				return err
			}

			return nil
		},
	}
}

func parseLegacyAddress(addr string) forwardAddress {
	protocol := "udp"
	target := addr

	switch {
	case strings.HasPrefix(addr, "udp://"):
		protocol = "udp"
		target = strings.TrimPrefix(addr, "udp://")
	case strings.HasPrefix(addr, "tcp://"):
		protocol = "tcp"
		target = strings.TrimPrefix(addr, "tcp://")
	case strings.HasPrefix(addr, "tls://"):
		protocol = "tls"
		target = strings.TrimPrefix(addr, "tls://")
	case strings.HasPrefix(addr, "https://"):
		protocol = "https"
		target = strings.TrimPrefix(addr, "https://")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		host = target
		portStr = "53"
		switch protocol {
		case "tls":
			portStr = "853"
		case "https":
			portStr = "443"
		}
	}

	port := 53
	switch protocol {
	case "tls":
		port = 853
	case "https":
		port = 443
	}

	// Simple port parsing
	var p int
	if _, err := fmt.Sscanf(portStr, "%d", &p); err == nil {
		port = p
	}

	dohPath := ""
	if protocol == "https" {
		parts := strings.SplitN(host, "/", 2)
		host = parts[0]
		if len(parts) > 1 {
			dohPath = "/" + parts[1]
		} else {
			dohPath = "/dns-query"
		}
	}

	return forwardAddress{
		Protocol: protocol,
		Address:  host,
		Port:     port,
		DOHPath:  dohPath,
	}
}
