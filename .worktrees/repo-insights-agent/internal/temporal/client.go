package temporal

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
)

// NewClient creates a Temporal client connected to the specified host and namespace.
func NewClient(host, namespace string, logger zerolog.Logger) (client.Client, error) {
	opts := client.Options{
		HostPort:  host,
		Namespace: namespace,
		Logger:    newZerologAdapter(logger),
	}

	c, err := client.Dial(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	return c, nil
}
