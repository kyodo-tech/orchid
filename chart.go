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

package orchid

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
)

func (wf *Workflow) isStartNode(node *Node) bool {
	for _, n := range wf.startingGraphNodes() {
		if n.ID() == node.ID {
			return true
		}
	}
	return false
}

func (wf *Workflow) isParallelNode(node *Node) bool {
	parallelNodes := markParallelNodes(wf.directedGraph)
	_, ok := parallelNodes[node.ID]
	return ok
}

func (wf *Workflow) ExportDot(indent string, optionalChildWorkflows map[string]*Workflow) []byte {
	var dotData []byte
	dotData = append(dotData, []byte("digraph \"")...)
	dotData = append(dotData, []byte(wf.Name)...)
	dotData = append(dotData, []byte("\" {\n")...)

	// Keep track of visited workflows to prevent infinite recursion
	visited := make(map[string]bool)
	dotData = append(dotData, wf.exportDotRecursive(indent+"    ", visited, optionalChildWorkflows)...)

	dotData = append(dotData, []byte("}\n")...)
	return dotData
}

func (wf *Workflow) exportDotRecursive(indent string, visited map[string]bool, optionalChildWorkflows map[string]*Workflow) []byte {
	var dotData []byte

	if visited[wf.Name] {
		return dotData
	}
	visited[wf.Name] = true

	for _, node := range wf.Nodes {
		if !wf.isStartNode(node) {
			continue
		}

		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\""+node.ActivityName+"\"")...)
		dotData = append(dotData, []byte(" [shape=doublecircle, color=green]")...)
		dotData = append(dotData, []byte(";\n")...)
	}

	// Node definitions with styling
	for _, node := range wf.Nodes {
		if wf.isStartNode(node) {
			continue
		}

		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\""+node.ActivityName+"\"")...)

		// Add styling for parallel nodes
		if wf.isParallelNode(node) {
			dotData = append(dotData, []byte(" [style=filled, fillcolor=lightblue]")...)
		}

		dotData = append(dotData, []byte(";\n")...)
	}

	// Edge definitions
	for _, edge := range wf.Edges {
		dotData = append(dotData, []byte(indent)...)
		dotData = append(dotData, []byte("\""+edge.From+"\" -> \""+edge.To+"\"")...)
		dotData = append(dotData, []byte(";\n")...)
	}

	// Recursively include child workflows
	for _, node := range wf.Nodes {
		if childWf, exists := optionalChildWorkflows[node.ActivityName]; exists {
			// Subgraph for child workflow
			dotData = append(dotData, []byte(indent)...)
			dotData = append(dotData, []byte("subgraph \"cluster_"+childWf.Name+"\" {\n")...)
			dotData = append(dotData, []byte(indent+"    label = \""+childWf.Name+"\";\n")...)
			dotData = append(dotData, childWf.exportDotRecursive(indent+"    ", visited, optionalChildWorkflows)...)
			dotData = append(dotData, []byte(indent+"}\n")...)
		}
	}

	return dotData
}

func (wf *Workflow) ExportDotToFile(filename string, optionalChildWorkflows map[string]*Workflow) error {
	dotData := wf.ExportDot("    ", optionalChildWorkflows)

	err := os.WriteFile(filename, []byte(dotData), 0644)
	if err != nil {
		return err
	}

	return nil
}

// ExportMermaid generates the Mermaid representation of the workflow.
// Optionally, it can include child workflows as subgraphs and requires a map of
// node names and the corresponding child workflows they spawn.
func (wf *Workflow) ExportMermaid(indent string, nodeToChildWorkflows map[string]*Workflow) []byte {
	var mermaidData []byte
	mermaidData = append(mermaidData, []byte("flowchart TD\n")...)

	visited := make(map[string]bool)
	classAssignments := make([]string, 0)
	mermaidData = append(mermaidData, wf.exportMermaidRecursive(indent+"    ", visited, nodeToChildWorkflows, "", &classAssignments)...)

	mermaidData = append(mermaidData, []byte("\n")...)

	// Append collected class assignments after subgraphs
	for _, classAssign := range classAssignments {
		mermaidData = append(mermaidData, []byte(classAssign)...)
	}

	mermaidData = append(mermaidData, []byte("\n")...)

	// Add class definitions at the end
	mermaidData = append(mermaidData, []byte("classDef startNode fill:#9f6,stroke:#333,stroke-width:4px;\n")...)
	mermaidData = append(mermaidData, []byte("classDef parallelNode fill:#6cf,stroke:#333,stroke-width:2px;\n")...)

	return mermaidData
}

