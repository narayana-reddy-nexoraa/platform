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
	tenantA = "00000000-0000-0000-0000-00000000000a"
	tenantB = "00000000-0000-0000-0000-00000000000b"
)

func TestMultiTenant_Isolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Create execution for tenant A
	payload := map[string]interface{}{
		"payload": map[string]interface{}{
			"test":      true,
			"tenant":    "A",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
	body, _ := json.Marshal(payload)

	reqA, err := http.NewRequest("POST", apiBaseURL+"/api/v2/sops/FS-01/execute", bytes.NewReader(body))
	require.NoError(t, err)
	reqA.Header.Set("Content-Type", "application/json")
	reqA.Header.Set("X-Tenant-ID", tenantA)

	client := &http.Client{Timeout: 30 * time.Second}
	respA, err := client.Do(reqA)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, respA.StatusCode)

	var execA SOPExecutionResponse
	decodeJSON(t, respA, &execA)

	// Tenant B should NOT be able to see tenant A's execution
	reqB, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v2/sop-executions/%s", apiBaseURL, execA.SOPExecutionID), nil)
	require.NoError(t, err)
	reqB.Header.Set("X-Tenant-ID", tenantB)

	respB, err := client.Do(reqB)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, respB.StatusCode, "tenant B should not see tenant A's execution")
	respB.Body.Close()
}

func TestMultiTenant_MissingHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	req, _ := http.NewRequest("GET", apiBaseURL+"/api/v2/sop-executions", nil)
	// Intentionally omit X-Tenant-ID header

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject requests without tenant header")
}
