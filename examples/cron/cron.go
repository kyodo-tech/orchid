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
	"fmt"
	"log/slog"
	"os"
	"time"

	orchid "github.com/kyodo-tech/orchid"
	"github.com/google/uuid"
)

func CronTrigger(ctx context.Context, data []byte) ([]byte, error) {
	cfg, _ := orchid.Config(ctx, "schedule")
	schedule := cfg.(string)

	workflow, _ := orchid.SyncExecutor(ctx)

	for {
		switch schedule[0] {
		case 'i': // Interval, e.g., "i@15m"
			duration, err := time.ParseDuration(schedule[2:])
			if err != nil {
				panic(err)
			}

			now := time.Now()

			<-time.After(duration - time.Duration(now.UnixNano())%duration)

			encoding, _ := time.Now().UTC().MarshalBinary()
			ctx = orchid.WithWorkflowID(ctx, uuid.New().String())

			_, err = workflow(ctx, encoding)
			if err != nil {
				orchid.Logger(ctx).Error("failed to execute workflow", "error", err)
			}

		case 'd': // Daily at specific time, e.g., "d@12:00"
			t, err := time.Parse("15:04", schedule[2:])
			if err != nil {
				panic(err)
			}

			now := time.Now()
			t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)

			if t.Before(now) {
				t = t.Add(24 * time.Hour)
			}

			<-time.After(t.Sub(now))

			encoding, _ := time.Now().UTC().MarshalBinary()
			ctx = orchid.WithWorkflowID(ctx, uuid.New().String())

			_, err = workflow(ctx, encoding)
			if err != nil {
				orchid.Logger(ctx).Error("failed to execute workflow", "error", err)
			}

		default:
			panic("invalid schedule")
		}
	}
}

func InitialActivity(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		var t time.Time
		err := t.UnmarshalBinary(input)
		if err != nil {
			return nil, err
		}

		fmt.Println("Scheduled at:", t)

		return append([]byte{'A'}, []byte(text)...), nil
	}
}

func ConcatPrintActivity(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Println(string(input))
		// concat
		return append(input, []byte(text)...), nil
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	// p := persistence.NewSlogDebugger(logger)

	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
		// orchid.WithPersistence(p),
	)

	o.RegisterActivity("A", CronTrigger)
	o.RegisterActivity("B", InitialActivity("B"))
	o.RegisterActivity("C", ConcatPrintActivity("C"))
	o.RegisterActivity("D", ConcatPrintActivity("D"))
	o.RegisterActivity("E", ConcatPrintActivity("E"))

	wf := orchid.NewWorkflow("cron")

	wf.AddNode(orchid.NewNode("A",
		orchid.WithNodeType(orchid.Trigger),
		orchid.WithNodeConfig(map[string]interface{}{"schedule": "i@5s"})),
	)

	wf.AddNode(orchid.NewNode("B"))
	wf.AddNode(orchid.NewNode("C"))
	wf.AddNode(orchid.NewNode("D"))
	wf.AddNode(orchid.NewNode("E"))

	wf.Link("A", "B")
	wf.Link("B", "C")
	wf.Link("C", "D")
	wf.Link("D", "E")

	o.LoadWorkflow(wf)
	o.RunTriggers(context.Background(), nil)
	select {}
}
