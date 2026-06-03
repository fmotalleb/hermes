package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/fmotalleb/hermes/registry"
)

type App struct {
	id     uuid.UUID
	Config Config
	Logger *slog.Logger
	DB     *sql.DB
	Redis  *redis.Client

	TraceProvider  *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	MetricsHandler http.Handler

	httpServer    *http.Server
	metricsServer *http.Server

	serviceRegistry *registry.RegistryConnection
}

func New(ctx context.Context, kind string, logger *slog.Logger) (*App, error) {
	var id uuid.UUID
	var err error
	if id, err = uuid.NewV7(); err != nil {
		return nil, err
	}
	cfg := LoadConfig()
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

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

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = db.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	traceProvider, meterProvider, metricsHandler, err := setupTelemetry(ctx, cfg, logger)
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
	if err := serviceRegistry.Start(ctx); err != nil {
		logger.Error("registry advertise failed")
	}
	return &App{
		id:              id,
		Config:          cfg,
		Logger:          logger,
		DB:              db,
		Redis:           redisClient,
		TraceProvider:   traceProvider,
		MeterProvider:   meterProvider,
		MetricsHandler:  metricsHandler,
		serviceRegistry: serviceRegistry,
	}, nil
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
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.httpServer.Shutdown(shutdownCtx)
	}()

	a.Logger.Info("http server started", "addr", addr)
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
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.metricsServer.Shutdown(shutdownCtx)
	}()

	a.Logger.Info("metrics server started", "addr", addr)
	err := a.metricsServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func setupTelemetry(ctx context.Context, cfg Config, logger *slog.Logger) (*sdktrace.TracerProvider, *metric.MeterProvider, http.Handler, error) {
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

	logger.Info("telemetry configured", "trace_exporter", cfg.TraceExporter, "tracer_url", cfg.TracerURL)
	return traceProvider, meterProvider, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

func newTraceProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TracerRatio))

	var exporter sdktrace.SpanExporter
	var err error
	switch strings.ToLower(cfg.TraceExporter) {
	case "otlp", "otel", "jaeger":
		conn, connErr := grpcDial(ctx, cfg.TracerURL)
		if connErr != nil {
			return nil, connErr
		}
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithGRPCConn(conn),
		)
	default:
		exporter = nil
	}
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
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

func grpcDial(ctx context.Context, target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
