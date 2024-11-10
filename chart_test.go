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

package orchid_test

import (
	"fmt"
	"testing"

	orchid "github.com/kyodo-tech/orchid"
	"github.com/stretchr/testify/assert"
)

func Test_ImportMermaid(t *testing.T) {
	mermaid := `
    flowchart TD
        A[Start] --> B{Decision}
        B -- "Yes" --> C[Option 1]
        B -- "No" --> D[Option 2]
        C --> E[End]
        D --> E
    `

	wf, err := orchid.ImportMermaid("mermaid import", mermaid)
	if err != nil {
		fmt.Println("Error importing Mermaid:", err)
		return
	}

	outputJSON, err := wf.Export()
	assert.Nil(t, err)

	expectedJSON := `{"name":"mermaid import","nodes":{"A":{"id":0,"activity":"A","config":{"label":"Start"},"type":"action"},"B":{"id":1,"activity":"B","config":{"label":"Decision","shape":"decision"},"type":"action"},"C":{"id":2,"activity":"C","config":{"label":"Option 1"},"type":"action"},"D":{"id":3,"activity":"D","config":{"label":"Option 2"},"type":"action"},"E":{"id":4,"activity":"E","config":{"label":"End"},"type":"action"}},"edges":[{"from":"A","to":"B"},{"from":"B","to":"C","label":"Yes"},{"from":"B","to":"D","label":"No"},{"from":"C","to":"E"},{"from":"D","to":"E"}]}`
	assert.JSONEq(t, expectedJSON, string(outputJSON))

	outputMermaid := wf.ExportMermaid("  ", nil, nil)

	expectedMermaid := `flowchart TD
      A[Start]
      B{Decision}
      C[Option 1]
      D[Option 2]
      E[End]

      A --> B
      B -- "Yes" --> C
      B -- "No" --> D
      C --> E
      D --> E`

	// starts with
	assert.Equal(t, expectedMermaid, string(outputMermaid[:len(expectedMermaid)]))
}
