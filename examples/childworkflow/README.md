# Child Workflow Example

This example demonstrates how to initiate a child workflow from a parent workflow.

```
go run examples/childworkflow/childworkflow.go
received: 'parent-start' sending: 'B'
received: 'B' sending: 'child-orchestrator-selector'
received: 'parent-spawning-child' sending: 'hello from child'
result: hello from child
```