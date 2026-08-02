# Hermes

**Hermes** is a DNS management platform with a built-in transparent proxy, REST API, and multi-backend pubsub messaging. It provides a complete solution for managing DNS zones, records, forwarding rules, and hijacking policies through a web API while optionally proxying traffic through SOCKS5 or direct connections.

## Features

- **DNS Server** — Full authoritative DNS server supporting UDP, TCP, DNS-over-TLS (DoT), and DNS-over-HTTPS (DoH)
- **Zone Management** — Create and manage DNS zones with flexible forwarding policies (`none`, `custom`, `default`)
- **Record Management** — Full record type support: A, AAAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, CAA, TLSA, DNSSEC types
- **Query Forwarding** — Forward unmatched queries to upstream DNS servers with multi-protocol support (UDP, TCP, TLS, DoH)
- **Hijack Rules** — Intercept matching queries with block, proxy, forward, or raw-response policies
- **Transparent Proxy** — HTTP/HTTPS proxy with SNI-based TLS routing and configurable SOCKS5 upstream
- **Allowed Host Filtering** — Zone-based access control for proxy traffic, cached for performance
- **PubSub Messaging** — Multiple backends: Go channels (in-process), Redis Streams, Kafka, PostgreSQL, RabbitMQ
- **Service Registry** — Redis-based service discovery with heartbeat health tracking
- **Caching** — Configurable caching with Redis, in-memory (Otter), or no-op backends
- **OpenTelemetry** — Distributed tracing with OTLP/ Jaeger exporters, Prometheus metrics
- **REST API** — Full CRUD API for zones, records, forward zones, hijacks, and settings
- **Database Migrations** — Built-in PostgreSQL migration system

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                      CLI (cobra)                     │
│  api │ dns │ proxy │ pubsub │ migrate               │
└──────┬───────────────────────┬──────────────────────┘
       │                       │
┌──────▼──────────────┐ ┌──────▼──────────────────┐
│    API Server       │ │    DNS Server            │
│  ┌────────────────┐ │ │  ┌────────────────────┐ │
│  │ Zone/Record    │ │ │  │ Zone Resolution    │ │
│  │ CRUD           │ │ │  │ Forwarding         │ │
│  │ Forward Zones  │ │ │  │ Hijacking          │ │
│  │ Hijacks        │ │ │  │ Caching            │ │
│  │ Settings       │ │ │  └────────────────────┘ │
│  └────────────────┘ │ │                         │
└──────┬──────────────┘ │  ┌────────────────────┐ │
       │                │  │ Transparent Proxy  │ │
┌──────▼──────────────┐ │  │  HTTP Router       │ │
│   Proxy Server      │ │  │  SNI/TLS Router    │ │
│  SOCKS5 Upstream    │ │  │  Allowed Host      │ │
│  Zone-level ACL     │ │  └────────────────────┘ │
└─────────────────────┘ └─────────────────────────┘
       │                         │
       └──────────┬──────────────┘
                  │
        ┌─────────▼──────────┐
        │   PostgreSQL        │
        │   Redis             │
        │   PubSub Backend    │
        └────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL
- Redis (optional, required for caching and pubsub)
- Docker (optional, for running dependencies)

### Configuration

Copy the example config and adjust as needed:

```bash
cp configs/local.env .env
```

Key environment variables:

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | auto-generated from `DB_*` vars | PostgreSQL connection string |
| `INSTANCE_NAME` | `server` | Instance identifier for service registry |
| `HTTP_PORT` | `8000` | API server port |
| `DNS_LISTEN_ADDR` | `:53` | DNS server listen address |
| `DNS_PROTOCOL` | `udp` | DNS protocol (`udp`, `tcp`, `both`, `tls`, `doh`) |
| `PROXY_LISTEN_ADDR` | `0.0.0.0` | Proxy listen address |
| `PROXY_HTTP_PORTS` | `1080` | Proxy HTTP ports (comma-separated) |
| `PROXY_TLS_PORTS` | `1443` | Proxy TLS ports (comma-separated) |
| `PROXY_URL` | `` | Upstream SOCKS5 proxy URL (empty = direct connect) |
| `REDIS_HOST` | `` | Redis host |
| `PUBSUB_BACKEND` | `redis` | Pubsub backend (`redis`, `gochannel`, `kafka`, `postgres`, `rabbitmq`) |
| `DNS_CACHE_BACKEND` | `memory` | Cache backend (`redis`, `memory`, `none`) |
| `TRACER_URL` | `` | OpenTelemetry collector URL; the scheme selects the exporter (`grpc://`/`jaeger://` gRPC, `grpcs://` TLS gRPC, `http://`/`https://` OTLP/HTTP), optionally `user:pass@` for basic auth, then the collector address (include the port, e.g. `grpc://localhost:4317`). Empty disables tracing. |
| `TRACER_HEADERS` | `` | Extra headers for OTLP requests as a JSON object, e.g. `{"api-key":"abc","x-tenant":"42"}` (legacy comma-separated `key=value` pairs also accepted). Sent on both HTTP and gRPC; overrides the URL's basic auth for the same header. |
| `LOG_URL` | `` | OpenTelemetry log collector URL (`http://`/`https://` OTLP/HTTP, or a bare `host:port` endpoint, e.g. `http://localhost:4318`). Empty disables OTLP log export. |
| `LOG_HEADERS` | `` | Extra headers for OTLP log requests as a JSON object, e.g. `{"api-key":"abc"}` (legacy comma-separated `key=value` pairs also accepted). |
| `LOG_QUEUE_SIZE` | `2048` | Max records buffered in the OTLP log batch processor queue before export. |
| `LOG_EXPORT_INTERVAL` | `1s` | How often the OTLP log batch processor exports a batch. |
| `LOG_EXPORT_TIMEOUT` | `30s` | Timeout for each OTLP log batch export. |
| `LOG_MAX_BATCH_SIZE` | `512` | Max records per exported OTLP log batch. |
| `LOG_EXPORT_BUFFER_SIZE` | `1` | Per-export copy buffer for the OTLP log batch processor. |
| `LOG_EXPORTER_TIMEOUT` | `10s` | Per-request timeout for the OTLP log HTTP exporter. |
| `LOG_MAX_REQUEST_SIZE` | `64 MiB` | Max OTLP log request body size. |
| `LOG_COMPRESSION` | `` | OTLP log request compression; set to `gzip` to enable. |

