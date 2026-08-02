package pubsub

import (
	"github.com/ThreeDotsLabs/watermill"
	"go.uber.org/zap"
)

// zapLoggerAdapter adapts a *zap.Logger to watermill's LoggerAdapter so pubsub
// backends emit their diagnostics through the shared context logger (and thus
// into the OTLP log pipeline when configured).
type zapLoggerAdapter struct {
	logger *zap.Logger
	fields watermill.LogFields
}

// NewZapLoggerAdapter wraps a zap logger as a watermill logger. Pass a logger
// derived from the request/application context (see [log.FromContext]) so
// pubsub logs inherit the same fields and sinks as everything else. A nil
// logger yields a no-op adapter.
func NewZapLoggerAdapter(logger *zap.Logger) watermill.LoggerAdapter {
	if logger == nil {
		return watermill.NopLogger{}
	}
	return &zapLoggerAdapter{logger: logger}
}

func (l *zapLoggerAdapter) Error(msg string, err error, fields watermill.LogFields) {
	l.logger.Error(msg, append(l.zapFields(fields), zap.Error(err))...)
}

func (l *zapLoggerAdapter) Info(msg string, fields watermill.LogFields) {
	l.logger.Info(msg, l.zapFields(fields)...)
}

func (l *zapLoggerAdapter) Debug(msg string, fields watermill.LogFields) {
	l.logger.Debug(msg, l.zapFields(fields)...)
}

func (l *zapLoggerAdapter) Trace(msg string, fields watermill.LogFields) {
	// zap has no trace level; debug is the closest.
	l.logger.Debug(msg, l.zapFields(fields)...)
}

func (l *zapLoggerAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &zapLoggerAdapter{logger: l.logger, fields: l.fields.Add(fields)}
}

func (l *zapLoggerAdapter) zapFields(fields watermill.LogFields) []zap.Field {
	merged := l.fields.Add(fields)
	out := make([]zap.Field, 0, len(merged))
	for k, v := range merged {
		out = append(out, zap.Any(k, v))
	}
	return out
}
