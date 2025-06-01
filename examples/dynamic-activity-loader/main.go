package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/kyodo-tech/orchid"
)

func main() {
	// Mermaid flow with shell-prefixed node labels
	mermaid := `flowchart TD
	A[sh:hello] --> B[sh:world]`

	wf, err := orchid.ImportMermaid("shell_workflow", mermaid)
	if err != nil {
		panic(err)
	}

	// orchestrator with dynamic loader
	orch := orchid.NewOrchestrator()
	orch.SetActivityLoader(DispatchingLoader(map[string]func(string) (orchid.Activity, error){
		"sh:": ShellScriptLoader("./scripts"), // looks up ./scripts/hello.sh and ./scripts/world.sh
	}))

	if err := orch.LoadWorkflow(wf); err != nil {
		panic(err)
	}

	ctx := orchid.WithWorkflowID(context.Background(), "sh-example")
	out, err := orch.Start(ctx, []byte("initial input\n"))
	if err != nil {
		panic(err)
	}

	fmt.Println("Final Output:")
	fmt.Println(string(out))
}

func ShellScriptLoader(dir string) func(string) (orchid.Activity, error) {
	return func(name string) (orchid.Activity, error) {
		path := fmt.Sprintf("%s/%s.sh", dir, name)
		return func(ctx context.Context, input []byte) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "sh", path)
			cmd.Stdin = bytes.NewReader(input)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = io.Discard // we could log this
			err := cmd.Run()
			return out.Bytes(), err
		}, nil
	}
}

func DispatchingLoader(prefixes map[string]func(string) (orchid.Activity, error)) func(*orchid.Node) (orchid.Activity, error) {
	return func(node *orchid.Node) (orchid.Activity, error) {
		for prefix, loader := range prefixes {
			label := node.Config["label"].(string) // the chart loader supports labels as of v0.8.0
			if strings.HasPrefix(label, prefix) {
				return loader(strings.TrimPrefix(label, prefix))
			}
		}
		return nil, fmt.Errorf("no loader matched: %s", node.ActivityName)
	}
}
