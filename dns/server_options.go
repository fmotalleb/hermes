package dns

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
)

const defaultListenAddr = "0.0.0.0:8053"

// Protocol represents the network protocols the DNS server can listen on.
type Protocol int

const (
	ProtocolUDP   Protocol = iota // UDP only
	ProtocolTCP                   // TCP only
	ProtocolBoth                  // TCP + UDP (default)
	ProtocolTLS                   // DNS-over-TLS (DoT)
	ProtocolHTTPS                 // DNS-over-HTTPS (DoH)
)

func (p Protocol) String() string {
	switch p {
	case ProtocolUDP:
		return "udp"
	case ProtocolTCP:
		return "tcp"
	case ProtocolTLS:
		return "tls"
	case ProtocolBoth:
		return "udp+tcp"
	case ProtocolHTTPS:
		return "doh"
	default:
		return ""
	}
}

// ProtocolFromStr converts a protocol string ("udp", "tcp", "tls", "doh", etc.)
// to its Protocol enum value. Returns an error for unrecognized values.
func ProtocolFromStr(protoStr string) (Protocol, error) {
	switch strings.ToLower(protoStr) {
	case "udp":
		return ProtocolUDP, nil
	case "tcp":
		return ProtocolTCP, nil
	case "both", "udp+tcp", "tcp+udp", "":
		return ProtocolBoth, nil
	case "tls":
		return ProtocolTLS, nil
	case "doh", "https":
		return ProtocolHTTPS, nil
	default:
		return 0, fmt.Errorf("invalid dns protocol: %s", protoStr)
	}
}

// ServerConfig holds all configuration for the DNS server listener.
type ServerConfig struct {
	listenAddr string
	protocol   Protocol
	tlsConfig  *tls.Config
	certFile   string
	keyFile    string
	httpPath   string // DoH endpoint path
}

// ServerOption is a functional option for configuring the DNS server.
type ServerOption func(*ServerConfig) error

// defaultServerConfig returns a config with sensible defaults.
func defaultServerConfig() *ServerConfig {
	return &ServerConfig{
		listenAddr: defaultListenAddr,
		protocol:   ProtocolBoth,
		httpPath:   "/dns-query",
	}
}

// WithListenAddr sets the address and port to listen on (e.g. "0.0.0.0:53").
func WithListenAddr(addr string) ServerOption {
	return func(c *ServerConfig) error {
		if addr == "" {
			return errors.New("listen address must not be empty")
		}
		c.listenAddr = addr
		return nil
	}
}

// WithProtocol sets the protocol(s) the server will accept.
func WithProtocol(p Protocol) ServerOption {
	return func(c *ServerConfig) error {
		c.protocol = p
		return nil
	}
}

// WithTLSFiles configures DNS-over-TLS using PEM certificate and key files.
// Implies ProtocolTLS unless overridden after this option.
func WithTLSFiles(certFile, keyFile string) ServerOption {
	return func(c *ServerConfig) error {
		if certFile == "" || keyFile == "" {
			return errors.New("both certFile and keyFile must be provided for TLS")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("loading TLS key pair: %w", err)
		}
		c.certFile = certFile
		c.keyFile = keyFile
		c.tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		return nil
	}
}

// WithTLSConfig supplies a pre-built *tls.Config (e.g. with custom CAs or
// client-auth settings). Implies ProtocolTLS unless overridden after.
func WithTLSConfig(cfg *tls.Config) ServerOption {
	return func(c *ServerConfig) error {
		if cfg == nil {
			return errors.New("tls.Config must not be nil")
		}
		c.tlsConfig = cfg
		c.protocol = ProtocolTLS
		return nil
	}
}

// WithHTTPS enables DNS-over-HTTPS. Requires TLS to be configured first via
// WithTLSFiles or WithTLSConfig. Optionally overrides the default DoH path.
func WithHTTPS(path string) ServerOption {
	return func(c *ServerConfig) error {
		if c.tlsConfig == nil {
			return errors.New("WithHTTPS requires TLS to be configured first (use WithTLSFiles or WithTLSConfig)")
		}
		if path != "" {
			c.httpPath = path
		}
		c.protocol = ProtocolHTTPS
		return nil
	}
}
