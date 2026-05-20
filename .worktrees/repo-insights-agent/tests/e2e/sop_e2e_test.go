package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiBaseURL = "http://localhost:8080"
	tenantID   = "00000000-0000-0000-0000-000000000001"
)

// SOPExecutionResponse matches the API v2 response shape.
type SOPExecutionResponse struct {
	SOPExecutionID     string  `json:"sop_execution_id"`
	SOPID              string  `json:"sop_id"`
	Status             string  `json:"status"`
	CurrentStep        string  `json:"current_step"`
	Industry           string  `json:"industry"`
	TemporalWorkflowID string  `json:"temporal_workflow_id"`
	Version            int     `json:"version"`
	CompletedAt        *string `json:"completed_at"`
}

type PaginatedSOPResponse struct {
	Data       []SOPExecutionResponse `json:"data"`
	TotalCount int                    `json:"total_count"`
}

func TestSOPExecution_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	sopIDs := []string{"FS-01", "INS-01", "HC-01", "HOSP-01", "LS-01", "MFG-01"}

	for _, sopID := range sopIDs {
		t.Run(sopID, func(t *testing.T) {
			// Start execution
			payload := map[string]interface{}{
				"test":      true,
				"sop_id":    sopID,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			body, _ := json.Marshal(map[string]interface{}{"payload": payload})

			resp := doRequest(t, "POST", fmt.Sprintf("/api/v2/sops/%s/execute", sopID), body)
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var exec SOPExecutionResponse
			decodeJSON(t, resp, &exec)

			assert.Equal(t, sopID, exec.SOPID)
			assert.NotEmpty(t, exec.SOPExecutionID)
			assert.Equal(t, "PENDING", exec.Status)

			// Get execution by ID
			resp = doRequest(t, "GET", fmt.Sprintf("/api/v2/sop-executions/%s", exec.SOPExecutionID), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var fetched SOPExecutionResponse
			decodeJSON(t, resp, &fetched)
			assert.Equal(t, exec.SOPExecutionID, fetched.SOPExecutionID)
		})
	}
}

func TestSOPExecution_ListWithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// List all
	resp := doRequest(t, "GET", "/api/v2/sop-executions?limit=10", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result PaginatedSOPResponse
	decodeJSON(t, resp, &result)
	assert.GreaterOrEqual(t, result.TotalCount, 0)

	// List with status filter
	resp = doRequest(t, "GET", "/api/v2/sop-executions?status=PENDING&limit=5", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// List with industry filter
	resp = doRequest(t, "GET", "/api/v2/sop-executions?industry=FINANCIAL_SERVICES&limit=5", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSOPExecution_InvalidSOP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	body, _ := json.Marshal(map[string]interface{}{"payload": map[string]string{"test": "true"}})
	resp := doRequest(t, "POST", "/api/v2/sops/NONEXISTENT-99/execute", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSOPExecution_InvalidPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	resp := doRequest(t, "POST", "/api/v2/sops/FS-01/execute", []byte(`{invalid json`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func doRequest(t *testing.T, method, path string, body []byte) *http.Response {
	t.Helper()
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, apiBaseURL+path, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, apiBaseURL+path, nil)
	}
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(v)
	require.NoError(t, err)
}
