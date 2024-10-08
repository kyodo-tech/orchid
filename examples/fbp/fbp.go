package main

import (
	"context"
	"fmt"
	"os"

	orchid "github.com/kyodo-tech/orchid"
)

const workflowName = "fbp.json"

func PrintActivity(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Println(text)
		return []byte(text), nil
	}
}

func PrintActivityWithRoutingKey(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Println(text)
		return []byte(text), &orchid.DynamicRoute{Key: "A2"}
	}
}

func main() {
	o := orchid.NewOrchestrator()
	o.RegisterActivity("A1", PrintActivityWithRoutingKey("A1"))
	o.RegisterActivity("A2", PrintActivity("A2"))
	o.RegisterActivity("B", PrintActivity("B"))
	o.RegisterActivity("C", PrintActivity("C"))
	o.RegisterActivity("D", PrintActivity("D"))
	o.RegisterActivity("E", PrintActivity("E"))

	wf := orchid.NewWorkflow(workflowName)
	if _, err := os.Stat(workflowName); err == nil {
		data, err := os.ReadFile(workflowName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = wf.Import(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Loaded workflow from file")

	} else {
		// file does not exist
		wf.AddNode(orchid.NewNode("A1"))
		wf.AddNode(orchid.NewNode("A2"))
		wf.AddNode(orchid.NewNode("B"))
		wf.AddNode(orchid.NewNode("C"))
		wf.AddNode(orchid.NewNode("D"))
		wf.AddNode(orchid.NewNode("E"))

		wf.Link("A1", "B")
		wf.Link("A1", "A2")
		wf.Link("A2", "C")
		wf.Link("B", "D")
		wf.Link("C", "D")
		wf.Link("D", "E")

		data, err := wf.Export()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := os.WriteFile(workflowName, data, 0644); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Saved workflow to file")
	}

	o.LoadWorkflow(wf)

	ctx := context.Background()
	ctx = orchid.WithWorkflowID(ctx, "944E1EC9-355D-42B6-9AE4-6AEA7AFE3F89")

	if out, err := o.Start(ctx, nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		fmt.Println("result:", string(out))
	}
}
