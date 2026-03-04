package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// ErrorResponse is the standard error JSON shape.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// mapDomainError translates a domain error into an HTTP status code and response body.
func mapDomainError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *domain.ErrNotFound:
		c.JSON(http.StatusNotFound, ErrorResponse{Error: e.Error(), Code: "NOT_FOUND"})
	case *domain.ErrIdempotencyConflict:
		c.JSON(http.StatusConflict, ErrorResponse{Error: e.Error(), Code: "IDEMPOTENCY_CONFLICT"})
	case *domain.ErrInvalidStateTransition:
		c.JSON(http.StatusConflict, ErrorResponse{Error: e.Error(), Code: "INVALID_STATE_TRANSITION"})
	case *domain.ErrOptimisticLock:
		c.JSON(http.StatusConflict, ErrorResponse{Error: e.Error(), Code: "OPTIMISTIC_LOCK_CONFLICT"})
	case *domain.ErrValidation:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: e.Error(), Code: "VALIDATION_ERROR"})
	case *domain.ErrMissingHeader:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: e.Error(), Code: "MISSING_HEADER"})
	case *domain.ErrClaimFailed:
		c.JSON(http.StatusNotFound, ErrorResponse{Error: e.Error(), Code: "NO_WORK_AVAILABLE"})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error", Code: "INTERNAL_ERROR"})
	}
}
