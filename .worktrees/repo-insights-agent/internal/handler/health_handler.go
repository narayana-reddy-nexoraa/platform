package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DBPinger is satisfied by *pgxpool.Pool and enables testing with mocks.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler provides liveness and readiness probes.
type HealthHandler struct {
	db DBPinger
}

// NewHealthHandler creates a new health handler that checks db connectivity.
func NewHealthHandler(db DBPinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// LiveHandler responds 200 to indicate the process is alive.
// It does NOT check external dependencies.
func (h *HealthHandler) LiveHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "up"})
}

// ReadyHandler checks database connectivity with a 2-second timeout.
// Returns 200 if ready, 503 if the DB is unreachable.
func (h *HealthHandler) ReadyHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"db":     "down",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"db":     "up",
	})
}

// RegisterRoutes registers all health-check routes on the given router.
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.ReadyHandler)
	router.GET("/health/live", h.LiveHandler)
	router.GET("/health/ready", h.ReadyHandler)
}

// NetHTTPHandler returns a plain net/http handler for use in the worker's
// minimal health server (no Gin dependency needed).
func (h *HealthHandler) NetHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	})

	readyFn := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := h.db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unavailable","db":"down"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","db":"up"}`))
	}

	mux.HandleFunc("/health", readyFn)
	mux.HandleFunc("/health/ready", readyFn)

	return mux
}
