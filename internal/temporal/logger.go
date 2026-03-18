package temporal

import (
	"fmt"

	"github.com/rs/zerolog"
	tlog "go.temporal.io/sdk/log"
)

// zerologAdapter bridges Temporal's log interface to zerolog.
type zerologAdapter struct {
	logger zerolog.Logger
}

func newZerologAdapter(logger zerolog.Logger) tlog.Logger {
	return &zerologAdapter{logger: logger.With().Str("component", "temporal").Logger()}
}

func (z *zerologAdapter) Debug(msg string, keyvals ...interface{}) {
	z.logger.Debug().Fields(kvsToMap(keyvals)).Msg(msg)
}

func (z *zerologAdapter) Info(msg string, keyvals ...interface{}) {
	z.logger.Info().Fields(kvsToMap(keyvals)).Msg(msg)
}

func (z *zerologAdapter) Warn(msg string, keyvals ...interface{}) {
	z.logger.Warn().Fields(kvsToMap(keyvals)).Msg(msg)
}

func (z *zerologAdapter) Error(msg string, keyvals ...interface{}) {
	z.logger.Error().Fields(kvsToMap(keyvals)).Msg(msg)
}

// kvsToMap converts Temporal's key-value pairs to a map for zerolog.
func kvsToMap(keyvals []interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(keyvals)/2)
	for i := 0; i < len(keyvals)-1; i += 2 {
		key := fmt.Sprintf("%v", keyvals[i])
		m[key] = keyvals[i+1]
	}
	return m
}
