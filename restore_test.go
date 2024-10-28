package orchid_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/persistence"
	"github.com/stretchr/testify/assert"
)

// Define the Flow struct
type Flow struct {
	Items         []string
	Results       []string
	SimulatePanic bool
}

// Activity to process an item
func processItem(ctx context.Context, data *Flow) (*Flow, error) {
	if data.SimulatePanic {
		panic(fmt.Sprintf("simulated failure with %s", data.Items[0]))
	}
	item := data.Items[0]
	// fmt.Printf("Processing item: %s\n", item)
	// Simulate some processing
	// time.Sleep(1 * time.Second)
	result := fmt.Sprintf("processed %s", item)
	data.Results = []string{result}
	return data, nil
}

// Activity to start child workflows
func startChildWorkflows(registry orchid.OrchestratorRegistry, failAt int) func(ctx context.Context, data *Flow) (*Flow, error) {
	return func(ctx context.Context, data *Flow) (*Flow, error) {
		parentWorkflowID := orchid.WorkflowID(ctx)

		orchestratorName, ok := orchid.ConfigString(ctx, "child-orchestrator")
		if !ok {
			return nil, fmt.Errorf("failed to get child orchestrator name")
		}

		co, err := registry.Get(orchestratorName)
		if err != nil {
			return nil, fmt.Errorf("failed to get child orchestrator: %w", err)
		}

		// Get maxConcurrency from config or set default
		maxConcurrency := 1
		if v, ok := orchid.ConfigInt(ctx, "max-concurrency"); ok {
			maxConcurrency = v
		}

		items := data.Items
		outputs := make([]string, len(items))
		errorsCh := make(chan error, len(items))

		semaphore := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup

		for i, item := range items {
			semaphore <- struct{}{} // Acquire a slot
			wg.Add(1)
			go func(i int, item string) {
				defer wg.Done()
				defer func() { <-semaphore }() // Release the slot when done

				// Create input data for child workflow
				childData := &Flow{
					Items:         []string{item},
					SimulatePanic: i == failAt,
				}

				in, err := json.Marshal(childData)
				if err != nil {
					errorsCh <- fmt.Errorf("failed to encode child data: %w", err)
					return
				}

				childWorkflowID := fmt.Sprintf("%s-child-%d", parentWorkflowID, i)
				cctx := orchid.WithWorkflowID(ctx, childWorkflowID)
				cctx = orchid.WithParentWorkflowID(cctx, parentWorkflowID)

				// fmt.Printf("Starting child workflow '%s'\n", childWorkflowID)
				output, err := co.Start(cctx, in)
				if err != nil {
					errorsCh <- fmt.Errorf("child workflow failed: %w", err)
					return
				}
				// fmt.Printf("Child workflow '%s' completed\n", childWorkflowID)

				var childOutput Flow
				if err := json.Unmarshal(output, &childOutput); err != nil {
					errorsCh <- fmt.Errorf("failed to decode child output: %w", err)
					return
				}

				outputs[i] = childOutput.Results[0]
			}(i, item)
		}

		// Wait for all child workflows to complete
		wg.Wait()

		close(errorsCh)
		if len(errorsCh) > 0 {
			return nil, <-errorsCh // Return the first error encountered
		}

		data.Results = outputs

		return data, nil
	}
}

func TestOrchestrator_DynamicParallelismWithChildWorkflowsRestoration(t *testing.T) {
	persister, err := persistence.NewSQLitePersister(":memory:")
	if err != nil {
		t.Fatalf("Failed to create SQLite persister: %v", err)
	}
	defer persister.DB.Close()

	// Create the child orchestrator
	co := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)
	childWorkflow := orchid.NewWorkflow("childWorkflow")
	childWorkflow.AddNewNode("processItem")
	co.RegisterActivity("processItem", orchid.TypedActivity(processItem))
	co.LoadWorkflow(childWorkflow)

	// Create the orchestrator registry and register the child orchestrator
	orchestratorRegistry := orchid.NewOrchestratorRegistry()
	orchestratorRegistry.Set("childOrchestrator", co)

	// Create the main orchestrator
	o := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)
	mainWorkflow := orchid.NewWorkflow("mainWorkflow")
	mainWorkflow.AddNode(
		orchid.NewNode("startChildWorkflows",
			orchid.WithNodeConfig(map[string]interface{}{
				"child-orchestrator": "childOrchestrator",
				"max-concurrency":    3, // Adjust concurrency limit here
			}),
		),
	)

	o.RegisterActivity("startChildWorkflows", orchid.TypedActivity(startChildWorkflows(orchestratorRegistry, 2))) // Fail at child index 2
	o.LoadWorkflow(mainWorkflow)

	// Create initial data
	data := &Flow{
		Items: []string{"item0", "item1", "item2", "item3", "item4", "item5"},
	}

	in, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to encode input data: %v", err)
	}

	// Start the main workflow
	workflowID := "main"
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	// Run the workflow and expect an error
	_, err = o.Start(ctx, in)
	assert.Error(t, err, "Expected error in Start due to simulated failure in child workflow")
	assert.Contains(t, err.Error(), "simulated failure with item2", "Error should indicate simulated failure")

	// Verify that the main workflow is still open
	err = verifyOpenWorkflowCount(t, persister, "mainWorkflow", 1)
	assert.NoError(t, err)

	// Now, simulate a restart by creating a new orchestrator
	o2 := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)
	o2.RegisterActivity("startChildWorkflows", orchid.TypedActivity(startChildWorkflows(orchestratorRegistry, -1))) // No failure this time
	o2.LoadWorkflow(mainWorkflow)

	// Restore the main workflow
	restorable, err := o2.RestorableWorkflows(ctx)
	assert.NoError(t, err)

	for _, workflows := range restorable {
		for _, restorable := range workflows {
			_, err = restorable.Entrypoint(ctx)
			assert.NoError(t, err)
		}
	}

	// Verify that the main workflow is now completed
	err = verifyOpenWorkflowCount(t, persister, "mainWorkflow", 0)
	assert.NoError(t, err)

	// Verify that child workflows are restored properly
	// Create a new context for child workflows restoration
	ctxChild := context.Background()

	// Restore child workflows
	restorableChildren, err := co.RestorableWorkflows(ctxChild)
	assert.NoError(t, err)

	for _, workflows := range restorableChildren {
		for _, restorable := range workflows {
			_, err = restorable.Entrypoint(ctxChild)
			assert.NoError(t, err)
		}
	}

	// Verify that there are no open child workflows
	err = verifyOpenWorkflowCount(t, persister, "childWorkflow", 0)
	assert.NoError(t, err)
}
