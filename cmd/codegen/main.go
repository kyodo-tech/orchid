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
	"os"
	"strings"

	"github.com/kyodo-tech/orchid"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: codegen <filename>")
		os.Exit(1)
	}

	filename := os.Args[1]
	filebody, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	wf := orchid.NewWorkflow("")

	if strings.HasSuffix(filename, ".json") {
		err = wf.Import(filebody)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else if strings.HasSuffix(filename, ".mermaid") || strings.HasSuffix(filename, ".mmd") {
		wf, err = orchid.ImportMermaid("mermaid import", string(filebody))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Unsupported file extension")
		os.Exit(1)
	}

	code, err := wf.Codegen()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(code)
}
