// Package logging builds the process-wide structured logger.
//
// The service logs with the standard library's log/slog: JSON in deployed
// environments (for log shipping and querying) and readable text locally.
package logging

import (
	"io"
	"log/slog"
)

// New returns a logger that writes to w. Every record carries the service
// name and environment, so logs from several services and environments can
// share one backend without ambiguity.
func New(w io.Writer, level slog.Level, format, service, env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level, ReplaceAttr: utcTime}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler).With(
		slog.String("service", service),
		slog.String("env", env),
	)
}

// utcTime writes record timestamps in UTC, whatever the host's time zone, so
// logs from machines and regions line up.
func utcTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 && a.Value.Kind() == slog.KindTime {
		a.Value = slog.TimeValue(a.Value.Time().UTC())
	}
	return a
}
