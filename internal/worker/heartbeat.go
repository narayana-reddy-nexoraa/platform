package worker

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/service"
)

const (
	heartbeatInterval      = 10 * time.Second
	maxConsecutiveFailures = 3
)

// Heartbeat runs a background goroutine that periodically extends the lease
// for an active execution. If the lease is lost or too many consecutive
// heartbeat failures occur, it cancels the context to abort processing.
type Heartbeat struct {
	svc         *service.ExecutionService
	executionID uuid.UUID
	workerID    string
	logger      zerolog.Logger
	stopped     atomic.Bool
}

// NewHeartbeat creates a heartbeat for the given execution.
func NewHeartbeat(svc *service.ExecutionService, executionID uuid.UUID, workerID string, logger zerolog.Logger) *Heartbeat {
	return &Heartbeat{
		svc:         svc,
		executionID: executionID,
		workerID:    workerID,
		logger:      logger.With().Str("execution_id", executionID.String()).Str("component", "heartbeat").Logger(),
	}
}

// Run starts the heartbeat loop. Calls cancelFunc if the lease is lost.
func (h *Heartbeat) Run(ctx context.Context, cancelFunc context.CancelFunc) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			h.stopped.Store(true)
			return
		case <-ticker.C:
			_, err := h.svc.SendHeartbeat(ctx, h.executionID, h.workerID)
			if err != nil {
				consecutiveFailures++
				h.logger.Warn().Err(err).
					Int("consecutive_failures", consecutiveFailures).
					Msg("heartbeat failed")

				if consecutiveFailures >= maxConsecutiveFailures {
					h.logger.Error().Msg("too many heartbeat failures, aborting execution")
					h.stopped.Store(true)
					cancelFunc()
					return
				}
				continue
			}

			consecutiveFailures = 0
			h.logger.Debug().Msg("heartbeat sent")
		}
	}
}

// WasStopped returns true if the heartbeat was stopped due to lease loss or failures.
func (h *Heartbeat) WasStopped() bool {
	return h.stopped.Load()
}
