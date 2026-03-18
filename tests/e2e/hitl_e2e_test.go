package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type HITLRequestResponse struct {
	RequestID      string `json:"request_id"`
	SOPExecutionID string `json:"sop_execution_id"`
	SOPID          string `json:"sop_id"`
	StepID         string `json:"step_id"`
	StepName       string `json:"step_name"`
	Decision       string `json:"decision"`
	IsOverdue      bool   `json:"is_overdue"`
}

type HITLPaginatedResponse struct {
	Data       []HITLRequestResponse `json:"data"`
	TotalCount int                   `json:"total_count"`
}

func TestHITL_ListPending(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	resp := doRequest(t, "GET", "/api/v2/hitl/pending?limit=10", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result HITLPaginatedResponse
	decodeJSON(t, resp, &result)
	assert.GreaterOrEqual(t, result.TotalCount, 0)
}

func TestHITL_DecideApprove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// First, list pending to find a request
	resp := doRequest(t, "GET", "/api/v2/hitl/pending?limit=1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result HITLPaginatedResponse
	decodeJSON(t, resp, &result)

	if len(result.Data) == 0 {
		t.Skip("no pending HITL requests to test with")
	}

	requestID := result.Data[0].RequestID

	// Approve the request
	body, _ := json.Marshal(map[string]string{
		"decision":   "APPROVED",
		"decided_by": "e2e-test",
		"reason":     "automated e2e approval",
	})

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v2/hitl/%s/decide", requestID), body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var decided HITLRequestResponse
	decodeJSON(t, resp, &decided)
	assert.Equal(t, "APPROVED", decided.Decision)
}

func TestHITL_DecideInvalidDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	body, _ := json.Marshal(map[string]string{
		"decision":   "INVALID",
		"decided_by": "e2e-test",
		"reason":     "bad decision",
	})

	// Use a fake UUID — this should fail with either 404 or 400
	resp := doRequest(t, "POST", "/api/v2/hitl/00000000-0000-0000-0000-000000000099/decide", body)
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound)
}

func TestHITL_GetRequestNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	resp := doRequest(t, "GET", "/api/v2/hitl/00000000-0000-0000-0000-000000000099", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
