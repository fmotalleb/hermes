package runtime

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	promclient "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/fmotalleb/go-tools/log"

	"github.com/fmotalleb/hermes/otellog"
	"github.com/fmotalleb/hermes/registry"
)

// App represents the runtime application with database, Redis, telemetry, and service registry.
type App struct {
	id     uuid.UUID
	Config Config
	DB     *sql.DB
	Redis  *redis.Client

	TraceProvider  *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	MetricsHandler http.Handler

	ctx context.Context

	httpServer    *http.Server
	metricsServer *http.Server

	ServiceRegistry *registry.RegistryConnection
}

// New creates and initializes a new App instance.
// It connects to the database and Redis, configures OpenTelemetry, and starts the service registry heartbeat.
func New(ctx context.Context, kind string) (*App, error) {
	var id uuid.UUID
	var err error
	if id, err = uuid.NewV7(); err != nil {
		return nil, err
	}
	cfg := LoadConfig()

	// OTEL log export: when LOG_URL is set, tee the context logger to the
	// configured collector so every record emitted through log.FromContext is
	// exported as well.
	if strings.TrimSpace(cfg.LogURL) != "" {
		var headers map[string]string
		if headers, err = parseOTLPHeaders(cfg.LogHeaders); err != nil {
			return nil, fmt.Errorf("parse log headers: %w", err)
		}
		if ctx, err = otellog.Integrate(ctx, cfg.LogURL, headers); err != nil {
			return nil, fmt.Errorf("integrate otlp logging: %w", err)
		}
	}

	logger := log.Of(ctx)
	driverName, err := otelsql.Register(
		"postgres",
		otelsql.WithAttributes(
			semconv.DBSystemPostgreSQL,
		),
		otelsql.WithSQLCommenter(true),
	)
	if err != nil {
		return nil, fmt.Errorf("register otelsql driver: %w", err)
	}

	db, err := sql.Open(driverName, cfg.DBDsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// db, err := sql.Open("postgres", cfg.DBDsn)
	// if err != nil {
	// 	return nil, fmt.Errorf("open database: %w", err)
	// }
	db.SetMaxIdleConns(cfg.DBMaxIdle)
	db.SetMaxOpenConns(cfg.DBMaxOpen)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})
	if err = redisClient.Ping(ctx).Err(); err != nil {
		_ = db.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	traceProvider, meterProvider, metricsHandler, err := setupTelemetry(ctx, cfg)
	if err != nil {
		_ = db.Close()
		_ = redisClient.Close()
		return nil, err
	}

	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // W3C traceparent/tracestate
			propagation.Baggage{},      // optional: baggage propagation
		),
	)

	// TODO: add retry mechanism
	serviceRegistry := registry.NewRegistryConnection(redisClient, id.String(), kind, map[string]any{})
	instanceIP := net.ParseIP(cfg.InstanceAddr)
	if err := serviceRegistry.Start(ctx, instanceIP); err != nil {
		logger.Error("registry advertise failed")
	}
	return &App{
		id:              id,
		Config:          cfg,
		DB:              db,
		Redis:           redisClient,
		TraceProvider:   traceProvider,
		MeterProvider:   meterProvider,
		MetricsHandler:  metricsHandler,
		ctx:             ctx,
		ServiceRegistry: serviceRegistry,
	}, nil
}

// Context returns the application context, which carries the configured zap
// logger (see log.FromContext) and any other values attached during bootstrap.
func (a *App) Context() context.Context {
	return a.ctx
}

func (a *App) ID() uuid.UUID {
	return a.id
}

func (a *App) Close(ctx context.Context) error {
	var errs []error
	if a.httpServer != nil {
		errs = append(errs, a.httpServer.Shutdown(ctx))
	}
	if a.metricsServer != nil {
		errs = append(errs, a.metricsServer.Shutdown(ctx))
	}
	if a.TraceProvider != nil {
		errs = append(errs, a.TraceProvider.Shutdown(ctx))
	}
	if a.MeterProvider != nil {
		errs = append(errs, a.MeterProvider.Shutdown(ctx))
	}
	if a.Redis != nil {
		errs = append(errs, a.Redis.Close())
	}
	if a.DB != nil {
		errs = append(errs, a.DB.Close())
	}
	if err := log.Of(ctx).Sync(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (a *App) HTTPClient() *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
}

func (a *App) StartHTTPServer(ctx context.Context, handler http.Handler) error {
	addr := fmt.Sprintf(":%d", a.Config.HTTPPort)
	a.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() { //nolint:gosec // shutdown needs fresh context; parent is already canceled at this point
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.httpServer.Shutdown(shutdownCtx)
	}()

	log.Of(ctx).Info("http server started", zap.String("addr", addr))
	err := a.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *App) StartMetricsServer(ctx context.Context, exporter http.Handler) error {
	if a.Config.MetricsPort <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", a.Config.MetricsPort)
	a.metricsServer = &http.Server{
		Addr:              addr,
		Handler:           exporter,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() { //nolint:gosec // shutdown needs fresh context; parent is already canceled at this point
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.metricsServer.Shutdown(shutdownCtx)
	}()
	log.Of(ctx).Info("metrics server started", zap.String("addr", addr))
	err := a.metricsServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func setupTelemetry(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, *metric.MeterProvider, http.Handler, error) {
	resourceAttrs := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("hermes."+cfg.InstanceName),
	)

	traceProvider, err := newTraceProvider(ctx, cfg, resourceAttrs)
	if err != nil {
		return nil, nil, nil, err
	}

	registry := promclient.NewRegistry()
	promExporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(resourceAttrs),
		metric.WithReader(promExporter),
	)

	log.Of(ctx).Info("telemetry configured", zap.String("tracer_url", cfg.TracerURL))
	return traceProvider, meterProvider, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

func newTraceProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TracerRatio))

	var exporter sdktrace.SpanExporter
	if target := strings.TrimSpace(cfg.TracerURL); target != "" {
		var err error
		if exporter, err = newTraceExporter(ctx, target, cfg.TracerHeaders); err != nil {
			return nil, err
		}
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

// newTraceExporter creates the span exporter matching the scheme of the
// configured tracer URL. An empty TRACER_URL disables tracing entirely.
// rawHeaders is the TRACER_HEADERS value: a JSON object or comma-separated
// key=value pairs.
func newTraceExporter(ctx context.Context, raw, rawHeaders string) (sdktrace.SpanExporter, error) {
	ep, err := parseTraceEndpoint(raw)
	if err != nil {
		return nil, err
	}
	if ep.headers, err = parseOTLPHeaders(rawHeaders); err != nil {
		return nil, err
	}

	switch ep.kind {
	case "grpc", "grpcs":
		return newGRPCTraceExporter(ctx, ep)
	case "http", "https":
		return newHTTPTraceExporter(ctx, ep)
	default:
		return nil, fmt.Errorf("unsupported tracer scheme %q", ep.kind)
	}
}

func newGRPCTraceExporter(ctx context.Context, ep traceEndpoint) (sdktrace.SpanExporter, error) {
	creds := insecure.NewCredentials()
	if ep.kind == "grpcs" {
		creds = credentials.NewTLS(&tls.Config{})
	}
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if ep.user != "" || len(ep.headers) > 0 {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(rpcMetadata{
			username: ep.user,
			password: ep.pass,
			headers:  ep.headers,
		}))
	}

	conn, err := grpc.NewClient(ep.url, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial tracer %q: %w", ep.url, err)
	}
	return otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
}

