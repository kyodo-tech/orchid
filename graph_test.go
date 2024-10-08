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
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

func TestGraph_MarkParallelNodes(t *testing.T) {
	g := simple.NewDirectedGraph()

	// A -> B -> D -> E1 -> E3 -> G
	//  \         \-> E2 -------/
	//   -> C -> F ------------/
	// B, C, D, E1, E2, F are parallel nodes, but A, G are not

	// Create nodes with IDs:
	// A(1), B(2), C(3), D(4), E1(5), E2(6), F(7), G(8), E3(9)
	nodes := make(map[int64]graph.Node)
	for i := int64(1); i <= 9; i++ {
		node := simple.Node(i)
		g.AddNode(node)
		nodes[i] = node
	}

	// Add edges:
	edges := [][2]int64{
		{1, 2}, // A->B
		{1, 3}, // A->C
		{2, 4}, // B->D
		{4, 5}, // D->E1
		{4, 6}, // D->E2
		{5, 8}, // E1->G
		{6, 9}, // E2->E3
		{9, 8}, // E3->G
		{3, 7}, // C->F
		{7, 8}, // F->G
	}
	for _, edge := range edges {
		g.SetEdge(simple.Edge{
			F: nodes[edge[0]],
			T: nodes[edge[1]],
		})
	}

	parallelNodes := markParallelNodes(g)

	count := 0
	for id := range parallelNodes {
		if id == 1 || id == 8 {
			t.Errorf("Node %d should not be parallel", id)
		}
		count++
	}

	if count != 7 {
		t.Errorf("Expected 7 parallel nodes, got %d", count)
	}
}
