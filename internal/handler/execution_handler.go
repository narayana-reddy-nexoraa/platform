package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/service"
)

// ExecutionHandler handles HTTP requests for the executions resource.
type ExecutionHandler struct {
	service *service.ExecutionService
}

// NewExecutionHandler creates a new handler instance.
func NewExecutionHandler(svc *service.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{service: svc}
}

// CreateExecution handles POST /api/v1/executions
func (h *ExecutionHandler) CreateExecution(c *gin.Context) {
	// Extract tenant ID from context (set by middleware)
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	// Extract idempotency key from header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		mapDomainError(c, &domain.ErrMissingHeader{Header: "Idempotency-Key"})
		return
	}

	// Bind JSON body
	var req domain.CreateExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body: " + err.Error(),
			Code:  "INVALID_REQUEST",
		})
		return
	}

	// Call service
	exec, isNew, err := h.service.CreateExecution(c.Request.Context(), tenantID, idempotencyKey, req)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	response := exec.ToResponse()
	if isNew {
		c.JSON(http.StatusCreated, response)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// GetExecution handles GET /api/v1/executions/:id
func (h *ExecutionHandler) GetExecution(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid execution ID format",
			Code:  "INVALID_ID",
		})
		return
	}

	exec, err := h.service.GetExecution(c.Request.Context(), executionID, tenantID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, exec.ToResponse())
}

// ListExecutions handles GET /api/v1/executions
func (h *ExecutionHandler) ListExecutions(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	// Parse query parameters
	limit := int32(20)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.ParseInt(o, 10, 32); err == nil {
			offset = int32(parsed)
		}
	}

	var status *domain.ExecutionStatus
	if s := c.Query("status"); s != "" {
		if domain.IsValidStatus(s) {
			st := domain.ExecutionStatus(s)
			status = &st
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "invalid status filter: " + s,
				Code:  "INVALID_STATUS",
			})
			return
		}
	}

	result, err := h.service.ListExecutions(c.Request.Context(), tenantID, status, limit, offset)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}
