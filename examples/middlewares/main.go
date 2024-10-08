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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/middleware"
	"github.com/google/uuid"
)

type flow struct {
	Data   []byte
	Rating int
}

func registerFlowActivity(o *orchid.Orchestrator, name string, activity func(ctx context.Context, input *flow) (*flow, error)) {
	o.RegisterActivity(name, orchid.TypedActivity(activity))
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
	)

	// Register global middlewares, before registering activities
	o.Use(middleware.Logging)

	o.RegisterActivity("A", orchid.TypedActivity(fnA))
	// or
	registerFlowActivity(o, "B", fnB)

	wf := orchid.NewWorkflow("test_workflow").
		AddNodes(
			orchid.NewNode("A"),
			orchid.NewNode("B"),
		).
		Link("A", "B")

	o.LoadWorkflow(wf)
	workflowID := uuid.New().String()
	ctx := orchid.WithWorkflowID(context.Background(), workflowID)

	out, _ := o.Start(ctx, nil)

	var f flow
	json.Unmarshal(out, &f)
	fmt.Printf("Data: %s, Rating: %d\n", f.Data, f.Rating)
}

func fnA(ctx context.Context, input *flow) (*flow, error) {
	fmt.Println("fnA", input)
	return &flow{
		Data:   []byte("A"),
		Rating: 60,
	}, nil
}

func fnB(ctx context.Context, input *flow) (*flow, error) {
	fmt.Println("fnB", input)
	return &flow{
		Data:   append(input.Data, 'B'),
		Rating: input.Rating + 9,
	}, nil
}
