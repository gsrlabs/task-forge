// internal/database/pgx_logger.go
package database

import (
	"context"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

// PgxZerologAdapter adapts the tracelog.Logger interface to zerolog
type PgxZerologAdapter struct {
	logger zerolog.Logger
}

func (l *PgxZerologAdapter) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	var event *zerolog.Event

	switch level {
	case tracelog.LogLevelTrace:
		event = l.logger.Trace()
	case tracelog.LogLevelDebug:
		event = l.logger.Debug()
	case tracelog.LogLevelInfo:
		event = l.logger.Info()
	case tracelog.LogLevelWarn:
		event = l.logger.Warn()
	case tracelog.LogLevelError:
		event = l.logger.Error()
	default:
		event = l.logger.Debug()
	}

	// Добавляем контекст SQL запроса (сам запрос, время, количество затронутых строк)
	if len(data) > 0 {
		for k, v := range data {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}