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
	mermaid := `flowchart TD
    A[Identify Newsworthy Topics] --> B{Check for Existing Coverage}
    B --"Overlap Found"--> A
    B --> C[Assign Story & Define Angle]
    C --"Refine Angle"--> A
    C --> D[Research & Fact-Gathering]
    D --"Issues in Research"--> C
    D --> E[Develop Story Hypothesis & Outline]
    E --"Outline Gaps Found"--> D
    E --> F[Draft Writing]
    F --"Gaps in Draft"--> D
    F --> G[Editorial Review]
    G --"Major Edits Needed"--> F
    G --> H{Legal and Ethical Review}
    H --"Compliance Issues"--> F
    H --> I[Final Edits & Approval]
    I --"Fact-Checking Failure"--> G
    I --> J[Publishing]
    J --"Post-Publication Updates Needed"--> D
    J --> K[Follow-Up & Updates]
    K --> L{New Developments?}
    L --"Yes"--> D
    L --"No"--> M[End Process]`

	wf, err := orchid.ImportMermaid("mermaid import", mermaid)
	if err != nil {
		fmt.Println("Error importing Mermaid:", err)
		return
	}

	outputJSON, err := wf.Export()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(outputJSON))

	outputMermaid := wf.ExportMermaid("  ", nil, nil)

	fmt.Println(string(outputMermaid))
}
