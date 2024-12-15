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
	"fmt"
	"go/format"
	"sort"
	"strings"
	"time"
)

func (wf *Workflow) Codegen() (string, error) {
	var buf bytes.Buffer

	// Collect unique RetryPolicies
	type retryPolicyKey struct {
		MaxRetries         int
		InitInterval       time.Duration
		MaxInterval        time.Duration
		BackoffCoefficient float64
	}

	retryPolicies := make(map[retryPolicyKey]string)
	retryPolicyVars := make(map[string]string)

	for _, node := range wf.Nodes {
		// Handle RetryPolicy
		if node.RetryPolicy != nil {
			key := retryPolicyKey{
				MaxRetries:         node.RetryPolicy.MaxRetries,
				InitInterval:       node.RetryPolicy.InitInterval,
				MaxInterval:        node.RetryPolicy.MaxInterval,
				BackoffCoefficient: node.RetryPolicy.BackoffCoefficient,
			}
			if _, exists := retryPolicies[key]; !exists {
				varName := fmt.Sprintf("retryPolicy%d", len(retryPolicies)+1)
				retryPolicies[key] = varName
				retryPolicyVars[node.ActivityName] = varName
			} else {
				retryPolicyVars[node.ActivityName] = retryPolicies[key]
			}
		}
	}

	// Generate code for RetryPolicies
	for key, varName := range retryPolicies {
		fmt.Fprintf(&buf, "%s := &orchid.RetryPolicy{\n", varName)
		fmt.Fprintf(&buf, "\tMaxRetries: %d,\n", key.MaxRetries)
		fmt.Fprintf(&buf, "\tInitInterval: %s,\n", key.InitInterval)
		fmt.Fprintf(&buf, "\tMaxInterval: %s,\n", key.MaxInterval)
		fmt.Fprintf(&buf, "\tBackoffCoefficient: %.1f,\n", key.BackoffCoefficient)
		fmt.Fprintf(&buf, "}\n\n")
	}

	// Create the Workflow
	fmt.Fprintf(&buf, "wf := orchid.NewWorkflow(%q)\n", wf.Name)

	// Build a set of edges in the workflow
	wfEdges := make(map[string]bool)
	for _, edge := range wf.Edges {
		edgeKey := fmt.Sprintf("%s->%s", edge.From, edge.To)
		wfEdges[edgeKey] = true
	}

	// Collect nodes and sort them by ID
	type nodeWithID struct {
		ID   int64
		Node *Node
	}
	var nodes []nodeWithID
	for _, node := range wf.Nodes {
		nodes = append(nodes, nodeWithID{ID: node.ID, Node: node})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Map to keep track of nodes added
	nodeNames := make(map[string]struct{})
	impliedEdges := make(map[string]bool)
	var prevNodeName string

	// Generate code for nodes
	for idx, nw := range nodes {
		node := nw.Node
		nodeName := node.ActivityName

		if _, exists := nodeNames[nodeName]; exists {
			continue
		}

		options := []string{}

		// Handle RetryPolicy
		if varName, ok := retryPolicyVars[node.ActivityName]; ok {
			options = append(options, fmt.Sprintf("orchid.WithNodeRetryPolicy(%s)", varName))
		}

		// Handle Config
		if node.Config != nil {
			// For other configs, format the map as Go code
			configCode, err := formatMap(node.Config)
			if err != nil {
				return "", err
			}
			options = append(options, fmt.Sprintf("orchid.WithNodeConfig(%s)", configCode))
		}

		// Handle DisableSequentialFlow
		if node.DisableSequentialFlow {
			options = append(options, "orchid.WithNodeDisableSequentialFlow()")
		}

		// Build node creation code
		var nodeCode string
		if idx == 0 {
			// First node
			if len(options) > 0 {
				nodeCode = fmt.Sprintf("wf.AddNode(orchid.NewNode(%q, %s))", node.ActivityName, strings.Join(options, ", "))
			} else {
				nodeCode = fmt.Sprintf("wf.AddNode(orchid.NewNode(%q))", node.ActivityName)
			}
		} else {
			// Decide whether to chain or not
			edgeKey := fmt.Sprintf("%s->%s", prevNodeName, nodeName)
			if wfEdges[edgeKey] {
				// There is an edge from prevNodeName to nodeName, we can chain
				if len(options) > 0 {
					nodeCode = fmt.Sprintf("wf.Then(orchid.NewNode(%q, %s))", node.ActivityName, strings.Join(options, ", "))
				} else {
					nodeCode = fmt.Sprintf("wf.ThenNewNode(%q)", node.ActivityName)
				}
				// Record implied edge
				impliedEdges[edgeKey] = true
			} else {
				// No edge from prevNodeName to nodeName, we need to add node separately
				if len(options) > 0 {
					nodeCode = fmt.Sprintf("wf.AddNode(orchid.NewNode(%q, %s))", node.ActivityName, strings.Join(options, ", "))
				} else {
					nodeCode = fmt.Sprintf("wf.AddNode(orchid.NewNode(%q))", node.ActivityName)
				}
			}
		}

		// Write node code
		fmt.Fprintf(&buf, "%s\n", nodeCode)

		nodeNames[nodeName] = struct{}{}
		prevNodeName = nodeName
	}

	// Now handle edges
	for _, edge := range wf.Edges {
		edgeKey := fmt.Sprintf("%s->%s", edge.From, edge.To)
		if impliedEdges[edgeKey] {
			// Edge is already implied by Then calls, skip it
			continue
		}
		if edge.Label != nil {
			// Use LinkWithLabel
			fmt.Fprintf(&buf, "wf.LinkWithLabel(%q, %q, %q)\n", edge.From, edge.To, *edge.Label)
		} else {
			// Use Link
			fmt.Fprintf(&buf, "wf.Link(%q, %q)\n", edge.From, edge.To)
		}
	}

	// Format the generated code
	formattedCode, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code with the error
		return buf.String(), fmt.Errorf("could not format generated code: %w", err)
	}

	return string(formattedCode), nil
}

// Helper function to format map[string]interface{} as Go code
func formatMap(m map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("map[string]interface{}{")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		v := m[k]
		valStr, err := formatValue(v)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&buf, "%q: %s", k, valStr)
		if i < len(keys)-1 {
			buf.WriteString(", ")
		}
	}
	buf.WriteString("}")
	return buf.String(), nil
}

// Helper function to format a value as Go code
func formatValue(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val), nil
	case []interface{}:
		var elems []string
		for _, e := range val {
			elemStr, err := formatValue(e)
			if err != nil {
				return "", err
			}
			elems = append(elems, elemStr)
		}
		return fmt.Sprintf("[]interface{}{%s}", strings.Join(elems, ", ")), nil
	case map[string]interface{}:
		return formatMap(val)
	default:
		// For other types, use fmt.Sprintf with %v
		return fmt.Sprintf("%v", val), nil
	}
}
