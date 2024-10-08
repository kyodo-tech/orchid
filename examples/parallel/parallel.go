package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	orchid "github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/persistence"
)

var orchestratorRegistry = make(map[string]*orchid.Orchestrator)

func registerOrchestrator(name string, o *orchid.Orchestrator) {
	orchestratorRegistry[name] = o
}

func getOrchestratorByName(name string) (*orchid.Orchestrator, error) {
	if o, ok := orchestratorRegistry[name]; ok {
		return o, nil
	}
	return nil, fmt.Errorf("orchestrator %s not found", name)
}

func Nop(ctx context.Context, input []byte) ([]byte, error) {
	return input, nil
}

func PrintAndForward(message string) orchid.Activity {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Activity message: %s, received: %s\n", message, string(input))
		return []byte(message), nil
	}
}

func PrintInput(ctx context.Context, input []byte) ([]byte, error) {
	fmt.Printf("Print: %s\n", string(input))
	return input, nil
}

func StartChildWorkflowActivity(ctx context.Context, input []byte) ([]byte, error) {
	<-time.After(1 * time.Second) // Simulate some work

	orchestratorName, ok := orchid.ConfigString(ctx, "child-orchestrator-name")
	if !ok {
		return nil, fmt.Errorf("failed to get child orchestrator name")
	}

	o, err := getOrchestratorByName(orchestratorName)
	if err != nil {
		return nil, fmt.Errorf("failed to get child orchestrator: %w", err)
	}

	childWorkflowID := uuid.New().String()
	ctx1 := orchid.WithWorkflowID(ctx, childWorkflowID)

	msg := fmt.Sprintf("Child workflow '%s' says hello, got input: %s", childWorkflowID, string(input))

	fmt.Printf("Starting child workflow '%s'\n", childWorkflowID)
	return o.Start(ctx1, []byte(msg))
}

func mergeOutputs(outputs [][]byte) []byte {
	var mergedOutput []byte
	for _, output := range outputs {
		mergedOutput = append(mergedOutput, output...)
	}

	return mergedOutput
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Initialize SQLite persister
	persister, err := persistence.NewSQLitePersister("orchid.db")
	if err != nil {
		fmt.Println("Failed to initialize SQLite persister:", err)
		os.Exit(1)
	}

	// Define child workflows
	co1 := orchid.NewOrchestrator(orchid.WithPersistence(persister))
	co1.RegisterActivity("PrintActivity", PrintAndForward("Hello from co1"))
	cw1 := orchid.NewWorkflow("child1")
	cw1.AddNode(orchid.NewNode("PrintActivity"))
	co1.LoadWorkflow(cw1)
	registerOrchestrator("co1", co1) // Register co1

	co2 := orchid.NewOrchestrator(orchid.WithPersistence(persister))
	co2.RegisterActivity("PrintActivity", PrintAndForward("Hello from co2"))
	cw2 := orchid.NewWorkflow("child2")
	cw2.AddNode(orchid.NewNode("PrintActivity"))
	co2.LoadWorkflow(cw2)
	registerOrchestrator("co2", co2) // Register co2

	// Initialize parent orchestrator
	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
		orchid.WithPersistence(persister),
	)

	o.RegisterActivity("Start", PrintInput)
	o.RegisterActivity("StartCw1", StartChildWorkflowActivity)
	o.RegisterActivity("StartCw2", StartChildWorkflowActivity)
	o.RegisterActivity("WaitAndMerge", Nop)
	o.RegisterReducer("WaitAndMerge", mergeOutputs)

	// Define parent workflow
	wf := orchid.NewWorkflow("parent")
	wf.AddNode(orchid.NewNode("Start"))
	wf.AddNode(orchid.NewNode("StartCw1", orchid.WithNodeConfig(map[string]interface{}{
		"child-orchestrator-name": "co1",
	})))
	wf.AddNode(orchid.NewNode("StartCw2", orchid.WithNodeConfig(map[string]interface{}{
		"child-orchestrator-name": "co2",
	})))
	wf.AddNode(orchid.NewNode("WaitAndMerge"))

	wf.Link("Start", "StartCw1")
	wf.Link("Start", "StartCw2")
	wf.Link("StartCw1", "WaitAndMerge")
	wf.Link("StartCw2", "WaitAndMerge")
	o.LoadWorkflow(wf)

	ctx := context.Background()
	ctx = orchid.WithWorkflowID(ctx, "parent-"+uuid.New().String())

	if output, err := o.Start(ctx, []byte("parent workflow start")); err != nil {
		fmt.Println("Workflow failed:", err)
		os.Exit(1)
	} else {
		fmt.Println("Workflow completed with output:", string(output))
	}

	// showcase what is persisted:

	// dwf, _ := wf.Export()
	// if err := os.WriteFile("wf.json", dwf, 0644); err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }

	// dcw2, _ := cw2.Export()
	// if err := os.WriteFile("cw2.json", dcw2, 0644); err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }

	// wf.ExportDotToFile("wf.dot", map[string]*orchid.Workflow{
	// 	"StartCw1": cw1,
	// 	"StartCw2": cw2,
	// })
}