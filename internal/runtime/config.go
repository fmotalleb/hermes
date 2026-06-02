package runtime

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/fmotalleb/go-tools/env"
	"github.com/joho/godotenv"
)

type Config struct {
	InstanceName        string
	HTTPPort            int
	MetricsPort         int
	AdminNoAuth         bool
	AdminUser           string
	AdminPass           string
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
	DNSCacheBackend     string
	DNSCacheTypes       []string
	DNSListenAddr       string
	DNSProtocol         string
}

func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return Config{
		InstanceName:        env.Or("INSTANCE_NAME", "server"),
		HTTPPort:            env.IntOr("HTTP_PORT", 8000),
		MetricsPort:         env.IntOr("METRICS_PORT", 0),
		AdminNoAuth:         env.BoolOr("ADMIN_NO_AUTH", false),
		AdminUser:           env.Or("ADMIN_USER", "admin"),
		AdminPass:           env.Or("ADMIN_PASS", "admin"),
		DBDsn:               env.Or("DATABASE_URL", defaultPostgresDSN()),
		DBMaxIdle:           env.IntOr("DB_MAX_IDLE_CONNECTION", 5),
		DBMaxOpen:           env.IntOr("DB_MAX_OPEN_CONNECTION", 10),
		RedisAddr:           net.JoinHostPort(env.Or("REDIS_HOST", "127.0.0.1"), strconv.Itoa(env.IntOr("REDIS_PORT", 6379))),
		RedisDB:             env.IntOr("REDIS_DB", 0),
		RedisPubSubDB:       env.IntOr("REDIS_PUBSUB_DB", 1),
		PubSubBrokers:       env.Or("PUBSUB_BROKERS", ""),
		PubSubRabbitMQURI:   env.Or("PUBSUB_RABBITMQ_URI", ""),
		PubSubConsumerGroup: env.Or("PUBSUB_CONSUMER_GROUP", "hermes"),
		TraceExporter:       env.Or("TRACE_EXPORTER", "otlp"),
		TracerURL:           env.Or("TRACER_URL", "localhost:4317"),
		TracerRatio:         env.Float64Or("TRACER_RATIO", 1),
		PubSubBackend:       env.Or("PUBSUB_BACKEND", "gochannel"),
		DNSCacheBackend:     env.Or("DNS_CACHE_BACKEND", "memory"),
		DNSCacheTypes:       env.SliceOr("DNS_CACHE_TYPES", []string{"A", "AAAA", "CNAME"}),
		DNSListenAddr:       env.Or("DNS_LISTEN_ADDR", ":53"),
		DNSProtocol:         env.Or("DNS_PROTOCOL", "udp"),
	}
}

func defaultPostgresDSN() string {
	host := env.Or("DB_HOST", "127.0.0.1")
	port := env.IntOr("DB_PORT", 5432)
	user := env.Or("DB_USER", "hermes")
	pass := env.Or("DB_PASSWORD", "hermes")
	name := env.Or("DB_NAME", "hermes")
	sslMode := env.Or("DB_SSL_MODE", "disable")
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, name, sslMode,
	)
}
