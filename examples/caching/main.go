package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/middleware"
)

// Custom data structure for workflow input/output
type flow struct {
	Data   []byte `json:"data"`
	Rating int    `json:"rating"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Initialize orchestrator with logging
	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
	)
	o.Use(middleware.Logging)

	// Initialize Snapshotter
	snapshotter := orchid.NewShapshotter()

	// Register activities
	o.RegisterActivity("fetchData", orchid.TypedActivity(fetchData))     // Fetch data initially
	o.RegisterActivity("checkpoint", snapshotter.Save)                   // Snapshot activity to save data
	o.RegisterActivity("processData", orchid.TypedActivity(processData)) // Process data, fail first time, succeed on retry
	o.RegisterActivity("snapshotDelete", snapshotter.Delete)             // Snapshot activity to save data
	o.RegisterActivity("complete", orchid.TypedActivity(complete))       // Exit activity

	// Define the workflow
	wf := orchid.NewWorkflow("snapshot_workflow").
		AddNewNode("fetchData").
		ThenNewNode("checkpoint").
		ThenNewNode("processData").
		Link("processData", "checkpoint"). // optional link to route back to checkpoint
		ThenNewNode("snapshotDelete").
		ThenNewNode("complete")

	// Load and start the workflow
	o.LoadWorkflow(wf)
	workflowID := uuid.New().String()
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	in, _ := json.Marshal(flow{})
	out, err := o.Start(ctx, in)
	if err != nil {
		fmt.Printf("Workflow execution failed: %v\n", err)
		return
	}

	// Print final workflow output
	var finalOutput flow
	if err := json.Unmarshal(out, &finalOutput); err == nil {
		fmt.Printf("Final Workflow Output - Data: %s, Rating: %d\n", finalOutput.Data, finalOutput.Rating)
	} else {
		fmt.Println("Failed to unmarshal final output:", err)
	}
}

func fetchData(ctx context.Context, input *flow) (*flow, error) {
	fmt.Println("Fetching data from external source...")
	return &flow{
		Data:   []byte("initial_data"),
		Rating: 10,
	}, nil
}

var visited bool

func processData(ctx context.Context, input *flow) (*flow, error) {
	if !visited {
		// First attempt: simulate a failure and route back to snapshot
		fmt.Println("Processing failed, will retry with cached data.")
		visited = true
		return nil, orchid.RouteTo("checkpoint")
	}

	// on retry
	fmt.Println("Processing data successfully:", string(input.Data))
	return &flow{
		Data:   []byte("processed_data"),
		Rating: input.Rating + 50,
	}, orchid.RouteTo("snapshotDelete")
}

func complete(ctx context.Context, input *flow) (*flow, error) {
	fmt.Println("Workflow completed successfully.")
	return input, nil
}
