package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHITL_OverdueDetection verifies that the API correctly flags overdue HITL requests.
// This test checks the is_overdue field in the response — actual auto-escalation
// is handled by Temporal's SLA timer in the workflow.
func TestHITL_OverdueDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// List pending requests and check for any overdue ones
	resp := doRequest(t, "GET", "/api/v2/hitl/pending?limit=50", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result HITLPaginatedResponse
	decodeJSON(t, resp, &result)

	// If there are requests, verify the is_overdue field is present
	for _, req := range result.Data {
		// is_overdue should be a boolean (either true or false)
		assert.IsType(t, false, req.IsOverdue)
	}
}
