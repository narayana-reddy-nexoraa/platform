package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/handler"
	"github.com/narayana-platform/execution-engine/internal/middleware"
)

// ---------------------------------------------------------------------------
// Mock service
// ---------------------------------------------------------------------------

type mockService struct {
	createResult *domain.Execution
	createIsNew  bool
	createErr    error

	getResult *domain.Execution
	getErr    error

	listResult *domain.PaginatedResponse
	listErr    error
}

func (m *mockService) CreateExecution(_ context.Context, _ uuid.UUID, _ string, _ domain.CreateExecutionRequest) (*domain.Execution, bool, error) {
	return m.createResult, m.createIsNew, m.createErr
}

func (m *mockService) GetExecution(_ context.Context, _, _ uuid.UUID) (*domain.Execution, error) {
	return m.getResult, m.getErr
}

func (m *mockService) ListExecutions(_ context.Context, _ uuid.UUID, _ *domain.ExecutionStatus, _, _ int32) (*domain.PaginatedResponse, error) {
	return m.listResult, m.listErr
}

// ---------------------------------------------------------------------------
// Mock DB pinger (for health endpoint)
// ---------------------------------------------------------------------------

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(_ context.Context) error {
	return m.err
}

// ---------------------------------------------------------------------------
// Test router setup — mirrors cmd/api/main.go route registration
// ---------------------------------------------------------------------------

func setupRouter(svc handler.ExecutionServiceInterface) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Global middleware (same as main.go minus RequestLogger which needs a real logger)
	r.Use(middleware.CorrelationID())

	// Health endpoints (with a mock pinger)
	healthH := handler.NewHealthHandler(&mockPinger{})
	healthH.RegisterRoutes(r)

	// API v1 routes with tenant extraction
	h := handler.NewExecutionHandler(svc)
	v1 := r.Group("/api/v1")
	v1.Use(middleware.TenantExtractor())
	{
		v1.POST("/executions", h.CreateExecution)
		v1.GET("/executions/:id", h.GetExecution)
		v1.GET("/executions", h.ListExecutions)
	}

	return r
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestExecution(tenantID uuid.UUID) *domain.Execution {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return &domain.Execution{
		ExecutionID:    uuid.New(),
		TenantID:       tenantID,
		IdempotencyKey: "test-key-1",
		Status:         domain.StatusCreated,
		AttemptCount:   0,
		MaxAttempts:    3,
		Payload:        json.RawMessage(`{"url":"https://example.com"}`),
		PayloadHash:    "abc123",
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
	}
}

var testTenantID = uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

func validCreateBody() []byte {
	b, _ := json.Marshal(domain.CreateExecutionRequest{
		Payload: json.RawMessage(`{"url":"https://example.com"}`),
	})
	return b
}

// ---------------------------------------------------------------------------
// 1. POST /api/v1/executions — 201 Created
// ---------------------------------------------------------------------------

