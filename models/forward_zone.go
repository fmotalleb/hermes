package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ForwardAddress struct {
	Protocol      string `json:"protocol"` // udp, tcp, tls, https
	Address       string `json:"address"`
	Port          int    `json:"port"`
	TLSServerName string `json:"tls_servername,omitempty"`
	DOHPath       string `json:"doh_path,omitempty"`
}

func parseForwardAddress(raw string) (*ForwardAddress, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("missing host")
	}

	portStr := u.Port()
	if portStr == "" {
		return nil, errors.New("missing port")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	addr := &ForwardAddress{
		Address: host,
		Port:    port,
	}

	switch strings.ToLower(u.Scheme) {
	case "udp":
		addr.Protocol = "udp"

	case "tcp":
		addr.Protocol = "tcp"

	case "tls":
		addr.Protocol = "tls"
		addr.TLSServerName = host

	case "doh":
		addr.Protocol = "https"
		addr.TLSServerName = host

		path := u.EscapedPath()
		if path == "" {
			path = "/"
		}

		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}

		addr.DOHPath = path

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}

	return addr, nil
}

func (f *ForwardAddress) UnmarshalJSON(data []byte) error {
	var raw string

	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("forward address must be string: %w", err)
	}

	parsed, err := parseForwardAddress(raw)
	if err != nil {
		return err
	}

	*f = *parsed

	return nil
}

func (f ForwardAddress) MarshalJSON() ([]byte, error) {
	scheme := f.Protocol

	if f.Protocol == "https" {
		scheme = "doh"
	}

	host := f.Address
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	out := fmt.Sprintf("%s://%s:%d", scheme, host, f.Port)

	if f.Protocol == "https" {
		path := f.DOHPath
		if path == "" {
			path = "/"
		}

		out += path
	}

	return json.Marshal(out)
}

type ForwardZone struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Addresses []ForwardAddress `json:"addresses"`
	ZoneCount int              `json:"zone_count,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type ForwardPolicy = string

const (
	ForwardPolicyNone    = ForwardPolicy("none")
	ForwardPolicyCustom  = ForwardPolicy("custom")
	ForwardPolicyDefault = ForwardPolicy("default")
)
