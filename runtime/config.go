package runtime

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/fmotalleb/go-tools/env"
	"github.com/joho/godotenv"
)

// Config holds all configuration values loaded from environment variables.
type Config struct {
	// Per instance configuration
	InstanceName string
	InstanceAddr string
	MetricsPort  int

	// API Service
	HTTPPort    int
	AdminNoAuth bool
	AdminUser   string
	AdminPass   string

	// Global Configuration
	DBDsn               string
	DBMaxIdle           int
	DBMaxOpen           int
	RedisAddr           string
	RedisDB             int
	RedisPubSubDB       int
	PubSubBrokers       string
	PubSubRabbitMQURI   string
	PubSubConsumerGroup string
	TraceExporter       string
	TracerURL           string
	TracerRatio         float64
	PubSubBackend       string

	// DNS specific section
	DNSCacheBackend       string
	DNSCacheTypes         []string
	DNSListenAddr         string
	DNSProtocol           string
	DNSTLSCertificateFile string
	DNSTLSPrivateKeyFile  string

	// Transparent Proxy Specific section
	ProxyListenAddr      string
	ProxyServerHTTPPorts []string
	ProxyServerTLSPorts  []string
	ProxyURL             string
	ProxyTimeout         time.Duration
}

// LoadConfig reads environment variables and returns a fully populated Config.
// It panics if the .env file cannot be loaded.
func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return Config{
		InstanceName: env.Or("INSTANCE_NAME", "server"),
		InstanceAddr: env.Or("INSTANCE_ADDRESS", "127.0.0.1"),
		HTTPPort:     env.IntOr("HTTP_PORT", 8000),
		MetricsPort:  env.IntOr("METRICS_PORT", 0),

		// Basic Auth setup
		AdminNoAuth: env.BoolOr("ADMIN_NO_AUTH", false),
		AdminUser:   env.Or("ADMIN_USER", "admin"),
		AdminPass:   env.Or("ADMIN_PASS", "admin"),

		// DB
		DBDsn:     env.Or("DATABASE_URL", defaultPostgresDSN()),
		DBMaxIdle: env.IntOr("DB_MAX_IDLE_CONNECTION", 5),
		DBMaxOpen: env.IntOr("DB_MAX_OPEN_CONNECTION", 10),

		// Redis
		RedisAddr:     net.JoinHostPort(env.Or("REDIS_HOST", ""), strconv.Itoa(env.IntOr("REDIS_PORT", 6379))),
		RedisDB:       env.IntOr("REDIS_DB", 0),
		RedisPubSubDB: env.IntOr("REDIS_PUBSUB_DB", 1),

		// Pubsub
		PubSubBrokers:       env.Or("PUBSUB_BROKERS", ""),
		PubSubRabbitMQURI:   env.Or("PUBSUB_RABBITMQ_URI", ""),
		PubSubConsumerGroup: env.Or("PUBSUB_CONSUMER_GROUP", "hermes"),
		PubSubBackend:       env.Or("PUBSUB_BACKEND", "redis"),

		// OTEL tracer
		TraceExporter: env.Or("TRACE_EXPORTER", ""),
		TracerURL:     env.Or("TRACER_URL", ""),
		TracerRatio:   env.Float64Or("TRACER_RATIO", 1),

		// DNS
		DNSCacheBackend: env.Or("DNS_CACHE_BACKEND", "memory"),
		DNSCacheTypes:   env.SliceOr("DNS_CACHE_TYPES", []string{"A", "AAAA", "CNAME"}),
		DNSListenAddr:   env.Or("DNS_LISTEN_ADDR", ":53"),
		DNSProtocol:     env.Or("DNS_PROTOCOL", "udp"),

		DNSTLSCertificateFile: env.Or("DNS_TLS_CERTIFICATE", ""),
		DNSTLSPrivateKeyFile:  env.Or("DNS_TLS_PRIVATE_KEY", ""),

		ProxyURL:             env.Or("PROXY_URL", ""),
		ProxyListenAddr:      env.Or("PROXY_LISTEN_ADDR", "0.0.0.0"),
		ProxyServerHTTPPorts: env.SliceOr("PROXY_HTTP_PORTS", []string{"1080"}),
		ProxyServerTLSPorts:  env.SliceOr("PROXY_TLS_PORTS", []string{"1443"}),
		ProxyTimeout:         env.DurationOr("PROXY_TIMEOUT", time.Minute),
	}
}

func defaultPostgresDSN() string {
	host := env.Or("DB_HOST", "")
	port := env.IntOr("DB_PORT", 5432)
	user := env.Or("DB_USER", "")
	pass := env.Or("DB_PASSWORD", "")
	name := env.Or("DB_NAME", "")
	sslMode := env.Or("DB_SSL_MODE", "disable")
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, name, sslMode,
	)
}
