package orchid

import (
	"os"

	"gonum.org/v1/gonum/graph"
)

func nodeInSet(set []graph.Node, node *Node) bool {
	for _, n := range set {
		if n.ID() == node.ID {
			return true
		}
	}
	return false
}

func (wf *Workflow) ExportDot(name, indent string) []byte {
	// strict digraph "Marketplace Simulation" {
	// 	// Node definitions.
	// 	0;
	// 	1;
	// 	2;
	// 	3;
	// 	4;

	// 	// Edge definitions.
	// 	0 -> 1;
	// 	1 -> 3;
	// 	2 -> 1;
	// 	2 -> 4;
	// 	3 -> 2;
	// }
	var dotData []byte
	dotData = append(dotData, []byte("strict digraph \"")...)
	dotData = append(dotData, []byte(name)...)
	dotData = append(dotData, []byte("\" {\n")...)

	// Starting nodes first.
	startingNodes := wf.startingNodes()
	for _, n := range startingNodes {
		activityName, _ := wf.getActivityNameByNodeID(n.ID())
		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(activityName)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(";\n")...)
	}

	// Other node definitions.
	for _, node := range wf.Nodes {
		if nodeInSet(startingNodes, node) {
			continue
		}

		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(node.ActivityName)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(";\n")...)
	}

	// Edge definitions.
	for _, edge := range wf.Edges {
		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(edge.From)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(" -> ")...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(edge.To)...)
		dotData = append(dotData, []byte("\"")...)
		dotData = append(dotData, []byte(";\n")...)
	}

	dotData = append(dotData, []byte("}\n")...)

	return dotData
}

func (wf *Workflow) ExportDotToFile(filename string) error {
	dotData := wf.ExportDot(wf.Name, "    ")

	err := os.WriteFile(filename, []byte(dotData), 0644)
	if err != nil {
		return err
	}

	return nil
}

// ExportMermaid generates the Mermaid representation of the workflow.
func (wf *Workflow) ExportMermaid(name, indent string) []byte {
	// Initial setup for Mermaid graph
	var mermaidData []byte
	mermaidData = append(mermaidData, []byte("graph TD\n")...)

	// Node definitions are implicit in Mermaid through edge definitions,
	// so we can omit explicit node declarations unless we need specific styling.

	// Edge definitions.
	for _, edge := range wf.Edges {
		mermaidData = append(mermaidData, []byte(indent)...)
		mermaidData = append(mermaidData, []byte(edge.From)...)
		mermaidData = append(mermaidData, []byte(" --> ")...)
		mermaidData = append(mermaidData, []byte(edge.To)...)
		mermaidData = append(mermaidData, []byte("\n")...)
	}

	return mermaidData
}

// ExportMermaidToFile writes the generated Mermaid diagram to a file.
func (wf *Workflow) ExportMermaidToFile(filename string) error {
	mermaidData := wf.ExportMermaid(wf.Name, "    ")

	err := os.WriteFile(filename, mermaidData, 0644)
	if err != nil {
		return err
	}

	return nil
}
