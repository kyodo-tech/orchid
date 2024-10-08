package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"

	orchid "github.com/kyodo-tech/orchid"
	"github.com/kyodo-tech/orchid/persistence"
	"github.com/google/uuid"
)

const workflowName = "webhook.json"

func WebhookListener(ctx context.Context, input []byte) ([]byte, error) {
	endpoint, _ := orchid.ConfigString(ctx, "endpoint")
	address, _ := orchid.ConfigString(ctx, "address")

	workflow, _ := orchid.AsyncExecutor(ctx)

	http.HandleFunc(endpoint, func(rw http.ResponseWriter, req *http.Request) {
		// capture POST body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, "can't read body", http.StatusBadRequest)
			return
		}

		// read request-id from header
		requestID := req.Header.Get("request-id")
		if requestID != "" {
			ctx = orchid.WithWorkflowID(ctx, requestID)
		} else {
			ctx = orchid.WithWorkflowID(ctx, uuid.New().String())
		}

		nonRecvoerySet := req.Header.Get("non-restorable")
		if nonRecvoerySet == "true" {
			ctx = orchid.WithNonRestorable(ctx)
		}

		// execute activity async after the first node
		out, err := workflow(ctx, body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Write(out)
	})

	http.ListenAndServe(address, nil)

	return nil, nil
}

func concatPrintActivity(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		out := append(input, []byte(text)...)
		fmt.Println(string(out))
		return out, nil
	}
}

func withDynamicRoute(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		out := append(input, []byte(text)...)
		fmt.Println(string(out))
		// return out, &orchid.DynamicRoute{Key: "E"}
		// route to first character of input
		c := fmt.Sprintf("%c", input[0])
		fmt.Println("routing to", c)
		return out, &orchid.DynamicRoute{Key: c}
	}
}

func randomFailConcateActivity(text string) func(ctx context.Context, input []byte) ([]byte, error) {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		out := append(input, []byte(text)...)
		if rand.Intn(5) == 0 {
			return nil, fmt.Errorf("random failure")
		}

		fmt.Println(string(out))
		return out, nil
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	persister, err := persistence.NewSQLitePersister("orchid.db")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer persister.DB.Close()

	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
		orchid.WithPersistence(persister),
	)

	o.RegisterActivity("webhooklistener", WebhookListener)
	o.RegisterActivity("B", concatPrintActivity("B"))
	o.RegisterActivity("C", withDynamicRoute("C"))
	o.RegisterActivity("D", randomFailConcateActivity("D"))
	o.RegisterActivity("E", randomFailConcateActivity("E"))

	wf := orchid.NewWorkflow(workflowName)

	if _, err := os.Stat(workflowName); err == nil {
		data, err := os.ReadFile(workflowName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = wf.Import(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Loaded workflow from file")

	} else {
		// file does not exist
		wf.AddNode(orchid.NewNode("webhooklistener",
			orchid.WithNodeType(orchid.Trigger),
			orchid.WithNodeConfig(map[string]interface{}{"address": ":8080", "endpoint": "/webhook"}),
		))

		wf.AddNode(orchid.NewNode("B"))
		wf.AddNode(orchid.NewNode("C"))

		p := orchid.DefaultRetryPolicy()
		p.MaxRetries = 3

		wf.AddNode(orchid.NewNode("D", orchid.WithNodeRetryPolicy(p)))
		wf.AddNode(orchid.NewNode("E"))

		// wf.Link("webhooklistener", "B")
		wf.Link("B", "C")
		wf.Link("C", "D")
		wf.Link("C", "E")
		wf.Link("D", "E")

		data, err := wf.Export()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := os.WriteFile(workflowName, data, 0644); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Saved workflow to file")
	}

	o.LoadWorkflow(wf)
	if err := o.RestoreWorkflowsAsync(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	o.RunTriggers(context.Background(), nil)
	select {}
}
