// Copyright 2024 Kyodo Tech合同会社
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchid_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	orchid "github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/persistence"
	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_WorkflowSimple(t *testing.T) {
	o := orchid.NewOrchestrator()
	ctx := context.Background()
	type testkey string
	ctx = context.WithValue(ctx, testkey("testkey"), "testvalue")

	// Register activities
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), nil
	})

	o.RegisterActivity("B", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'B'), nil
	})

	o.RegisterActivity("C", func(ctx context.Context, input []byte) ([]byte, error) {
		ctxTestValue := ctx.Value(testkey("testkey"))
		assert.Equal(t, "testvalue", ctxTestValue)
		return append(input, 'C'), nil
	})

	var err error

	// Create a workflow
	wf := orchid.NewWorkflow("test_workflow")
	wf.AddNode(orchid.NewNode("A")).
		Then(orchid.NewNode("B")).
		Then(orchid.NewNode("C"))

	// Load the workflow into the orchestrator
	o.LoadWorkflow(wf)

	// Start the workflow
	workflowID := uuid.New().String()
	ctx = orchid.WithWorkflowID(ctx, workflowID)

	out, err := o.Start(ctx, []byte{})
	if err != nil {
		t.Logf("Start returned error: %v", err)
	}

	assert.Equal(t, "ABC", string(out))
}

func TestOrchestrator_UniqueWorkflowIDs(t *testing.T) {
	// Create an orchestrator with the in-memory persister
	persister, err := persistence.NewSQLitePersister(":memory:")
	if err != nil {
		t.Fatalf("Failed to create SQLite persister: %v", err)
	}
	defer persister.DB.Close()

	// Create an orchestrator with the in-memory persister
	o := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)

	// Register activities
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), nil
	})

	wf := orchid.NewWorkflow("test_workflow")
	wf.AddNode(orchid.NewNode("A"))

	o.LoadWorkflow(wf)

	// Start the workflow
	workflowID := uuid.New().String()

	ctx := context.Background()
	ctx = orchid.WithWorkflowID(ctx, workflowID)

	out, err := o.Start(ctx, []byte{})
	assert.NoError(t, err)
	assert.Equal(t, "A", string(out))

	// Start the workflow again with the same ID
	_, err = o.Start(ctx, []byte{})
	assert.Error(t, err, persistence.ErrWorkflowIDExists.Error())

	// Start the workflow again with a different ID
	workflowID2 := uuid.New().String()
	ctx = orchid.WithWorkflowID(ctx, workflowID2)

	out, err = o.Start(ctx, []byte{})
	assert.NoError(t, err)
	assert.Equal(t, "A", string(out))
}

func TestOrchestrator_WorkflowsWithRouting(t *testing.T) {
	o := orchid.NewOrchestrator()

	// Register activities
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), orchid.RouteTo("C")
	})

	o.RegisterActivity("B", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'B'), nil
	})

	o.RegisterActivity("C", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'C'), nil
	})

	o.RegisterActivity("D", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'D'), nil
	})

	// Create a workflow
	wf := orchid.NewWorkflow("test_workflow")
	wf.AddNode(orchid.NewNode("A"))
	wf.AddNode(orchid.NewNode("B"))
	wf.AddNode(orchid.NewNode("C"))
	wf.AddNode(orchid.NewNode("D"))

	wf.Link("A", "B")
	wf.Link("A", "C")
	wf.Link("C", "D")

	// Load the workflow into the orchestrator
	o.LoadWorkflow(wf)

	// Start the workflow
	workflowID := uuid.New().String()
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	out, err := o.Start(ctx, []byte{'Z'})
	if err != nil {
		t.Logf("Start returned error: %v", err)
	}

	assert.Equal(t, "ZACD", string(out))

	// Replace A activity with a new one that routes to B
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), orchid.RouteTo("B")
	})

	// Rerun the workflow
	out, err = o.Start(ctx, []byte{'Z'})
	if err != nil {
		t.Logf("Start returned error on rerun: %v", err)
	}

	assert.Equal(t, "ZAB", string(out))
}

