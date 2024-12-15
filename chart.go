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
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Define NodeMetadata struct with optional description and links
type NodeMetadata struct {
	Description string     // Node description, which may include line breaks
	Links       []NodeLink // List of links to display within the node
	Standalone  bool       // Flag indicating if the node should be rendered as standalone
}

// Define NodeLink struct for each link's name and URI
type NodeLink struct {
	Name string // Display text for the link
	URI  string // URL for the link
}

// ExportMermaid generates the Mermaid representation of the workflow.
// Optionally, it can include child workflows as subgraphs and requires a map of
// node names to corresponding child workflows they spawn. It also accepts an optional
// nodeToMetadata parameter to add descriptions and links to nodes.
func (wf *Workflow) ExportMermaid(indent string, nodeToChildWorkflows map[string]*Workflow, nodeToMetadata map[string]NodeMetadata) []byte {
	var mermaidData []byte
	mermaidData = append(mermaidData, []byte("flowchart TD\n")...)

	visited := make(map[string]bool)
	classAssignments := make([]string, 0)
	mermaidData = append(mermaidData, wf.exportMermaidRecursive(indent+"    ", visited, nodeToChildWorkflows, nodeToMetadata, "", &classAssignments)...)

	mermaidData = append(mermaidData, []byte("\n")...)

	// Render standalone nodes separately
	for node, metadata := range nodeToMetadata {
		if metadata.Standalone {
			mermaidData = append(mermaidData, []byte(wf.renderStandaloneNode(indent, node, metadata))...)
		}
	}

	// Append collected class assignments after subgraphs
	for _, classAssign := range classAssignments {
		mermaidData = append(mermaidData, []byte(classAssign)...)
	}

	mermaidData = append(mermaidData, []byte("\n")...)

	// Add class definitions at the end
	mermaidData = append(mermaidData, []byte("classDef startNode fill:#9f6,stroke:#333,stroke-width:4px;\n")...)
	mermaidData = append(mermaidData, []byte("classDef endNode fill:#f96,stroke:#333,stroke-width:4px;\n")...)
	mermaidData = append(mermaidData, []byte("classDef parallelNode fill:#6cf,stroke:#333,stroke-width:2px;\n")...)

	return mermaidData
}

// Function to render standalone nodes
func (wf *Workflow) renderStandaloneNode(indent, nodeName string, metadata NodeMetadata) string {
	var nodeLabel = nodeName

	// Include description and links if available
	if metadata.Description != "" {
		nodeLabel += "<br>" + metadata.Description
	}
	for _, link := range metadata.Links {
		nodeLabel += fmt.Sprintf(" <b><a href='%s' target='_blank'>%s</a></b>", link.URI, link.Name)
	}

	// Return standalone node definition
	return indent + nodeName + "[" + nodeLabel + "]\n"
}

// Function to render a node
func (wf *Workflow) renderNode(indent string, node *Node, prefix string, nodeToMetadata map[string]NodeMetadata) []byte {
	// Check if the node is standalone; if so, skip rendering it here
	if metadata, ok := nodeToMetadata[node.ActivityName]; ok && metadata.Standalone {
		return nil
	}

	var mermaidData []byte

	nodeName := prefix + node.ActivityName
	nodeLabel := node.ActivityName

	// Use the label from node.Config if available
	if label, ok := node.Config["label"].(string); ok && label != "" {
		nodeLabel = label
	}

	// Determine node shape
	nodeStart, nodeEnd := "[", "]" // Default to square brackets
	if shape, ok := node.Config["shape"].(string); ok && shape == "decision" {
		nodeStart, nodeEnd = "{", "}"
	}

	nodeLine := indent + nodeName + nodeStart + nodeLabel + nodeEnd + "\n"
	mermaidData = append(mermaidData, []byte(nodeLine)...)

	if node.EditLink != nil {
		mermaidData = append(mermaidData, []byte(fmt.Sprintf("click %s \"%s\" _blank\n", nodeName, *node.EditLink))...)
	}

	return mermaidData
}