### Running with Docker Compose

```bash
docker-compose up -d
```

This starts PostgreSQL, Redis, and the Hermes API server.

### Running from Source

```bash
# Run database migrations
go run main.go migrate

# Start the API server
go run main.go api

# Start the DNS server (requires root for port 53, or change DNS_LISTEN_ADDR)
sudo -E go run main.go dns

# Start the transparent proxy
go run main.go proxy
```

## CLI Commands

| Command | Description |
|---|---|
| `hermes api` | Start the REST API server |
| `hermes dns` | Start the DNS resolver server |
| `hermes proxy` | Start the transparent proxy |
| `hermes migrate` | Run database migrations and exit |
| `hermes pubsub` | Start a standalone pubsub subscriber (debugging) |

### Additional Flags

```bash
# Run migrations before starting
hermes api --migrate
hermes dns --migrate
```

## API Endpoints

The REST API is available at `http://localhost:8000/api/`.

### Zones

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/zones` | List all zones |
| `GET` | `/api/zones/{id}` | Get zone details |
| `POST` | `/api/zones` | Create a zone |
| `POST` | `/api/zones/{id}` | Update a zone |
| `DELETE` | `/api/zones/{id}` | Delete a zone |

### DNS Records

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/zones/{zone}/records` | List records in a zone |
| `GET` | `/api/zones/{zone}/records/{id}` | Get record details |
| `POST` | `/api/zones/{zone}/records` | Create a record |
| `POST` | `/api/zones/{zone}/records/{id}` | Update a record |
| `DELETE` | `/api/zones/{zone}/records/{id}` | Delete a record |

### Forward Zones

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/forward-zones` | List forward zones |
| `GET` | `/api/forward-zones/{id}` | Get forward zone details |
| `POST` | `/api/forward-zones` | Create a forward zone |
| `POST` | `/api/forward-zones/{id}` | Update a forward zone |
| `DELETE` | `/api/forward-zones/{id}` | Delete a forward zone |

### Hijacks

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/hijacks` | List hijack rules |
| `GET` | `/api/hijacks/{id}` | Get hijack details |
| `POST` | `/api/hijacks` | Create a hijack rule |
| `POST` | `/api/hijacks/{id}` | Update a hijack rule |
| `DELETE` | `/api/hijacks/{id}` | Delete a hijack rule |

### Settings & Utilities

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/settings` | Get global settings |
| `POST` | `/api/settings` | Update global settings |
| `GET` | `/api/services/{kind}` | List registered services by kind |
| `POST` | `/api/pubsub/{topic}` | Publish a message to a pubsub topic |
| `POST` | `/api/migrations/run` | Run pending database migrations |
| `GET` | `/api/metrics` | Prometheus metrics endpoint |

### Authentication

The API uses HTTP Basic Authentication. By default:
- Username: `admin`
- Password: `admin`

Set `ADMIN_NO_AUTH=true` to disable authentication for development.

## DNS Forward Address Format

Forward addresses use URL-style strings:

```
udp://1.1.1.1:53
tcp://8.8.8.8:53
tls://1.1.1.1:853
doh://cloudflare-dns.com:443/dns-query
```

## Transparent Proxy

The transparent proxy supports two modes:

1. **HTTP Proxy** — Listens on configured HTTP ports, forwards CONNECT and regular HTTP requests through an optional SOCKS5 upstream
2. **SNI/TLS Proxy** — Listens on configured TLS ports, inspects the TLS ClientHello for the SNI hostname, and forwards the raw TCP connection

Access control is enforced via the `AllowedHost` check against the zones database, with optional Redis caching.

## Project Structure

```
├── api/          # REST API handlers, repository, types, errors
├── auth/         # HTTP Basic Auth middleware
├── cache/        # Cache interface + Redis, memory, no-op implementations
├── cmd/          # CLI subcommands (api, dns, proxy, migrate, pubsub)
├── configs/      # Example configuration files
├── convert/      # Type conversion utilities
├── dns/          # DNS server, resolution, forwarding, hijacking, caching
├── docs/burno/   # Bruno API client collection
├── migrations/   # Database schema migrations
├── models/       # Domain types (Zone, Record, ForwardZone, Hijack, Settings)
├── proxy/        # Transparent proxy (HTTP + SNI/TLS routers)
├── pubsub/       # Pubsub abstraction with multiple backends
├── queries/      # SQL query functions for all entities
├── registry/     # Redis-based service registry with heartbeats
├── request/      # HTTP request parsing utilities
├── runtime/      # Application bootstrap, config loading, telemetry
├── static/       # Web UI static files
├── web/          # HTTP router, middleware, CORS, JSON utilities
└── main.go       # Entry point
```

## Development

### Linting

```bash
go tool golangci-lint run
```

### Testing

```bash
go test ./...
```

### API Client (Bruno)

API route definitions are available in `docs/burno/` for use with [Bruno](https://www.usebruno.com/), an open-source API client.

## License

This project is licensed under the terms of the LICENSE file included in the repository.
