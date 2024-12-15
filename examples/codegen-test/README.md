# mermaid codegen

From the root of the repository, run the following command to generate the code from a mermaid file.:

    go run ./cmd/codegen examples/codegen-test/flow.mmd

The output is as follows and is exemplified in the `examples/codegen-test/main.go` file:

```go
wf := orchid.NewWorkflow("mermaid import")
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
```