// Recursive function to render nodes, edges, and metadata as Mermaid syntax
func (wf *Workflow) exportMermaidRecursive(indent string, visited map[string]bool, nodeToChildWorkflows map[string]*Workflow, nodeToMetadata map[string]NodeMetadata, prefix string, classAssignments *[]string) []byte {
	var mermaidData []byte

	if visited[wf.Name] {
		return mermaidData
	}
	visited[wf.Name] = true

	// Start by defining starting nodes to ensure they appear at the top
	startNodes := wf.startingNodes()
	// Sort startNodes
	sort.Slice(startNodes, func(i, j int) bool {
		return startNodes[i].ID < startNodes[j].ID
	})

	for _, node := range startNodes {
		mermaidData = append(mermaidData, wf.renderNode(indent, node, prefix, nodeToMetadata)...)

		nodeName := prefix + node.ActivityName
		// Collect class assignment
		*classAssignments = append(*classAssignments, fmt.Sprintf("class %s startNode\n", nodeName))
	}

	// Render remaining nodes (excluding starting nodes)
	var nodes []*Node
	for _, node := range wf.Nodes {
		if wf.isStartNode(node) {
			continue // Skip already rendered starting nodes
		}
		nodes = append(nodes, node)
	}

	// Sort nodes
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	for _, node := range nodes {
		mermaidData = append(mermaidData, wf.renderNode(indent, node, prefix, nodeToMetadata)...)

		nodeName := prefix + node.ActivityName

		// Collect class assignment if node is a parallel node
		if wf.isParallelNode(node) {
			*classAssignments = append(*classAssignments, fmt.Sprintf("class %s parallelNode\n", nodeName))
		}
	}

	// Collect class assignments for end nodes
	endNodes := wf.exitNodes()
	// Sort endNodes
	sort.Slice(endNodes, func(i, j int) bool {
		return endNodes[i].ID < endNodes[j].ID
	})
	for _, node := range endNodes {
		nodeName := prefix + node.ActivityName
		*classAssignments = append(*classAssignments, fmt.Sprintf("class %s endNode\n", nodeName))
	}

	// Handle the case where there are no start or end nodes
	if len(startNodes) == 0 && len(nodes) > 0 {
		nodeName := prefix + nodes[0].ActivityName
		*classAssignments = append(*classAssignments, fmt.Sprintf("class %s startNode\n", nodeName))
	}
	if len(endNodes) == 0 && len(nodes) > 0 {
		nodeName := prefix + nodes[len(nodes)-1].ActivityName
		*classAssignments = append(*classAssignments, fmt.Sprintf("class %s endNode\n", nodeName))
	}

	mermaidData = append(mermaidData, []byte("\n")...)

	// Edge definitions
	edges := wf.Edges
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	for _, edge := range edges {
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
				mermaidData = append(mermaidData, []byte(edgeConnector(edge))...)
				mermaidData = append(mermaidData, []byte(childPrefix+entryNode.ActivityName)...)
				mermaidData = append(mermaidData, []byte("\n")...)
			}

			// Render the child workflow subgraph recursively
			mermaidData = append(mermaidData, []byte(indent+"subgraph "+edge.To+"\n")...)
			mermaidData = append(mermaidData, childWf.exportMermaidRecursive(indent+"    ", visited, nodeToChildWorkflows, nodeToMetadata, childPrefix, classAssignments)...)
			mermaidData = append(mermaidData, []byte(indent+"end\n")...)
		} else if childWf, exists := nodeToChildWorkflows[edge.From]; exists {
			// 'From' node is a child workflow
			childPrefix := edge.From + "_"
			exitNodes := childWf.exitNodes()

			// Connect child workflow's exit nodes to parent node
			for _, exitNode := range exitNodes {
				mermaidData = append(mermaidData, []byte(indent)...)
				mermaidData = append(mermaidData, []byte(childPrefix+exitNode.ActivityName)...)
				mermaidData = append(mermaidData, []byte(edgeConnector(edge))...)
				mermaidData = append(mermaidData, []byte(toNode)...)
				mermaidData = append(mermaidData, []byte("\n")...)
			}
		} else {
			// Regular edge
			mermaidData = append(mermaidData, []byte(indent)...)
			mermaidData = append(mermaidData, []byte(fromNode)...)
			mermaidData = append(mermaidData, []byte(edgeConnector(edge))...)
			mermaidData = append(mermaidData, []byte(toNode)...)
			mermaidData = append(mermaidData, []byte("\n")...)
		}
	}

	return mermaidData
}

func edgeConnector(edge *Edge) string {
	// Handle edge label if present
	var edgeConnector string
	if edge.Label != nil {
		edgeConnector = fmt.Sprintf(" -- \"%s\" --> ", *edge.Label)
	} else {
		edgeConnector = " --> "
	}

	return edgeConnector
}

func (wf *Workflow) isParallelNode(node *Node) bool {
	parallelNodes := markParallelNodes(wf.directedGraph, wf.spawningParallelNodes())
	_, ok := parallelNodes[node.ID]
	return ok
}

//go:embed templates/mermaid.html
var mermaidHTML string

