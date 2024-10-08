package main

import (
	"context"
	"fmt"
	"os"

	orchid "github.com/kyodo-tech/orchid"
)

const workflowName = "fbp.json"

func PrintAndSend(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("received: '%s' sending: '%s'\n", string(input), text)
		return []byte(text), nil
	}
}

// StartChildWorkflowActivity starts a child workflow.
func StartChildWorkflowActivity(ctx context.Context, childWorkflowName []byte) ([]byte, error) {
	// Convert childWorkflowName to string to identify the workflow.
	childName := string(childWorkflowName)

	// Get the child orchestrator from the context.
	childOrchestrator, ok := orchid.Config(ctx, childName)
	if !ok {
		return nil, fmt.Errorf("failed to get child orchestrator '%s'", childName)
	}

	o, ok := childOrchestrator.(*orchid.Orchestrator)
	if !ok {
		return nil, fmt.Errorf("failed to cast child orchestrator '%s' to *orchid.Orchestrator", childName)
	}

	// Start the child workflow. Consider if you want to start it synchronously or asynchronously.
	ctx1 := orchid.WithWorkflowID(ctx, "child-workflow")
	output, err := o.Start(ctx1, []byte("parent-spawning-child"))
	if err != nil {
		return nil, fmt.Errorf("failed to start child workflow '%s': %w", childName, err)
	}

	return output, nil
}

func main() {
	// Initialize the main orchestrator
	o := orchid.NewOrchestrator()
	o.RegisterActivity("A", PrintAndSend("B"))
	o.RegisterActivity("B", PrintAndSend("child-orchestrator-selector"))
	o.RegisterActivity("child-workflow-starter", StartChildWorkflowActivity)

	wf := orchid.NewWorkflow(workflowName)
	wf.AddNode(orchid.NewNode("A"))
	wf.AddNode(orchid.NewNode("B"))

	// Define the child workflow
	co := orchid.NewOrchestrator()
	co.RegisterActivity("A", PrintAndSend("hello from child"))

	cwf := orchid.NewWorkflow("child")
	cwf.AddNode(orchid.NewNode("A"))
	co.LoadWorkflow(cwf)

	// Register the child orchestrator with the main orchestrator
	wf.AddNode(orchid.NewNode("child-workflow-starter", orchid.WithNodeConfig(map[string]interface{}{
		"child-orchestrator-selector": co,
	})))

	wf.Link("A", "B")
	wf.Link("B", "child-workflow-starter")

	o.LoadWorkflow(wf)

	ctx := context.Background()
	ctx = orchid.WithWorkflowID(ctx, "944E1EC9-355D-42B6-9AE4-6AEA7AFE3F89")

	if out, err := o.Start(ctx, []byte("parent-start")); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		fmt.Println("result:", string(out))
	}
}
