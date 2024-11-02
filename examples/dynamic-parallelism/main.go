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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kyodo-tech/orchid"
)

type Flow struct {
	Items   []string
	Results []string
}

func startChildWorkflows(registry orchid.OrchestratorRegistry) func(ctx context.Context, data *Flow) (*Flow, error) {
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
					Items: []string{item},
				}

				in, err := json.Marshal(childData)
				if err != nil {
					errorsCh <- fmt.Errorf("failed to encode child data: %w", err)
					return
				}

				childWorkflowID := uuid.New().String()
				cctx := orchid.WithWorkflowID(ctx, childWorkflowID)
				cctx = orchid.WithParentWorkflowID(cctx, parentWorkflowID)

				fmt.Printf("Starting child workflow '%s'\n", childWorkflowID)
				output, err := co.Start(cctx, in)
				if err != nil {
					errorsCh <- fmt.Errorf("child workflow failed: %w", err)
					return
				}
				fmt.Printf("Child workflow '%s' completed\n", childWorkflowID)

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

func processItem(ctx context.Context, data *Flow) (*Flow, error) {
	item := data.Items[0]
	fmt.Printf("Processing item: %s\n", item)
	time.Sleep(1 * time.Second) // Simulate some processing
	result := fmt.Sprintf("processing %s", item)
	data.Results = []string{result}
	return data, nil
}

func main() {
	ctx := context.Background()

	// Create the child orchestrator
	co := orchid.NewOrchestrator()
	childWorkflow := orchid.NewWorkflow("childWorkflow")
	childWorkflow.AddNewNode("processItem")
	co.RegisterActivity("processItem", orchid.TypedActivity(processItem))
	co.LoadWorkflow(childWorkflow)

	// Create the orchestrator registry and register the child orchestrator
	orchestratorRegistry := orchid.NewOrchestratorRegistry()
	orchestratorRegistry.Set("childOrchestrator", co)

	// Create the main orchestrator
	o := orchid.NewOrchestrator()
	mainWorkflow := orchid.NewWorkflow("mainWorkflow")
	mainWorkflow.AddNode(
		orchid.NewNode("startChildWorkflows",
			orchid.WithNodeConfig(map[string]interface{}{
				"child-orchestrator": "childOrchestrator",
				"max-concurrency":    3, // Adjust concurrency limit here
			}),
		),
	)

	o.RegisterActivity("startChildWorkflows", orchid.TypedActivity(startChildWorkflows(orchestratorRegistry)))
	o.LoadWorkflow(mainWorkflow)

	// Create initial data
	data := &Flow{
		Items: []string{"item1", "item2", "item3", "item4", "item5"},
	}

	in, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Failed to encode input data:", err)
		os.Exit(1)
	}

	// Start the main workflow
	ctx = orchid.WithWorkflowID(ctx, "main")
	output, err := o.Start(ctx, in)
	if err != nil {
		fmt.Println("Workflow failed:", err)
		os.Exit(1)
	}

	// Decode the output
	var resultData Flow
	if err := json.Unmarshal(output, &resultData); err != nil {
		fmt.Println("Failed to decode output data:", err)
		os.Exit(1)
	}

	fmt.Println("Workflow completed. Results:")
	for _, res := range resultData.Results {
		fmt.Println(res)
	}
}