func TestOrchestrator_RestoreWorkflowAfterPanic(t *testing.T) {
	persister, err := persistence.NewSQLitePersister(":memory:")
	if err != nil {
		t.Fatalf("Failed to create SQLite persister: %v", err)
	}
	defer persister.DB.Close()

	// Create an orchestrator with the in-memory persister
	o := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)

	// Register activities
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), nil
	})

	nodeB := orchid.NewNode("B", orchid.WithNodeRetryPolicy(&orchid.RetryPolicy{
		MaxRetries:         3,
		InitInterval:       100 * time.Millisecond,
		MaxInterval:        500 * time.Millisecond,
		BackoffCoefficient: 1.0,
	}))

	o.RegisterActivity("B", func(ctx context.Context, input []byte) ([]byte, error) {
		panic("simulated failure in activity B")
	})

	o.RegisterActivity("C", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'C'), nil
	})

	// Create a workflow
	wf := orchid.NewWorkflow("test_workflow")
	wf.AddNode(orchid.NewNode("A"))
	wf.AddNode(nodeB)
	wf.AddNode(orchid.NewNode("C"))

	wf.Link("A", "B")
	wf.Link("B", "C")

	// Load the workflow into the orchestrator
	o.LoadWorkflow(wf)

	// Start the workflow
	workflowID := uuid.New().String()
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	// Run the workflow and expect an error instead of a panic
	_, err = o.Start(ctx, []byte{})
	assert.Error(t, err, "Expected error in Start due to panic in activity B")
	assert.Contains(t, err.Error(), "panic occurred", "Error should indicate a panic occurred in activity B")

	// Verify that the orchestrator logged the error and updated the workflow state
	err = verifyState(t, persister, 4, workflowID, "B")
	assert.NoError(t, err)

	err = verifyOpenWorkflowCount(t, persister, "test_workflow", 1)
	assert.NoError(t, err)

	// Now, simulate a restart by creating a new orchestrator
	o2 := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)

	// Register activities with the new orchestrator
	o2.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), nil
	})

	o2.RegisterActivity("B", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'B'), nil
	})

	o2.RegisterActivity("C", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'C'), nil
	})

	// Load the workflow into the new orchestrator
	o2.LoadWorkflow(wf)

	restorable, err := o2.RestorableWorkflows(ctx)
	assert.NoError(t, err)

	var out []byte
	for _, workflows := range restorable {
		for _, restorable := range workflows {
			out, err = restorable.Entrypoint(ctx)
			assert.NoError(t, err)
		}
	}

	assert.Equal(t, "ABC", string(out))

	err = verifyState(t, persister, 8, workflowID, "C")
	assert.NoError(t, err)

	err = verifyOpenWorkflowCount(t, persister, "test_workflow", 0)
	assert.NoError(t, err)
}

func TestOrchestrator_RestoreWorkflowAfterPanicWithRouting(t *testing.T) {
	persister, err := persistence.NewSQLitePersister(":memory:")
	if err != nil {
		t.Fatalf("Failed to create SQLite persister: %v", err)
	}
	defer persister.DB.Close()

	// Create an orchestrator with the in-memory persister
	o := orchid.NewOrchestrator(
		orchid.WithPersistence(persister),
	)

	// Register activities
	o.RegisterActivity("A", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'A'), orchid.RouteTo("C")
	})

	o.RegisterActivity("B", func(ctx context.Context, input []byte) ([]byte, error) {
		panic("this workflow should never reach node B")
	})

	o.RegisterActivity("C", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'C'), nil
	})

	o.RegisterActivity("D", func(ctx context.Context, input []byte) ([]byte, error) {
		panic("initial failure in activity D")
	})

	// Create a workflow
	wf := orchid.NewWorkflow("test_workflow")
	wf.AddNode(orchid.NewNode("A"))
	wf.AddNode(orchid.NewNode("B"))
	wf.AddNode(orchid.NewNode("C"))
	wf.AddNode(orchid.NewNode("D", orchid.WithNodeRetryPolicy(&orchid.RetryPolicy{
		InitInterval:       100 * time.Millisecond,
		MaxInterval:        500 * time.Millisecond,
		BackoffCoefficient: 1.0,
	})))

	wf.Link("A", "B")
	wf.Link("A", "C")
	wf.Link("C", "D")

	// Load the workflow into the orchestrator
	o.LoadWorkflow(wf)

	// Start the workflow
	workflowID := uuid.New().String()
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	_, err = o.Start(ctx, []byte{})
	assert.Error(t, err, "Expected error in Start due to panic in activity B")

	// 2x successful nodes + 1 failure
	err = verifyState(t, persister, 6, workflowID, "D")
	assert.NoError(t, err)

	err = verifyOpenWorkflowCount(t, persister, "test_workflow", 1)
	assert.NoError(t, err)

	// Replace D activity with one that completes
	o.RegisterActivity("D", func(ctx context.Context, input []byte) ([]byte, error) {
		return append(input, 'D'), nil
	})

	restorable, err := o.RestorableWorkflows(ctx)
	assert.NoError(t, err)

	var out []byte
	for _, workflows := range restorable {
		for _, restorable := range workflows {
			out, err = restorable.Entrypoint(ctx)
			assert.NoError(t, err)
		}
	}

	assert.Equal(t, "ACD", string(out))

	// 5+ 2x2 open complete on D and
	err = verifyState(t, persister, 10, workflowID, "D")
	assert.NoError(t, err)

	err = verifyOpenWorkflowCount(t, persister, "test_workflow", 0)
	assert.NoError(t, err)
}

func verifyState(t *testing.T, persister persistence.Persister, nsteps int, workflowID, lastActivityName string) error {
	// Verify that the workflow completed successfully
	steps, err := persister.LoadWorkflowSteps(context.Background(), workflowID)
	assert.NoError(t, err)
	if !assert.Equal(t, nsteps, len(steps), fmt.Sprintf("Expected at least %v steps in the workflow log", nsteps)) {
		return fmt.Errorf("Expected at least %v steps in the workflow log", nsteps)
	}

	// Check that the final step is activity "C" and completed successfully
	lastStep := steps[len(steps)-1]
	assert.Equal(t, lastActivityName, lastStep.ActivityName)
	// assert.Equal(t, persistence.StateCompleted, lastStep.ActivityState)

	return nil
}

func verifyOpenWorkflowCount(t *testing.T, persister persistence.Persister, workflowName string, expectedCount int) error {
	// Verify the workflow status is now completed
	workflows, err := persister.LoadOpenWorkflows(context.Background(), workflowName)
	assert.NoError(t, err)
	if !assert.Len(t, workflows, expectedCount) {
		return fmt.Errorf("Expected %v open workflows, got %v", expectedCount, len(workflows))
	}

	return nil
}
