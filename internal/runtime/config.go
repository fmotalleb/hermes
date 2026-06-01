package runtime

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPPort         int
	MetricsPort      int
	AdminNoAuth      bool
	AdminUser        string
	AdminPass        string
	DBDsn            string
	DBMaxIdle        int
	DBMaxOpen        int
	RedisAddr        string
	RedisDB          int
	RedisPubSubDB    int
	RedisPubSubMode  string
	TraceExporter    string
	TracerURL        string
	TracerRatio      float64
	PubSubBackend    string
	CacheBackend     string
	DNSListenAddr    string
	DNSProtocol      string
}

func LoadConfig() Config {
	return Config{
		HTTPPort:        envInt("HTTP_PORT", 8000),
		MetricsPort:     envInt("METRICS_PORT", 0),
		AdminNoAuth:     envBool("ADMIN_NO_AUTH", false),
		AdminUser:       envString("ADMIN_USER", "admin"),
		AdminPass:       envString("ADMIN_PASS", "admin"),
		DBDsn:           envString("DATABASE_URL", defaultPostgresDSN()),
		DBMaxIdle:       envInt("DB_MAX_IDLE_CONNECTION", 5),
		DBMaxOpen:       envInt("DB_MAX_OPEN_CONNECTION", 10),
		RedisAddr:       net.JoinHostPort(envString("REDIS_HOST", "127.0.0.1"), strconv.Itoa(envInt("REDIS_PORT", 6379))),
		RedisDB:         envInt("REDIS_DB", 0),
		RedisPubSubDB:   envInt("REDIS_PUBSUB_DB", 1),
		RedisPubSubMode: envString("REDIS_PUBSUB_MODE", "pubsub"),
		TraceExporter:   envString("TRACE_EXPORTER", "otlp"),
		TracerURL:       envString("TRACER_URL", "localhost:4317"),
		TracerRatio:     envFloat("TRACER_RATIO", 1),
		PubSubBackend:   envString("PUBSUB_BACKEND", "redis"),
		CacheBackend:    envString("DNS_CACHE_BACKEND", "memory"),
		DNSListenAddr:   envString("DNS_LISTEN_ADDR", ":53"),
		DNSProtocol:     envString("DNS_PROTOCOL", "udp"),
	}
}

func defaultPostgresDSN() string {
	host := envString("DB_HOST", "127.0.0.1")
	port := envInt("DB_PORT", 5432)
	user := envString("DB_USER", "hermes")
	pass := envString("DB_PASSWORD", "hermes")
	name := envString("DB_NAME", "hermes")
	sslMode := envString("DB_SSL_MODE", "disable")
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, name, sslMode,
	)
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "t", "yes", "y":
		return true
	case "0", "false", "f", "no", "n":
		return false
	default:
		return fallback
	}
}

