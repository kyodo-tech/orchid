# Orchid: Dynamic Dataflow Orchestration

[![Go Report Card](https://goreportcard.com/badge/github.com/kyodo-tech/orchid)](https://goreportcard.com/report/github.com/kyodo-tech/orchid)
[![GoDoc](https://godoc.org/github.com/kyodo-tech/orchid?status.svg)](https://godoc.org/github.com/kyodo-tech/orchid)

Orchid is a lightweight Go framework for orchestrating data-driven workflows. It combines concepts from Flow-Based Programming (FBP) and workflow engines to provide a simple, fault-tolerant solution for managing data flows and task execution within applications.

Inspired by tools like [Uber Cadence](https://github.com/uber/cadence) and [Temporal.io](https://temporal.io/), Orchid offers a minimalistic approach to workflow orchestration without the complexity and heavy dependencies often found in other solutions. It was created out of a lack of a simple executor such as [temporalite](https://github.com/temporalio/temporalite), but is no longer maintained. Other solutions are often complex with many dependencies.

Orchid is designed with the following principles in mind:
- **Simplicity**: Designed to be easy to understand and use, with minimal boilerplate. Orchid's core is around 1k lines of code.
- **Data Passing**: Facilitates data passing between nodes using byte arrays, aligning with flow-based programming paradigms.
- **Dynamic Routing**: Supports dynamic routing based on data and error conditions, enabling flexible workflow logic.
- **Sequential Execution by Default**: Executes workflows sequentially by default. When a node has multiple outgoing edges, it deterministically follows the first path unless parallelism is explicitly enabled.
- **Explicit Parallelism**: Allows explicit parallel execution through the `DisableSequentialFlow` node option. By setting this option on a node, you enable it to execute multiple outgoing paths in parallel, making parallelism an intentional design choice.
- **Hybrid Execution**: Allows both synchronous and asynchronous task execution.
- **Modular**: Encourages modular code by encapsulating functionality within independent nodes.
- **Retry Policies**: Allows to define custom retry policies for each task.
- **Context Propagation**: Utilizes Go's `context` package to pass metadata and cancellation signals throughout workflow execution.
- **Graph-Based Workflows**: Workflows are directed graphs, making complex processes easier to visualize.
- **Fault Tolerance**: Provides optional state persistence and workflow recovery.
- **JSON Workflow Definitions**: Supports  a human-readable DSL, a basic JSON format for defining workflows, in addition to code-based definitions.

Use Cases:
- **Data Pipelines**: Orchestrate data processing tasks, including data ingestion, transformation, and analysis.
- **Automation Workflows**: Manage automated tasks and interactions with machine or human agents.
- **Service Orchestration**: Coordinate interactions between application services and manage distributed API workflows.
- **Human-in-the-Loop Processes**: Handle workflows that require human intervention or approval.
- **Agent-Based Models**: Manage the interactions and behaviors of agents in simulations.

Connections to Related Concepts:
- **Workflow Engines**: Orchid shares foundational similarities with other DAG-based workflow engines like Airflow and Prefect but focuses on simplicity and ease of use and does allow for loops for retries. It provides a streamlined configuration format, error-based routing, and supports both local and remote execution.
- **Flow-Based Programming**: The serial execution and data passing between nodes resemble flow-based programming paradigms, but Orchid offers additional features like triggers, callbacks, and conditional branching for more complex orchestration.
- **Agent-Based Modeling (ABM)**: Orchid's ability to represent dependencies and trigger workflows based on events aligns well with the interaction patterns of agent-based models.
- **Large language model (LLM) agents**: The flexibility to pass data and handle conditional branching can be applied in orchestrating tasks involving language models.

## Trade-offs, Constraints, and Considerations

Orchid is a lot simpler than other workflow engines, designed to be easy to understand and a low number of lines of code. This simplicity comes with trade-offs and constraints that users should be aware of:

- **Fixed Activity Interface**: For simple fault tolerance, all activities must adhere to a simple interface with an input and output byte array. This design choice simplifies the implementation but may require additional logic for complex data types client side.
- **Sequential Flow by Default**: Orchid executes nodes sequentially, following a single path even if multiple outgoing edges are present. When a node has multiple outgoing edges, it deterministically picks the first next node based on the order nodes were added, ensuring consistent execution. This default behavior helps prevent unintended parallelism.
- **Enabling Parallelism**: To enable parallel execution from a node, you must explicitly set the `DisableSequentialFlow` option on that node. By doing so, you allow the node to execute multiple outgoing paths in parallel. This explicit control ensures that parallelism occurs only where intended.

```go
wf.AddNode(orchid.NewNode("ParallelNode", orchid.WithDisableSequentialFlow())).
    // other nodes...
```
A workflow ends with a single node, which can be a merge point where parallel branches converge. A merge point is a special node type "reducer" with the signature `func([][]byte) []byte` that combines the outputs of parallel branches. See the [examples/parallel](./examples/parallel) example for more details.
- **Dynamic Parallelism via Activities**: Orchid can support dynamic fan-out/fan-in or dynamic parallelism scenarios, where the number of parallel tasks is determined at runtime. This is achieved by using activities to spawn child workflows or goroutines. Note that panics in spawned, client side goroutines _MAY_ be able to crash the program and cannot be recovered by orchid. See the [examples/dynamic-parallelism](./examples/dynamic-parallelism) example for more details.
- **Activity Panic Recovery**: Activity panics can be recovered in both top-level and child workflows. Persisted workflows can therefore continue after code fixes. This only works for panics in activities that don't occur in goroutines spawned by activities. The functional option `WithFailWorkflowOnActivityPanic` of the orchestrator can be used terminate workflow and mark them non-restorable after activity panics.
- **Dynamic Routing Over Explicit Edges**: With dynamic routing is used (e.g., `orchid.RouteTo("nodeName")`), the orchestrator can route dynamically to a path.
- **Forced Transition Support**: By default, orchid does not support forced transitions between nodes. This is to prevent unintended side effects and ensure that workflows are executed as designed. If forced transitions are required, enable this on the orchestrator with `WithForcedTransitions`.
- **Idempotency and Error Handling**: Orchid ensures that workflows and activities have unique execution IDs to prevent duplicate processing. All activities are executed at-least-once, so they _MUST BE_ **idempotent** to ensure that retries do not cause unintended side effects. Pass the `ActivityToken` to external systems to ensure idempotency and fault tolerance.
- **Atomicity and Side Effects**: Activities _SHALL_ perform **atomic operations**, non-atomicity (like partial database updates) may leave the system in an inconsistent state if they fail midway.
- **Avoid shared mutable state in Activities**: Activities _SHOULD NOT_ share mutable state between them, as this can lead to race conditions and data corruption. Use the `context` package to pass data between activities, atomic datastores or synchronization primitives for shared state.
- **Execution and Recovery Trade-off**: If an optional persister is provided, Orchid can recover the state of workflows after failures.
    + Note that **passing objects in `NodeConfig` DOES NOT Work for Restoration**. NodeConfig is included in the serialized form of the workflow when exporting or persisting the workflow. The NodeConfig must be serializable into JSON along with the rest of the workflow data.
    + Further to note, the NodeConfig is currently stored **in plaintext JSON** in the persister.
- **Distributed Execution**: Clients can utilize the Activity interface to implement an executor pattern for remote execution of tasks. Each activity executes with a stable ActivityToken that can be used to register callbacks for human-in-the-loop scenarios. The token will be restored in failure cases to ensure the workflow can continue.

### Note on LangChain Comparison

Orchid supports workflows with modular task chaining, error handling, and context-passing—concepts similar to LangChain's approach to workflow chaining and agent-driven tasks. While language-agnostic, Orchid can be extended for LLMs and dynamic prompts, offering a Go-based alternative for structured workflows akin to [LangChain](https://www.langchain.com/).

## Usage

Install the package:

```bash
go get github.com/kyodo-tech/orchid
```

Define a **Workflow**: Create a new workflow and add nodes (tasks) and edges (data routes) to define the processing steps and how data moves between them.

```go
wf := orchid.NewWorkflow("example_workflow")
wf.AddNode(orchid.NewNode("task1"))
wf.AddNode(orchid.NewNode("task2"))
wf.Link("task1", "task2")
```

Or use the fluent API for linear use cases:

```go
wf := orchid.NewWorkflow("example_workflow").
    AddNode(orchid.NewNode("task1")).
    Then(orchid.NewNode("task2"))
```

Implement an **Activity**: Define the logic for each task in the workflow.
```go
func task1Handler(ctx context.Context, input []byte) ([]byte, error) {
    // Task logic here
    return output, nil
}
```

Execute the Workflow: Initialize the orchestrator, register task handlers, and execute the workflow.
```go
o := orchid.NewOrchestrator()
o.RegisterActivity("task1", task1Handler)
o.RegisterActivity("task2", task2Handler)
o.LoadWorkflow(wf)
o.RunTriggers(context.Background(), nil)
```

Context based logging if slog is used and the orchestrator is set up with it:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

o := orchid.NewOrchestrator(
    orchid.WithLogger(logger),
)
```

Which is available in activities:

```go
orchid.Logger(ctx).Error("Error executing task", "task", taskName, "error", err)
```

## Persistence and Fault Tolerance

Orchid supports fault tolerance through state persistence and recovery mechanisms. By providing an optional persister, Orchid can recover the state of workflows after failures. The persistence layer _MUST_ accurately reflect the state of the workflow to avoid incorrect restoration. Cyclic dependencies, disconnected graphs, or malformed workflows _CAN_ cause the restoration to fail or behave unpredictably. Well-formed workflows recover at the last non-parallel node, replay successful node executions until the failure point, and retry the failed nodes.

To enable persistence, create a persister instance and pass it to the orchestrator:

```go
persister, err := persistence.NewSQLitePersister("orchid.db")
if err != nil {
    // Handle error
}
defer persister.DB.Close()

o := orchid.NewOrchestrator(orchid.WithPersistence(persister))
```

We can also pass a default retry policy for all nodes if desired:

```go
o := orchid.NewOrchestratorWithWorkflow(wf,
    orchid.WithDefaultRetryPolicy(&orchid.RetryPolicy{
        MaxRetries:         3,
        InitInterval:       2 * time.Second,
        MaxInterval:        30 * time.Second,
        BackoffCoefficient: 2.0,
    }),
    // ... other options ...
)
```

## Middleware Support

Middlewares allow wrapping activities with additional functionality, such as logging, error handling, or custom serialization. We can apply cross-cutting concerns consistently across activities without modifying their core logic.

A middleware is a function that takes an Activity and returns a new Activity. This allows us to compose additional behavior.

```go
type Middleware func(Activity) Activity
```

To apply a middleware, register it with the orchestrator before registering activities:

```go
o := orchid.NewOrchestrator(
    orchid.WithLogger(logger),
    // Other options...
)

// Register global middlewares
o.Use(middleware.Logging)
// ...

// Register activities after adding middlewares
o.RegisterActivity("ActivityA", activityA)
o.RegisterActivity("ActivityB", activityB)
```

For example, a logging middleware can be defined as follows:

```go
func SomeMiddleware(activity orchid.Activity) orchid.Activity {
	return func(ctx context.Context, input []byte) ([]byte, error) {
        // pre process
		output, outErr := activity(ctx, input)
        // post process
		return output, outErr
	}
}
```

## Typed Activities

Activities use `[]byte` for inputs and outputs to facilitate easy serialization and deserialization, which is important for simple state persistence and recovery. To work with custom types, you can use the `TypedActivity` helper, which handles JSON marshaling and unmarshaling. Define a custom, JSON serializable struct for your data:

```go
type flow struct {
    Data   []byte
    Rating int
}
```

Then, use `TypedActivity` to wrap your activity functions:

```go
func someTypedActivity(ctx context.Context, input *flow) (*flow, error) {
    fmt.Println("fnA input:", input)
    return &flow{
        Data:   []byte("A"),
        Rating: 60,
    }, nil
}
```

Register your activities using `TypedActivity`:

```go
o.RegisterActivity("A", orchid.TypedActivity(fnA))
```

## Caching

Orchid provides a snapshotter activity that can either save and replay the full payload or cache specific fields. This can be useful for caching data from external sources or expensive operations to avoid recomputation.

```go
fieldsToCache := []string{"user.name", "user.email", "transaction.id"}

checkpoint := orchid.NewSnapshotter(fieldsToCache)
o.RegisterActivity("checkpoint", checkpoint.Save)
```

Note that even with typed activities, snapshotting can be used as we operate on JSON data.

See the `./examples` directory for different usage scenarios.
