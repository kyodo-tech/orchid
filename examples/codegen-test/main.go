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

package main

import (
	"fmt"

	"github.com/kyodo-tech/orchid"
)

func main() {
	wf := orchid.NewWorkflow("orchid")
	wf.AddNode(orchid.NewNode("A", orchid.WithNodeConfig(map[string]interface{}{"label": "Identify Newsworthy Topics"})))
	wf.Then(orchid.NewNode("B", orchid.WithNodeConfig(map[string]interface{}{"label": "Check for Existing Coverage", "shape": "decision"})))
	wf.Then(orchid.NewNode("C", orchid.WithNodeConfig(map[string]interface{}{"label": "Assign Story & Define Angle"})))
	wf.Then(orchid.NewNode("D", orchid.WithNodeConfig(map[string]interface{}{"label": "Research & Fact-Gathering"})))
	wf.Then(orchid.NewNode("E", orchid.WithNodeConfig(map[string]interface{}{"label": "Develop Story Hypothesis & Outline"})))
	wf.Then(orchid.NewNode("F", orchid.WithNodeConfig(map[string]interface{}{"label": "Draft Writing"})))
	wf.Then(orchid.NewNode("G", orchid.WithNodeConfig(map[string]interface{}{"label": "Editorial Review"})))
	wf.Then(orchid.NewNode("H", orchid.WithNodeConfig(map[string]interface{}{"label": "Legal and Ethical Review", "shape": "decision"})))
	wf.Then(orchid.NewNode("I", orchid.WithNodeConfig(map[string]interface{}{"label": "Final Edits & Approval"})))
	wf.Then(orchid.NewNode("J", orchid.WithNodeConfig(map[string]interface{}{"label": "Publishing"})))
	wf.Then(orchid.NewNode("K", orchid.WithNodeConfig(map[string]interface{}{"label": "Follow-Up & Updates"})))
	wf.Then(orchid.NewNode("L", orchid.WithNodeConfig(map[string]interface{}{"label": "New Developments?", "shape": "decision"})))
	wf.Then(orchid.NewNode("M", orchid.WithNodeConfig(map[string]interface{}{"label": "End Process"})))
	wf.LinkWithLabel("B", "A", "Overlap Found")
	wf.LinkWithLabel("C", "A", "Refine Angle")
	wf.LinkWithLabel("D", "C", "Issues in Research")
	wf.LinkWithLabel("E", "D", "Outline Gaps Found")
	wf.LinkWithLabel("F", "D", "Gaps in Draft")
	wf.LinkWithLabel("G", "F", "Major Edits Needed")
	wf.LinkWithLabel("H", "F", "Compliance Issues")
	wf.LinkWithLabel("I", "G", "Fact-Checking Failure")
	wf.LinkWithLabel("J", "D", "Post-Publication Updates Needed")
	wf.LinkWithLabel("L", "D", "Yes")

	outputJSON, err := wf.Export()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(outputJSON))

	outputMermaid := wf.ExportMermaid("  ", nil, nil)

	fmt.Println(string(outputMermaid))
}