func TestCreateExecution_201Created(t *testing.T) {
	exec := newTestExecution(testTenantID)
	svc := &mockService{createResult: exec, createIsNew: true}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", testTenantID.String())
	req.Header.Set("Idempotency-Key", "idem-1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var body domain.ExecutionResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, body.ExecutionID)
	assert.Equal(t, exec.TenantID, body.TenantID)
	assert.Equal(t, domain.StatusCreated, body.Status)
	assert.Equal(t, int32(3), body.MaxAttempts)

	// Verify correlation ID is returned in response header
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

// ---------------------------------------------------------------------------
// 2. POST /api/v1/executions — 200 OK (idempotent replay)
// ---------------------------------------------------------------------------

func TestCreateExecution_200IdempotentReplay(t *testing.T) {
	exec := newTestExecution(testTenantID)
	svc := &mockService{createResult: exec, createIsNew: false}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", testTenantID.String())
	req.Header.Set("Idempotency-Key", "idem-1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body domain.ExecutionResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, body.ExecutionID)
}

// ---------------------------------------------------------------------------
// 3. POST /api/v1/executions — 400 missing Idempotency-Key
// ---------------------------------------------------------------------------

func TestCreateExecution_400MissingIdempotencyKey(t *testing.T) {
	svc := &mockService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", testTenantID.String())
	// No Idempotency-Key header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "MISSING_HEADER", body.Code)
	assert.Contains(t, body.Error, "Idempotency-Key")
}

// ---------------------------------------------------------------------------
// 4. POST /api/v1/executions — 400 missing X-Tenant-ID
// ---------------------------------------------------------------------------

func TestCreateExecution_400MissingTenantID(t *testing.T) {
	svc := &mockService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-1")
	// No X-Tenant-ID header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "MISSING_HEADER", body.Code)
	assert.Contains(t, body.Error, "X-Tenant-ID")
}

// ---------------------------------------------------------------------------
// 5. POST /api/v1/executions — 400 invalid JSON
// ---------------------------------------------------------------------------

func TestCreateExecution_400InvalidJSON(t *testing.T) {
	svc := &mockService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader([]byte(`{not json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", testTenantID.String())
	req.Header.Set("Idempotency-Key", "idem-1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", body.Code)
	assert.Contains(t, body.Error, "invalid request body")
}

// ---------------------------------------------------------------------------
// 6. POST /api/v1/executions — 409 idempotency conflict
// ---------------------------------------------------------------------------

func TestCreateExecution_409IdempotencyConflict(t *testing.T) {
	svc := &mockService{
		createErr: &domain.ErrIdempotencyConflict{IdempotencyKey: "idem-1"},
	}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions", bytes.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", testTenantID.String())
	req.Header.Set("Idempotency-Key", "idem-1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "IDEMPOTENCY_CONFLICT", body.Code)
	assert.Contains(t, body.Error, "idem-1")
}

// ---------------------------------------------------------------------------
// 7. GET /api/v1/executions/:id — 200 OK
// ---------------------------------------------------------------------------

func TestGetExecution_200OK(t *testing.T) {
	exec := newTestExecution(testTenantID)
	svc := &mockService{getResult: exec}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+exec.ExecutionID.String(), nil)
	req.Header.Set("X-Tenant-ID", testTenantID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body domain.ExecutionResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, body.ExecutionID)
	assert.Equal(t, exec.TenantID, body.TenantID)
	assert.Equal(t, domain.StatusCreated, body.Status)
	assert.JSONEq(t, `{"url":"https://example.com"}`, string(body.Payload))
}

// ---------------------------------------------------------------------------
// 8. GET /api/v1/executions/:id — 404 not found
// ---------------------------------------------------------------------------

func TestGetExecution_404NotFound(t *testing.T) {
	missingID := uuid.New()
	svc := &mockService{
		getErr: &domain.ErrNotFound{Entity: "execution", ID: missingID.String()},
	}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+missingID.String(), nil)
	req.Header.Set("X-Tenant-ID", testTenantID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", body.Code)
	assert.Contains(t, body.Error, "execution not found")
}

// ---------------------------------------------------------------------------
// 9. GET /api/v1/executions/:id — 400 invalid UUID
// ---------------------------------------------------------------------------

func TestGetExecution_400InvalidUUID(t *testing.T) {
	svc := &mockService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", testTenantID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body handler.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_ID", body.Code)
	assert.Contains(t, body.Error, "invalid execution ID format")
}

// ---------------------------------------------------------------------------
// 10. GET /api/v1/executions — 200 with pagination
// ---------------------------------------------------------------------------

func TestListExecutions_200WithPagination(t *testing.T) {
	exec1 := newTestExecution(testTenantID)
	exec2 := newTestExecution(testTenantID)

	svc := &mockService{
		listResult: &domain.PaginatedResponse{
			Data:       []domain.ExecutionResponse{exec1.ToResponse(), exec2.ToResponse()},
			TotalCount: 42,
			Limit:      10,
			Offset:     0,
		},
	}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=10&offset=0", nil)
	req.Header.Set("X-Tenant-ID", testTenantID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body domain.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, int64(42), body.TotalCount)
	assert.Equal(t, int32(10), body.Limit)
	assert.Equal(t, int32(0), body.Offset)
	assert.Len(t, body.Data, 2)
	assert.Equal(t, exec1.ExecutionID, body.Data[0].ExecutionID)
	assert.Equal(t, exec2.ExecutionID, body.Data[1].ExecutionID)
}

// ---------------------------------------------------------------------------
// 11. GET /health/live — 200
// ---------------------------------------------------------------------------

func TestHealthLive_200(t *testing.T) {
	svc := &mockService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "up", body["status"])
}