func newHTTPTraceExporter(ctx context.Context, ep traceEndpoint) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(ep.url)}
	if headers := mergeTracerHeaders(ep.headers, ep.user, ep.pass); len(headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(headers))
	}
	return otlptracehttp.New(ctx, opts...)
}

// traceEndpoint is a parsed tracer URL: the exporter kind derived from the URL
// scheme, the collector address, optional basic auth credentials, and extra
// headers to attach to every OTLP request.
type traceEndpoint struct {
	kind    string // "grpc", "grpcs", "http" or "https"
	url     string // collector address without credentials
	user    string
	pass    string
	headers map[string]string
}

// parseTraceEndpoint derives the exporter kind from the URL scheme and splits
// the remaining URL into address and optional basic auth credentials.
func parseTraceEndpoint(raw string) (traceEndpoint, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return traceEndpoint{}, fmt.Errorf("parse tracer url %q: %w", raw, err)
	}

	kind := strings.ToLower(u.Scheme)
	switch kind {
	case "jaeger":
		kind = "grpc"
	case "grpc", "grpcs", "http", "https":
	default:
		return traceEndpoint{}, fmt.Errorf("unsupported tracer url scheme %q (want grpc, jaeger, grpcs, http or https)", u.Scheme)
	}

	if u.Host == "" {
		return traceEndpoint{}, fmt.Errorf("tracer url %q is missing an address", raw)
	}

	ep := traceEndpoint{kind: kind, url: u.Host}
	if u.User != nil {
		ep.user = u.User.Username()
		ep.pass, _ = u.User.Password()
	}
	if kind == "http" || kind == "https" {
		// Keep scheme and path for the HTTP exporter, but drop credentials and
		// trailing slashes so the default /v1/traces path is not overridden by "/".
		u.User = nil
		u.Path = strings.TrimRight(u.Path, "/")
		ep.url = u.String()
	}
	return ep, nil
}

// parseOTLPHeaders parses an OTLP header configuration value (TRACER_HEADERS
// or LOG_HEADERS), either as a JSON object of string headers, e.g.
// `{"api-key":"abc","x-tenant":"42"}`, or as the legacy comma-separated
// key=value pairs, e.g. "api-key=abc,x-tenant=42". An empty value yields no
// headers; malformed input is an error.
func parseOTLPHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}

	if strings.HasPrefix(raw, "{") {
		var headers map[string]string
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return nil, fmt.Errorf("parse otlp headers as json: %w", err)
		}
		return headers, nil
	}

	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid otlp header %q (want key=value or json object)", pair)
		}
		headers[key] = strings.TrimSpace(value)
	}
	return headers, nil
}

// mergeTracerHeaders merges the custom headers with the basic auth derived from
// the URL. An explicitly configured authorization header wins over the URL's
// basic auth.
func mergeTracerHeaders(custom map[string]string, user, pass string) map[string]string {
	headers := make(map[string]string, len(custom)+1)
	for k, v := range custom {
		headers[k] = v
	}
	if user != "" && !headerKeyExists(headers, "authorization") {
		headers["authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}
	return headers
}

// headerKeyExists reports whether headers contains key, ignoring case.
func headerKeyExists(headers map[string]string, key string) bool {
	for k := range headers {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// rpcMetadata implements grpc credentials.PerRPCCredentials, sending custom
// headers and optional basic auth as per-RPC metadata. Credentials are sent in
// plaintext when used over an insecure grpc:// transport.
type rpcMetadata struct {
	username string
	password string
	headers  map[string]string
}

func (m rpcMetadata) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return mergeTracerHeaders(m.headers, m.username, m.password), nil
}

func (rpcMetadata) RequireTransportSecurity() bool {
	return false
}
