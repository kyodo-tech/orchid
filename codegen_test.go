package orchid_test

import (
	"fmt"
	"os"
	"testing"

	orchid "github.com/kyodo-tech/orchid"
	"github.com/stretchr/testify/assert"
)

func Test_Codegen(t *testing.T) {
	wfJSON := `{"name":"orchid-codegen","nodes":{"A":{"id":0,"activity":"A","config":{"label":"Identify Newsworthy Topics"},"type":"action"},"B":{"id":1,"activity":"B","config":{"label":"Check for Existing Coverage","shape":"decision"},"type":"action"},"C":{"id":2,"activity":"C","config":{"label":"Assign Story \u0026 Define Angle"},"type":"action"},"D":{"id":3,"activity":"D","config":{"label":"Research \u0026 Fact-Gathering"},"type":"action"},"E":{"id":4,"activity":"E","config":{"label":"Develop Story Hypothesis \u0026 Outline"},"type":"action"},"F":{"id":5,"activity":"F","config":{"label":"Draft Writing"},"type":"action"},"G":{"id":6,"activity":"G","config":{"label":"Editorial Review"},"type":"action"},"H":{"id":7,"activity":"H","config":{"label":"Legal and Ethical Review","shape":"decision"},"type":"action"},"I":{"id":8,"activity":"I","config":{"label":"Final Edits \u0026 Approval"},"type":"action"},"J":{"id":9,"activity":"J","config":{"label":"Publishing"},"type":"action"},"K":{"id":10,"activity":"K","config":{"label":"Follow-Up \u0026 Updates"},"type":"action"},"L":{"id":11,"activity":"L","config":{"label":"New Developments?","shape":"decision"},"type":"action"},"M":{"id":12,"activity":"M","config":{"label":"End Process"},"type":"action"}},"edges":[{"from":"A","to":"B"},{"from":"B","to":"A","label":"Overlap Found"},{"from":"B","to":"C"},{"from":"C","to":"A","label":"Refine Angle"},{"from":"C","to":"D"},{"from":"D","to":"C","label":"Issues in Research"},{"from":"D","to":"E"},{"from":"E","to":"D","label":"Outline Gaps Found"},{"from":"E","to":"F"},{"from":"F","to":"D","label":"Gaps in Draft"},{"from":"F","to":"G"},{"from":"G","to":"F","label":"Major Edits Needed"},{"from":"G","to":"H"},{"from":"H","to":"F","label":"Compliance Issues"},{"from":"H","to":"I"},{"from":"I","to":"G","label":"Fact-Checking Failure"},{"from":"I","to":"J"},{"from":"J","to":"D","label":"Post-Publication Updates Needed"},{"from":"J","to":"K"},{"from":"K","to":"L"},{"from":"L","to":"D","label":"Yes"},{"from":"L","to":"M","label":"No"}]}`

	wf := orchid.NewWorkflow("")
	err := wf.Import([]byte(wfJSON))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	code, err := wf.Codegen()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	expectedCode := `wf := orchid.NewWorkflow("orchid-codegen")
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
`

	assert.Equal(t, expectedCode, code)
}
