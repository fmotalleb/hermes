package runtime

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/fmotalleb/go-tools/env"
	"github.com/fmotalleb/go-tools/log"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
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
	TracerURL           string
	TracerHeaders       string
	TracerRatio         float64
	LogURL              string
	LogHeaders          string
	MetricPushURL       string        // METRIC_PUSH_URL (OTLP metric push collector URL)
	MetricPushHeaders   string        // METRIC_PUSH_HEADERS (JSON object or key=value pairs)
	MetricPushInterval  time.Duration // METRIC_PUSH_INTERVAL (default 60s)
	LogQueueSize        int           // LOG_QUEUE_SIZE (default 2048)
	LogExportInterval   time.Duration // LOG_EXPORT_INTERVAL (default 1s)
	LogExportTimeout    time.Duration // LOG_EXPORT_TIMEOUT (default 30s)
	LogMaxBatchSize     int           // LOG_MAX_BATCH_SIZE (default 512)
	LogExportBufferSize int           // LOG_EXPORT_BUFFER_SIZE (default 1)
	LogExporterTimeout  time.Duration // LOG_EXPORTER_TIMEOUT (default 10s)
	LogMaxRequestSize   int           // LOG_MAX_REQUEST_SIZE (default 64 MiB)
	LogCompression      string        // LOG_COMPRESSION ("gzip" enables gzip)
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
// It exits the process through the context logger if the .env file cannot be
// loaded.
func LoadConfig(ctx context.Context) Config {
	if err := godotenv.Load(); err != nil {
		log.Of(ctx).Fatal("failed to load .env file", zap.Error(err))
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
		TracerURL:     env.Or("TRACER_URL", ""),
		TracerHeaders: env.Or("TRACER_HEADERS", ""),
		TracerRatio:   env.Float64Or("TRACER_RATIO", 1),

		// OTEL log export
		LogURL:              env.Or("LOG_URL", ""),
		LogHeaders:          env.Or("LOG_HEADERS", ""),
		LogQueueSize:        env.IntOr("LOG_QUEUE_SIZE", 2048),
		LogExportInterval:   env.DurationOr("LOG_EXPORT_INTERVAL", time.Second),
		LogExportTimeout:    env.DurationOr("LOG_EXPORT_TIMEOUT", 30*time.Second),
		LogMaxBatchSize:     env.IntOr("LOG_MAX_BATCH_SIZE", 512),
		LogExportBufferSize: env.IntOr("LOG_EXPORT_BUFFER_SIZE", 1),
		LogExporterTimeout:  env.DurationOr("LOG_EXPORTER_TIMEOUT", 10*time.Second),
		LogMaxRequestSize:   env.IntOr("LOG_MAX_REQUEST_SIZE", 64*1024*1024),
		LogCompression:      env.Or("LOG_COMPRESSION", ""),

		// OTEL metric push export (same URL/header parsing as the tracer and log connectors)
		MetricPushURL:      env.Or("METRIC_PUSH_URL", ""),
		MetricPushHeaders:  env.Or("METRIC_PUSH_HEADERS", ""),
		MetricPushInterval: env.DurationOr("METRIC_PUSH_INTERVAL", time.Minute),

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