func (wf *Workflow) ExportMermaidHTML(indent string, optionalChildWorkflows map[string]*Workflow, nodeToMetadata map[string]NodeMetadata) ([]byte, error) {
	mermaidData := wf.ExportMermaid(indent, optionalChildWorkflows, nodeToMetadata)

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

func (wf *Workflow) ExportMermaidToFile(filename string, optionalChildWorkflows map[string]*Workflow, nodeToMetadata map[string]NodeMetadata) error {
	mermaidData := wf.ExportMermaid("    ", optionalChildWorkflows, nodeToMetadata)

	err := os.WriteFile(filename, []byte(mermaidData), 0644)
	if err != nil {
		return err
	}

	return nil
}

func ImportMermaid(name, chart string) (*Workflow, error) {
	scanner := bufio.NewScanner(strings.NewReader(chart))
	nodes := make(map[string]*Node)
	edges := []*Edge{}

	nodeRegex := regexp.MustCompile(`^\s*([A-Za-z0-9_]+)(?:\[(.+?)\]|{(.+?)})?\s*$`)
	edgeRegex := regexp.MustCompile(`^\s*([A-Za-z0-9_]+)(?:\[(.+?)\]|{(.+?)})?\s*(-->|--\s*"(.+?)"\s*-->)\s*([A-Za-z0-9_]+)(?:\[(.+?)\]|{(.+?)})?\s*$`)
	startRegex := regexp.MustCompile(`^\s*flowchart\s+(TB|TD|LR|RL|BT)\s*$`)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Skip flowchart direction line
		if matches := startRegex.FindStringSubmatch(line); matches != nil {
			continue
		}

		// Check for edge definitions
		if matches := edgeRegex.FindStringSubmatch(line); matches != nil {
			fromNodeID := matches[1]
			fromNodeLabel := ""
			fromNodeShape := ""
			if matches[2] != "" {
				fromNodeLabel = matches[2]
			} else if matches[3] != "" {
				fromNodeLabel = matches[3]
				fromNodeShape = "decision"
			}

			// edgeOperator := matches[4] // Edge operator (e.g., `-->`, `-- "label" -->`)
			edgeLabel := matches[5]

			toNodeID := matches[6]
			toNodeLabel := ""
			toNodeShape := ""
			if matches[7] != "" {
				toNodeLabel = matches[7]
			} else if matches[8] != "" {
				toNodeLabel = matches[8]
				toNodeShape = "decision"
			}

			// Ensure 'from' node exists
			fromNode, exists := nodes[fromNodeID]
			if !exists {
				fromNode = NewNode(fromNodeID)
				nodes[fromNodeID] = fromNode
			}
			if fromNodeLabel != "" || fromNodeShape != "" {
				if fromNode.Config == nil {
					fromNode.Config = make(map[string]interface{})
				}
				if fromNodeLabel != "" {
					fromNode.Config["label"] = fromNodeLabel
				}
				if fromNodeShape != "" {
					fromNode.Config["shape"] = fromNodeShape
				}
			}

			// Ensure 'to' node exists
			toNode, exists := nodes[toNodeID]
			if !exists {
				toNode = NewNode(toNodeID)
				nodes[toNodeID] = toNode
			}
			if toNodeLabel != "" || toNodeShape != "" {
				if toNode.Config == nil {
					toNode.Config = make(map[string]interface{})
				}
				if toNodeLabel != "" {
					toNode.Config["label"] = toNodeLabel
				}
				if toNodeShape != "" {
					toNode.Config["shape"] = toNodeShape
				}
			}

			// Create edge
			edge := &Edge{
				From: fromNodeID,
				To:   toNodeID,
			}
			if edgeLabel != "" {
				edge.Label = &edgeLabel
			}
			edges = append(edges, edge)
			continue
		}

		// Check for node definitions
		if matches := nodeRegex.FindStringSubmatch(line); matches != nil {
			nodeID := matches[1]
			nodeLabel := ""
			nodeShape := ""
			if matches[2] != "" {
				nodeLabel = matches[2]
			} else if matches[3] != "" {
				nodeLabel = matches[3]
				nodeShape = "decision"
			}

			// Create or update node
			node, exists := nodes[nodeID]
			if !exists {
				node = NewNode(nodeID)
				nodes[nodeID] = node
			}
			if nodeLabel != "" || nodeShape != "" {
				if node.Config == nil {
					node.Config = make(map[string]interface{})
				}
				if nodeLabel != "" {
					node.Config["label"] = nodeLabel
				}
				if nodeShape != "" {
					node.Config["shape"] = nodeShape
				}
			}
			continue
		}

		// we skip subgraphs
	}

	// Create workflow
	wf := NewWorkflow(name)

	nodesAdded := make(map[string]bool)
	for _, edge := range edges {
		// add nodes in the order of edges
		if !nodesAdded[edge.From] {
			wf.AddNode(nodes[edge.From])
			nodesAdded[edge.From] = true
		}
		if !nodesAdded[edge.To] {
			wf.AddNode(nodes[edge.To])
			nodesAdded[edge.To] = true
		}

		if err := wf.addEdge(edge); err != nil {
			return nil, err
		}
	}

	// add remaining nodes if any
	for _, node := range nodes {
		if !nodesAdded[node.ActivityName] {
			wf.AddNode(node)
		}
	}

	return wf, nil
}
