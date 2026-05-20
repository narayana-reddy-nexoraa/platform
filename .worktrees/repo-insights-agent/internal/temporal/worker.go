package temporal

import (
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/narayana-platform/execution-engine/internal/temporal/activities"
	"github.com/narayana-platform/execution-engine/internal/temporal/workflows"
)

// TaskQueues defines the Temporal task queues per industry.
var TaskQueues = []string{
	"financial-services-tasks",
	"insurance-tasks",
	"healthcare-tasks",
	"hospital-ops-tasks",
	"life-sciences-tasks",
	"manufacturing-tasks",
}

// StartWorkers creates and starts a Temporal worker for each industry task queue.
// Returns a slice of workers that must be stopped on shutdown.
func StartWorkers(c client.Client, acts *activities.Activities, logger zerolog.Logger) []worker.Worker {
	var workers []worker.Worker

	for _, tq := range TaskQueues {
		w := worker.New(c, tq, worker.Options{
			MaxConcurrentActivityExecutionSize:     10,
			MaxConcurrentWorkflowTaskExecutionSize: 10,
		})

		// Register workflows
		w.RegisterWorkflow(workflows.SOPWorkflow)
		w.RegisterWorkflow(workflows.BridgeWorkflow)

		// Register all shared activities
		w.RegisterActivity(acts.Intake)
		w.RegisterActivity(acts.DataRetrieval)
		w.RegisterActivity(acts.Classification)
		w.RegisterActivity(acts.Decisioning)
		w.RegisterActivity(acts.CreateHITLRequest)
		w.RegisterActivity(acts.Execution)
		w.RegisterActivity(acts.Audit)

		logger.Info().Str("task_queue", tq).Msg("registered Temporal worker")
		workers = append(workers, w)
	}

	return workers
}