func (wf *Workflow) exportMermaidRecursive(indent string, visited map[string]bool, nodeToChildWorkflows map[string]*Workflow, prefix string, classAssignments *[]string) []byte {
	var mermaidData []byte

	if visited[wf.Name] {
		return mermaidData
	}
	visited[wf.Name] = true

	// Start by defining starting nodes to ensure they appear at the top
	startNodes := wf.startingNodes()
	for _, node := range startNodes {
		nodeName := prefix + node.ActivityName
		nodeLabel := node.ActivityName
		nodeLine := indent + nodeName + "[" + nodeLabel + "]\n"
		mermaidData = append(mermaidData, []byte(nodeLine)...)

		if node.EditLink != nil {
			mermaidData = append(mermaidData, []byte(fmt.Sprintf("click %s \"%s\" _blank\n", nodeName, *node.EditLink))...)
		}

		// Collect class assignment
		*classAssignments = append(*classAssignments, fmt.Sprintf("class %s startNode\n", nodeName))
	}

	// Render remaining nodes (excluding starting nodes)
	for _, node := range wf.Nodes {
		if wf.isStartNode(node) {
			continue // Skip already rendered starting nodes
		}
		nodeName := prefix + node.ActivityName
		nodeLabel := node.ActivityName
		nodeLine := indent + nodeName + "[" + nodeLabel + "]\n"
		mermaidData = append(mermaidData, []byte(nodeLine)...)

		if node.EditLink != nil {
			mermaidData = append(mermaidData, []byte(fmt.Sprintf("click %s \"%s\" _blank\n", nodeName, *node.EditLink))...)
		}

		// Collect class assignment if node is a parallel node
		if wf.isParallelNode(node) {
			*classAssignments = append(*classAssignments, fmt.Sprintf("class %s parallelNode\n", nodeName))
		}
	}

	mermaidData = append(mermaidData, []byte("\n")...)

	// Edge definitions
	for _, edge := range wf.Edges {
		fromNode := prefix + edge.From
		toNode := prefix + edge.To

		if childWf, exists := nodeToChildWorkflows[edge.To]; exists {
			// 'To' node is a child workflow
			childPrefix := edge.To + "_"
			entryNodes := childWf.startingNodes()

			// Connect parent node to child workflow's entry nodes
			for _, entryNode := range entryNodes {
				mermaidData = append(mermaidData, []byte(indent)...)
				mermaidData = append(mermaidData, []byte(fromNode)...)
				mermaidData = append(mermaidData, []byte(" --> ")...)
				mermaidData = append(mermaidData, []byte(childPrefix+entryNode.ActivityName)...)
				mermaidData = append(mermaidData, []byte("\n")...)
			}

			// Render the child workflow subgraph recursively
			mermaidData = append(mermaidData, []byte(indent+"subgraph "+edge.To+"\n")...)
			mermaidData = append(mermaidData, childWf.exportMermaidRecursive(indent+"    ", visited, nodeToChildWorkflows, childPrefix, classAssignments)...)
			mermaidData = append(mermaidData, []byte(indent+"end\n")...)
		} else if childWf, exists := nodeToChildWorkflows[edge.From]; exists {
			// 'From' node is a child workflow
			childPrefix := edge.From + "_"
			exitNodes := childWf.exitNodes()

			// Connect child workflow's exit nodes to parent node
			for _, exitNode := range exitNodes {
				mermaidData = append(mermaidData, []byte(indent)...)
				mermaidData = append(mermaidData, []byte(childPrefix+exitNode.ActivityName)...)
				mermaidData = append(mermaidData, []byte(" --> ")...)
				mermaidData = append(mermaidData, []byte(toNode)...)
				mermaidData = append(mermaidData, []byte("\n")...)
			}
		} else {
			// Regular edge
			mermaidData = append(mermaidData, []byte(indent)...)
			mermaidData = append(mermaidData, []byte(fromNode)...)
			mermaidData = append(mermaidData, []byte(" --> ")...)
			mermaidData = append(mermaidData, []byte(toNode)...)
			mermaidData = append(mermaidData, []byte("\n")...)
		}
	}

	return mermaidData
}

//go:embed templates/mermaid.html
var mermaidHTML string

func (wf *Workflow) ExportMermaidHTML(indent string, optionalChildWorkflows map[string]*Workflow) ([]byte, error) {
	mermaidData := wf.ExportMermaid(indent, optionalChildWorkflows)

	// render template with map of .Flowchart
	tmpl, err := template.New("mermaid").Parse(mermaidHTML)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Title":     wf.Name,
		"Flowchart": string(mermaidData),
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (wf *Workflow) ExportMermaidToFile(filename string, optionalChildWorkflows map[string]*Workflow) error {
	mermaidData := wf.ExportMermaid("    ", optionalChildWorkflows)

	err := os.WriteFile(filename, []byte(mermaidData), 0644)
	if err != nil {
		return err
	}

	return nil
}
